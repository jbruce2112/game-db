import Foundation
import GRDB

struct SyncResult {
    var igdbConfigured: Bool
}

final class SyncService {
    func sync(db: DatabaseQueue, api: APIClient) async throws -> SyncResult {
        let igdb = (try? await api.me()) ?? false
        var cursor: Int64 = try await db.read { db in
            let row = try Row.fetchOne(db, sql: "SELECT value FROM meta WHERE key = ?", arguments: ["sync_cursor"])
            guard let row else { return 0 }
            let raw: String = row["value"]
            return Int64(raw) ?? 0
        }
        var outgoing: [LibraryItem] = try await db.read { db in
            try LibraryItem.filter(sql: "dirty = 1").fetchAll(db)
        }

        // Empty server (fresh Docker volume) + phone already synced to an old
        // DB: locals are dirty=0 so a delta push sends nothing. Treat an empty
        // remote shelf as first pairing and upload the whole local library.
        let snapshotBefore = (try? await api.libraryItems()) ?? []
        if snapshotBefore.isEmpty {
            let locals: [LibraryItem] = try await db.read { db in
                try LibraryItem.fetchAll(db)
            }
            if !locals.isEmpty {
                outgoing = locals
                cursor = 0
            }
        }

        var response = try await api.sync(cursor: cursor, changes: outgoing)
        if response.cursor < cursor, snapshotBefore.isEmpty == false {
            // Server seq went backwards (restored/replaced DB). Full push.
            let locals: [LibraryItem] = try await db.read { db in
                try LibraryItem.fetchAll(db)
            }
            response = try await api.sync(cursor: 0, changes: locals)
            outgoing = locals
        }

        let snapshot = (try? await api.libraryItems()) ?? snapshotBefore
        let applied = response
        let sent = outgoing
        try await db.write { db in
            for var item in applied.changes {
                item.dirty = false
                try item.save(db)
            }
            let pendingIDs = Set(sent.map(\.id))
            var remoteIDs = Set<String>()
            for var item in snapshot {
                remoteIDs.insert(item.id)
                if pendingIDs.contains(item.id) {
                    continue
                }
                if item.coverId == nil, let existing = try LibraryItem.fetchOne(db, key: item.id), existing.coverId != nil {
                    item.coverId = existing.coverId
                }
                item.dirty = false
                try item.save(db)
            }
            for var item in sent {
                if applied.changes.contains(where: { $0.id == item.id }) {
                    continue
                }
                item.dirty = false
                try item.save(db)
            }
            if !snapshot.isEmpty {
                let locals = try LibraryItem.fetchAll(db)
                for var item in locals {
                    if remoteIDs.contains(item.id) || item.dirty {
                        continue
                    }
                    if item.deletedAt == nil || item.deletedAt?.isEmpty == true {
                        item.deletedAt = LibraryItem.now()
                        item.dirty = false
                        try item.save(db)
                    }
                }
            }
            try db.execute(
                sql: "INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
                arguments: ["sync_cursor", String(applied.cursor)]
            )
        }
        return SyncResult(igdbConfigured: igdb)
    }
}
