package store

import (
	"database/sql"
	"fmt"
)

func migrate(db *sql.DB) error {
	if err := addColumnIfMissing(db, "library_items", "barcode", "TEXT"); err != nil {
		return err
	}
	if err := addColumnIfMissing(db, "library_items", "box_cover_id", "TEXT"); err != nil {
		return err
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS barcode_cache (
			barcode        TEXT PRIMARY KEY NOT NULL,
			product_title  TEXT,
			query          TEXT,
			source         TEXT,
			igdb_game_id   INTEGER,
			updated_at     TEXT NOT NULL
		)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_library_items_barcode ON library_items (barcode)`); err != nil {
		return err
	}
	if _, err := db.Exec(`
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
		)`); err != nil {
		return err
	}
	if err := addColumnIfMissing(db, "price_quotes", "source", "TEXT"); err != nil {
		return err
	}
	if err := addColumnIfMissing(db, "price_quotes", "listings", "INTEGER"); err != nil {
		return err
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tgdb_platforms (
			id         INTEGER PRIMARY KEY,
			name       TEXT NOT NULL,
			alias      TEXT,
			updated_at TEXT NOT NULL
		)`); err != nil {
		return err
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS box_cover_misses (
			item_id    TEXT PRIMARY KEY NOT NULL,
			query_key  TEXT NOT NULL,
			fetched_at TEXT NOT NULL
		)`); err != nil {
		return err
	}
	return nil
}

func addColumnIfMissing(db *sql.DB, table, column, decl string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, decl))
	return err
}
