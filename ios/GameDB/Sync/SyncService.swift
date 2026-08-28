import Foundation
import GRDB

struct SyncResult {
    var igdbConfigured: Bool
}

final class SyncService {
    func sync(db: DatabaseQueue, api: APIClient) async throws -> SyncResult {
        let igdb = (try? await api.me()) ?? false
        let cursor: Int64 = try await db.read { db in
            let row = try Row.fetchOne(db, sql: "SELECT value FROM meta WHERE key = ?", arguments: ["sync_cursor"])
            guard let row else { return 0 }
            let raw: String = row["value"]
            return Int64(raw) ?? 0
        }
        let dirty: [LibraryItem] = try await db.read { db in
            try LibraryItem.filter(sql: "dirty = 1").fetchAll(db)
        }
        let response = try await api.sync(cursor: cursor, changes: dirty)
        // Full snapshot fills holes if the cursor skipped a seq (e.g. Super
        // Mario Sunshine sitting at seq 4 after the client already stored cursor 6).
        let snapshot = (try? await api.libraryItems()) ?? []
        try await db.write { db in
            for var item in response.changes {
                item.dirty = false
                try item.save(db)
            }
            let pendingIDs = Set(dirty.map(\.id))
            var remoteIDs = Set<String>()
            for var item in snapshot {
                remoteIDs.insert(item.id)
                if pendingIDs.contains(item.id) {
                    continue
                }
                item.dirty = false
                try item.save(db)
            }
            for var item in dirty {
                if response.changes.contains(where: { $0.id == item.id }) {
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
                arguments: ["sync_cursor", String(response.cursor)]
            )
        }
        return SyncResult(igdbConfigured: igdb)
    }
}
