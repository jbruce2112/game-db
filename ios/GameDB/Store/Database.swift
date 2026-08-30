import Foundation
import GRDB

enum AppDatabase {
    static func open() throws -> DatabaseQueue {
        let dir = try FileManager.default.url(
            for: .applicationSupportDirectory,
            in: .userDomainMask,
            appropriateFor: nil,
            create: true
        ).appendingPathComponent("game-db", isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let dbURL = dir.appendingPathComponent("game-db.sqlite")
        var config = Configuration()
        config.prepareDatabase { db in
            try db.execute(sql: "PRAGMA foreign_keys = ON")
        }
        let db = try DatabaseQueue(path: dbURL.path, configuration: config)
        try migrator.migrate(db)
        return db
    }

    static var coversDir: URL {
        let dir = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("game-db/covers", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir
    }

    static func coverURL(id: String) -> URL {
        coversDir.appendingPathComponent("\(id).jpg")
    }

    static func coverURLCandidates(id: String) -> [URL] {
        [
            coversDir.appendingPathComponent("\(id).jpg"),
            coversDir.appendingPathComponent("\(id).img"),
            coversDir.appendingPathComponent(id),
        ]
    }

    private static var migrator: DatabaseMigrator {
        var migrator = DatabaseMigrator()
        migrator.registerMigration("v1") { db in
            try db.execute(sql: """
                CREATE TABLE library_items (
                    id TEXT PRIMARY KEY NOT NULL,
                    title TEXT NOT NULL,
                    platform TEXT NOT NULL,
                    igdb_platform_id INTEGER,
                    region TEXT,
                    completeness TEXT NOT NULL DEFAULT 'unknown',
                    notes TEXT NOT NULL DEFAULT '',
                    igdb_game_id INTEGER,
                    cover_id TEXT,
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    deleted_at TEXT,
                    sync_seq INTEGER NOT NULL DEFAULT 0,
                    dirty INTEGER NOT NULL DEFAULT 1
                );
                CREATE TABLE meta (
                    key TEXT PRIMARY KEY NOT NULL,
                    value TEXT NOT NULL
                );
                """)
        }
        migrator.registerMigration("v2-barcode") { db in
            try db.execute(sql: "ALTER TABLE library_items ADD COLUMN barcode TEXT")
        }
        migrator.registerMigration("v3-price-quotes") { db in
            try db.execute(sql: """
                CREATE TABLE price_quotes (
                    item_id TEXT PRIMARY KEY NOT NULL,
                    pc_id TEXT NOT NULL,
                    product_name TEXT NOT NULL,
                    console_name TEXT NOT NULL DEFAULT '',
                    url TEXT NOT NULL DEFAULT '',
                    loose_cents INTEGER,
                    cib_cents INTEGER,
                    new_cents INTEGER
                )
                """)
        }
        migrator.registerMigration("v4-price-source") { db in
            try db.execute(sql: "ALTER TABLE price_quotes ADD COLUMN source TEXT")
            try db.execute(sql: "ALTER TABLE price_quotes ADD COLUMN listings INTEGER")
        }
        return migrator
    }
}
