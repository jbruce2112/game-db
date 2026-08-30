import Foundation
import GRDB
import Observation
import UIKit

@MainActor
@Observable
final class LibraryStore {
    var items: [LibraryItem] = []
    var query: String = ""
    var platformFilter: String = ""
    var sort: String = "title"
    var serverURL: String = UserDefaults.standard.string(forKey: "serverURL") ?? ""
    var lastSync: Date? = UserDefaults.standard.object(forKey: "lastSync") as? Date
    var online: Bool = false
    var syncMessage: String = ""
    var igdbConfigured: Bool = false
    var errorMessage: String?

    @ObservationIgnored private var db: DatabaseQueue!
    @ObservationIgnored let api = APIClient()
    @ObservationIgnored let sync = SyncService()
    @ObservationIgnored private var coverCache: [String: UIImage] = [:]

    var filtered: [LibraryItem] {
        var list = items.filter { ($0.deletedAt ?? "").isEmpty }
        if !query.isEmpty {
            list = list.filter { $0.title.localizedCaseInsensitiveContains(query) }
        }
        if !platformFilter.isEmpty {
            list = list.filter { $0.platform == platformFilter }
        }
        if sort == "added" {
            list.sort { $0.createdAt > $1.createdAt }
        } else {
            list.sort { $0.title.localizedCaseInsensitiveCompare($1.title) == .orderedAscending }
        }
        return list
    }

    var platforms: [String] {
        platformCounts.map(\.name)
    }

    var shelfCount: Int {
        items.reduce(0) { $0 + (($1.deletedAt ?? "").isEmpty ? 1 : 0) }
    }

    var platformCounts: [(name: String, count: Int)] {
        var map: [String: Int] = [:]
        for item in items where (item.deletedAt ?? "").isEmpty {
            map[item.platform, default: 0] += 1
        }
        return map.keys.sorted().map { (name: $0, count: map[$0] ?? 0) }
    }

    var isPaired: Bool { KeychainStore.token() != nil && !serverURL.isEmpty }

    func bootstrap() async {
        do {
            db = try AppDatabase.open()
            try reload()
            if let token = KeychainStore.token(), let url = URL(string: serverURL), !serverURL.isEmpty {
                api.configure(baseURL: url, token: token)
                await runSync()
            }
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func reload() throws {
        items = try db.read { db in
            try LibraryItem.fetchAll(db)
        }
    }

    func upsert(_ item: LibraryItem) throws {
        try db.write { db in
            try item.save(db)
        }
        try reload()
        Task { await runSync() }
    }

    func add(_ item: LibraryItem) throws {
        try upsert(item)
    }

    func importCSV(_ data: Data) async {
        errorMessage = nil
        do {
            if isPaired {
                _ = try await api.importCSV(data)
                try await db.write { db in
                    try db.execute(sql: "DELETE FROM library_items")
                    try db.execute(sql: "DELETE FROM meta")
                }
                coverCache.removeAll()
                await runSync()
            } else {
                guard let text = String(data: data, encoding: .utf8) else {
                    errorMessage = "CSV must be UTF-8"
                    return
                }
                let parsed = try LibraryCSV.parse(text)
                try await db.write { db in
                    try db.execute(sql: "DELETE FROM library_items")
                    try db.execute(sql: "DELETE FROM meta")
                    for item in parsed {
                        try item.insert(db)
                    }
                }
                coverCache.removeAll()
                try reload()
            }
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func delete(_ item: LibraryItem) throws {
        var copy = item.touching()
        copy.deletedAt = LibraryItem.now()
        try upsert(copy)
    }

    func pair(urlString: String, password: String) async {
        errorMessage = nil
        let trimmed = Self.normalizedServerURL(urlString)
        guard let url = URL(string: trimmed), let scheme = url.scheme, scheme == "http" || scheme == "https" else {
            errorMessage = "Enter a URL like http://192.168.1.10:8080"
            return
        }
        do {
            let token = try await api.login(baseURL: url, password: password)
            try KeychainStore.setToken(token)
            serverURL = trimmed
            UserDefaults.standard.set(trimmed, forKey: "serverURL")
            api.configure(baseURL: url, token: token)
            await runSync()
        } catch {
            errorMessage = error.localizedDescription
        }
    }

    func forgetServer() {
        if let token = KeychainStore.token(), let url = URL(string: serverURL) {
            api.configure(baseURL: url, token: token)
            Task { try? await api.logout() }
        }
        KeychainStore.deleteToken()
        serverURL = ""
        UserDefaults.standard.removeObject(forKey: "serverURL")
        lastSync = nil
        UserDefaults.standard.removeObject(forKey: "lastSync")
        online = false
        igdbConfigured = false
        syncMessage = "Local only"
    }

    func runSync() async {
        guard isPaired, let db else { return }
        do {
            let result = try await sync.sync(db: db, api: api)
            igdbConfigured = result.igdbConfigured
            online = true
            lastSync = Date()
            UserDefaults.standard.set(lastSync, forKey: "lastSync")
            syncMessage = "Synced"
            try reload()
            await downloadCovers()
            try reload()
        } catch APIError.unauthorized {
            KeychainStore.deleteToken()
            online = false
            syncMessage = "Sign in again"
            errorMessage = "Server rejected the saved token."
        } catch {
            online = false
            syncMessage = "Offline"
        }
    }

    func cachedCoverImage(for coverId: String?) -> UIImage? {
        guard let coverId, !coverId.isEmpty else { return nil }
        if let cached = coverCache[coverId] {
            return cached
        }
        if let img = CoverFile.image(for: coverId) {
            coverCache[coverId] = img
            return img
        }
        return nil
    }

    func coverImage(for coverId: String?) async -> UIImage? {
        guard let coverId, !coverId.isEmpty else { return nil }
        if let cached = coverCache[coverId] {
            return cached
        }
        if let img = CoverFile.image(for: coverId) {
            coverCache[coverId] = img
            return img
        }
        guard isPaired, let data = try? await api.cover(id: coverId), let img = UIImage(data: data) else {
            return nil
        }
        try? data.write(to: AppDatabase.coverURL(id: coverId), options: .atomic)
        coverCache[coverId] = img
        return img
    }

    private func downloadCovers() async {
        for item in items where item.deletedAt == nil {
            _ = await coverImage(for: item.coverId)
        }
    }

    static func normalizedServerURL(_ raw: String) -> String {
        let s = raw.trimmingCharacters(in: .whitespacesAndNewlines)
            .trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        if s.hasPrefix("http://") || s.hasPrefix("https://") {
            return s
        }
        return "http://\(s)"
    }
}
