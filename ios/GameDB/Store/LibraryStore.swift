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
    var pricechartingConfigured: Bool = false
    var quotes: [String: PriceQuote] = [:]
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

    var stats: ShelfStats {
        ShelfStats(items: items, quotes: quotes)
    }

    var platformCounts: [(name: String, count: Int)] {
        var map: [String: Int] = [:]
        for item in items where (item.deletedAt ?? "").isEmpty {
            map[item.platform, default: 0] += 1
        }
        return map.keys.sorted().map { (name: $0, count: map[$0] ?? 0) }
    }

    var shelfValueCents: Int? {
        var sum = 0
        var n = 0
        for item in filtered {
            if let cents = quotes[item.id]?.cents(for: item.completeness) {
                sum += cents
                n += 1
            }
        }
        return n > 0 ? sum : nil
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
        quotes = try db.read { db in
            let rows = try Row.fetchAll(db, sql: "SELECT * FROM price_quotes")
            var map: [String: PriceQuote] = [:]
            for row in rows {
                let id: String = row["item_id"]
                map[id] = PriceQuote(
                    pcId: row["pc_id"],
                    productName: row["product_name"],
                    consoleName: row["console_name"],
                    url: row["url"],
                    source: row["source"],
                    listings: row["listings"],
                    looseCents: row["loose_cents"],
                    cibCents: row["cib_cents"],
                    newCents: row["new_cents"]
                )
            }
            return map
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

    func clearCache() async {
        errorMessage = nil
        if isPaired {
            do {
                try await api.clearCache()
            } catch {
                errorMessage = error.localizedDescription
                return
            }
        }
        do {
            try await db.write { db in
                try db.execute(sql: "DELETE FROM price_quotes")
            }
            quotes = [:]
            coverCache.removeAll()
            let files = (try? FileManager.default.contentsOfDirectory(
                at: AppDatabase.coversDir,
                includingPropertiesForKeys: nil
            )) ?? []
            for url in files {
                try? FileManager.default.removeItem(at: url)
            }
            try reload()
            if isPaired {
                await runSync()
            }
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
        pricechartingConfigured = false
        syncMessage = "Local only"
    }

    func runSync() async {
        guard isPaired, let db else { return }
        do {
            let result = try await sync.sync(db: db, api: api)
            igdbConfigured = result.igdbConfigured
            pricechartingConfigured = result.pricechartingConfigured
            online = true
            lastSync = Date()
            UserDefaults.standard.set(lastSync, forKey: "lastSync")
            syncMessage = "Synced"
            try reload()
            await downloadCovers()
            await downloadPrices()
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

    private func downloadPrices() async {
        guard isPaired else { return }
        let remote = (try? await api.libraryItems()) ?? []
        guard let db else { return }
        do {
            try await db.write { db in
                for item in remote {
                    guard let q = item.value else { continue }
                    try db.execute(sql: """
                        INSERT INTO price_quotes (item_id, pc_id, product_name, console_name, url, source, listings, loose_cents, cib_cents, new_cents)
                        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                        ON CONFLICT(item_id) DO UPDATE SET
                            pc_id=excluded.pc_id,
                            product_name=excluded.product_name,
                            console_name=excluded.console_name,
                            url=excluded.url,
                            source=excluded.source,
                            listings=excluded.listings,
                            loose_cents=excluded.loose_cents,
                            cib_cents=excluded.cib_cents,
                            new_cents=excluded.new_cents
                        """, arguments: [
                        item.id, q.pcId, q.productName, q.consoleName, q.url,
                        q.source, q.listings, q.looseCents, q.cibCents, q.newCents,
                    ])
                }
            }
        } catch {
            errorMessage = error.localizedDescription
        }
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
