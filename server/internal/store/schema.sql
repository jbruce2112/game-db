-- Canonical SQLite schema for the game-db server.
-- iOS GRDB mirrors library_items plus client-only columns (dirty, meta).

CREATE TABLE IF NOT EXISTS library_items (
    id               TEXT PRIMARY KEY NOT NULL,
    title            TEXT NOT NULL,
    platform         TEXT NOT NULL,
    igdb_platform_id INTEGER,
    region           TEXT,
    completeness     TEXT NOT NULL DEFAULT 'unknown',
    notes            TEXT NOT NULL DEFAULT '',
    igdb_game_id     INTEGER,
    cover_id         TEXT,
    box_cover_id     TEXT,
    barcode          TEXT,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL,
    deleted_at       TEXT,
    sync_seq         INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_library_items_sync_seq ON library_items (sync_seq);
CREATE INDEX IF NOT EXISTS idx_library_items_platform ON library_items (platform);
CREATE INDEX IF NOT EXISTS idx_library_items_title ON library_items (title COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS price_quotes (
    item_id      TEXT PRIMARY KEY NOT NULL,
    query_key    TEXT NOT NULL,
    pc_id        TEXT,
    product_name TEXT,
    console_name TEXT,
    url          TEXT,
    source       TEXT,
    listings     INTEGER,
    loose_cents  INTEGER,
    cib_cents    INTEGER,
    new_cents    INTEGER,
    status       TEXT NOT NULL,
    fetched_at   TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS barcode_cache (
    barcode        TEXT PRIMARY KEY NOT NULL,
    product_title  TEXT,
    query          TEXT,
    source         TEXT,
    igdb_game_id   INTEGER,
    updated_at     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tokens (
    token      TEXT PRIMARY KEY NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS covers (
    id            TEXT PRIMARY KEY NOT NULL,
    igdb_image_id TEXT,
    content_type  TEXT NOT NULL,
    filename      TEXT NOT NULL,
    created_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS igdb_platforms (
    id           INTEGER PRIMARY KEY,
    name         TEXT NOT NULL,
    slug         TEXT,
    abbreviation TEXT,
    category     INTEGER,
    updated_at   TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS igdb_games (
    id                 INTEGER PRIMARY KEY,
    name               TEXT NOT NULL,
    summary            TEXT,
    cover_image_id     TEXT,
    first_release_date INTEGER,
    platforms_json     TEXT NOT NULL DEFAULT '[]',
    updated_at         TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tgdb_platforms (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    alias      TEXT,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS box_cover_misses (
    item_id    TEXT PRIMARY KEY NOT NULL,
    query_key  TEXT NOT NULL,
    fetched_at TEXT NOT NULL
);
