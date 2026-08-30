package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"game-db/internal/model"
	"game-db/internal/syncmerge"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type Store struct {
	DB      *sql.DB
	DataDir string
}

func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dataDir, "covers"), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir %s: %w", dataDir, err)
	}
	probe := filepath.Join(dataDir, ".write-test")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return nil, fmt.Errorf("data dir %s is not writable: %w", dataDir, err)
	}
	_ = os.Remove(probe)

	dbPath := filepath.Join(dataDir, "game-db.sqlite")
	dsn := sqliteDSN(dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{DB: db, DataDir: dataDir}, nil
}

func sqliteDSN(dbPath string) string {
	p := filepath.ToSlash(dbPath)
	q := "?mode=rwc&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	if filepath.IsAbs(dbPath) || strings.HasPrefix(p, "/") {
		return "file://" + p + q
	}
	return "file:" + p + q
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) CreateToken(ctx context.Context, token string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO tokens (token, created_at) VALUES (?, ?)`, token, model.FormatTime(time.Now()))
	return err
}

func (s *Store) ValidToken(ctx context.Context, token string) bool {
	if token == "" {
		return false
	}
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM tokens WHERE token = ?`, token).Scan(&n)
	return err == nil && n == 1
}

func (s *Store) DeleteToken(ctx context.Context, token string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM tokens WHERE token = ?`, token)
	return err
}

type ListFilter struct {
	Platform string
	Query    string
	Sort     string
}

func (s *Store) List(ctx context.Context, f ListFilter) ([]model.Item, error) {
	q := `SELECT ` + itemCols + ` FROM library_items WHERE deleted_at IS NULL`
	args := []any{}
	if f.Platform != "" {
		q += ` AND platform = ?`
		args = append(args, f.Platform)
	}
	if f.Query != "" {
		q += ` AND lower(title) LIKE ?`
		args = append(args, "%"+strings.ToLower(f.Query)+"%")
	}
	if f.Sort == "added" {
		q += ` ORDER BY created_at DESC, title COLLATE NOCASE`
	} else {
		q += ` ORDER BY title COLLATE NOCASE`
	}
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, err := scanItems(rows)
	if err != nil {
		return nil, err
	}
	s.attachCoverURLs(ctx, items)
	return items, nil
}

func (s *Store) Get(ctx context.Context, id string) (model.Item, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT `+itemCols+` FROM library_items WHERE id = ?`, id)
	item, err := scanItem(row)
	if err != nil {
		return model.Item{}, err
	}
	s.attachCoverURL(ctx, &item)
	return item, nil
}

func (s *Store) ReplaceAll(ctx context.Context, items []model.Item) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var maxSeq int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sync_seq), 0) FROM library_items`).Scan(&maxSeq); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM library_items`); err != nil {
		return err
	}
	now := model.TimeUTC(time.Now())
	for i, item := range items {
		item.SyncSeq = maxSeq + int64(i) + 1
		item.UpdatedAt = now
		item.DeletedAt = nil
		if item.CreatedAt.IsZero() {
			item.CreatedAt = now
		}
		if err := insertItem(ctx, tx, item); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Insert(ctx context.Context, item model.Item) (model.Item, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return model.Item{}, err
	}
	defer func() { _ = tx.Rollback() }()
	seq, err := nextSeq(ctx, tx)
	if err != nil {
		return model.Item{}, err
	}
	item.SyncSeq = seq
	if err := insertItem(ctx, tx, item); err != nil {
		return model.Item{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Item{}, err
	}
	s.attachCoverURL(ctx, &item)
	return item, nil
}

func (s *Store) Replace(ctx context.Context, item model.Item) (model.Item, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return model.Item{}, err
	}
	defer func() { _ = tx.Rollback() }()
	seq, err := nextSeq(ctx, tx)
	if err != nil {
		return model.Item{}, err
	}
	item.SyncSeq = seq
	if err := updateItem(ctx, tx, item); err != nil {
		return model.Item{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Item{}, err
	}
	s.attachCoverURL(ctx, &item)
	return item, nil
}

type SyncResult struct {
	Cursor  int64        `json:"cursor"`
	Changes []model.Item `json:"changes"`
}

func (s *Store) Sync(ctx context.Context, cursor int64, incoming []model.Item, now time.Time) (SyncResult, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return SyncResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	echoIDs := map[string]struct{}{}
	for i := range incoming {
		item := incoming[i]
		item.UpdatedAt = syncmerge.ClampUpdatedAt(item.UpdatedAt, now)
		item.CreatedAt = model.TimeUTC(item.CreatedAt)
		if item.CreatedAt.IsZero() {
			item.CreatedAt = item.UpdatedAt
		}
		item.Completeness = model.NormalizeCompleteness(item.Completeness)
		if item.Region != nil {
			item.Region = model.NormalizeRegion(*item.Region)
		}
		existing, err := getItemTx(ctx, tx, item.ID)
		if err == sql.ErrNoRows {
			seq, err := nextSeq(ctx, tx)
			if err != nil {
				return SyncResult{}, err
			}
			item.SyncSeq = seq
			if err := insertItem(ctx, tx, item); err != nil {
				return SyncResult{}, err
			}
			continue
		}
		if err != nil {
			return SyncResult{}, err
		}
		if syncmerge.IncomingWins(existing, item) {
			seq, err := nextSeq(ctx, tx)
			if err != nil {
				return SyncResult{}, err
			}
			item.CreatedAt = existing.CreatedAt
			item.SyncSeq = seq
			if err := updateItem(ctx, tx, item); err != nil {
				return SyncResult{}, err
			}
			continue
		}
		echoIDs[item.ID] = struct{}{}
	}

	out := []model.Item{}
	seen := map[string]struct{}{}

	rows, err := tx.QueryContext(ctx, `SELECT `+itemCols+` FROM library_items WHERE sync_seq > ? ORDER BY sync_seq`, cursor)
	if err != nil {
		return SyncResult{}, err
	}
	pulled, err := scanItems(rows)
	rows.Close()
	if err != nil {
		return SyncResult{}, err
	}
	for _, it := range pulled {
		out = append(out, it)
		seen[it.ID] = struct{}{}
	}
	for id := range echoIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		it, err := getItemTx(ctx, tx, id)
		if err != nil {
			return SyncResult{}, err
		}
		out = append(out, it)
	}

	var maxSeq int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sync_seq), 0) FROM library_items`).Scan(&maxSeq); err != nil {
		return SyncResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return SyncResult{}, err
	}
	s.attachCoverURLs(ctx, out)
	return SyncResult{Cursor: maxSeq, Changes: out}, nil
}

func (s *Store) SaveCover(ctx context.Context, id, igdbImageID, contentType, filename string) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO covers (id, igdb_image_id, content_type, filename, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			igdb_image_id=excluded.igdb_image_id,
			content_type=excluded.content_type,
			filename=excluded.filename`,
		id, nullStr(igdbImageID), contentType, filename, model.FormatTime(time.Now()))
	return err
}

func (s *Store) GetCover(ctx context.Context, id string) (contentType, absPath string, err error) {
	var filename string
	err = s.DB.QueryRowContext(ctx, `SELECT content_type, filename FROM covers WHERE id = ?`, id).
		Scan(&contentType, &filename)
	if err != nil {
		return "", "", err
	}
	return contentType, filepath.Join(s.DataDir, "covers", filename), nil
}

func (s *Store) CoverExists(ctx context.Context, id *string) bool {
	if id == nil || *id == "" {
		return false
	}
	_, path, err := s.GetCover(ctx, *id)
	if err != nil {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func (s *Store) LinkIGDB(ctx context.Context, itemID string, gameID int64, platformID *int64) error {
	if itemID == "" || gameID == 0 {
		return nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	seq, err := nextSeq(ctx, tx)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE library_items SET igdb_game_id = ?, igdb_platform_id = COALESCE(?, igdb_platform_id), sync_seq = ?
		WHERE id = ?`, gameID, nullInt(platformID), seq, itemID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SetCoverID(ctx context.Context, itemID, coverID string) error {
	if itemID == "" || coverID == "" {
		return nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var current sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT cover_id FROM library_items WHERE id = ?`, itemID).Scan(&current); err != nil {
		return err
	}
	if current.Valid && current.String == coverID {
		return nil
	}
	seq, err := nextSeq(ctx, tx)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE library_items SET cover_id = ?, sync_seq = ? WHERE id = ?`, coverID, seq, itemID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) attachCoverURLs(ctx context.Context, items []model.Item) {
	for i := range items {
		s.attachCoverURL(ctx, &items[i])
	}
}

func (s *Store) attachCoverURL(ctx context.Context, item *model.Item) {
	if s.CoverExists(ctx, item.CoverID) {
		item.CoverURL = model.CoverURL(item.CoverID)
		return
	}
	if item.IGDBGameID != nil {
		u := "/v1/library/" + item.ID + "/cover"
		item.CoverURL = &u
		return
	}
	item.CoverURL = nil
}

type Platform struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Slug         *string `json:"slug"`
	Abbreviation *string `json:"abbreviation"`
}

func (s *Store) ListPlatforms(ctx context.Context) ([]Platform, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, name, slug, abbreviation FROM igdb_platforms ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Platform
	for rows.Next() {
		var p Platform
		if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Abbreviation); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) PlatformsStale(ctx context.Context, maxAge time.Duration) (bool, error) {
	var n int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM igdb_platforms`).Scan(&n); err != nil {
		return true, err
	}
	if n < 20 {
		return true, nil
	}
	var raw sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT MAX(updated_at) FROM igdb_platforms`).Scan(&raw)
	if err != nil {
		return true, err
	}
	if !raw.Valid {
		return true, nil
	}
	t, err := model.ParseTime(raw.String)
	if err != nil {
		return true, nil
	}
	return time.Since(t) > maxAge, nil
}

func (s *Store) UpsertPlatforms(ctx context.Context, platforms []Platform) error {
	now := model.FormatTime(time.Now())
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO igdb_platforms (id, name, slug, abbreviation, category, updated_at)
		VALUES (?, ?, ?, ?, NULL, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			slug=excluded.slug,
			abbreviation=excluded.abbreviation,
			updated_at=excluded.updated_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, p := range platforms {
		var slug, abbr any
		if p.Slug != nil {
			slug = *p.Slug
		}
		if p.Abbreviation != nil {
			abbr = *p.Abbreviation
		}
		if _, err := stmt.ExecContext(ctx, p.ID, p.Name, slug, abbr, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) PlatformName(ctx context.Context, id int64) (string, error) {
	var name string
	err := s.DB.QueryRowContext(ctx, `SELECT name FROM igdb_platforms WHERE id = ?`, id).Scan(&name)
	return name, err
}

const itemCols = `id, title, platform, igdb_platform_id, region, completeness, notes, igdb_game_id, cover_id, barcode, created_at, updated_at, deleted_at, sync_seq`

type scanner interface {
	Scan(dest ...any) error
}

func scanItem(row scanner) (model.Item, error) {
	var (
		it                                  model.Item
		igdbPlat, igdbGame                  sql.NullInt64
		created, upd                        string
		regionN, coverN, barcodeN, deletedN sql.NullString
	)
	err := row.Scan(
		&it.ID, &it.Title, &it.Platform, &igdbPlat, &regionN, &it.Completeness, &it.Notes,
		&igdbGame, &coverN, &barcodeN, &created, &upd, &deletedN, &it.SyncSeq,
	)
	if err != nil {
		return model.Item{}, err
	}
	if igdbPlat.Valid {
		v := igdbPlat.Int64
		it.IGDBPlatformID = &v
	}
	if igdbGame.Valid {
		v := igdbGame.Int64
		it.IGDBGameID = &v
	}
	if regionN.Valid {
		region := regionN.String
		it.Region = &region
	}
	if coverN.Valid {
		coverID := coverN.String
		it.CoverID = &coverID
	}
	if barcodeN.Valid && barcodeN.String != "" {
		b := barcodeN.String
		it.Barcode = &b
	}
	it.CreatedAt, err = model.ParseTime(created)
	if err != nil {
		return model.Item{}, err
	}
	it.UpdatedAt, err = model.ParseTime(upd)
	if err != nil {
		return model.Item{}, err
	}
	if deletedN.Valid {
		t, err := model.ParseTime(deletedN.String)
		if err != nil {
			return model.Item{}, err
		}
		it.DeletedAt = &t
	}
	it.CoverURL = model.CoverURL(it.CoverID)
	return it, nil
}

func scanItems(rows *sql.Rows) ([]model.Item, error) {
	var out []model.Item
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	if out == nil {
		out = []model.Item{}
	}
	return out, rows.Err()
}

func getItemTx(ctx context.Context, tx *sql.Tx, id string) (model.Item, error) {
	return scanItem(tx.QueryRowContext(ctx, `SELECT `+itemCols+` FROM library_items WHERE id = ?`, id))
}

func nextSeq(ctx context.Context, tx *sql.Tx) (int64, error) {
	var seq int64
	err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(sync_seq), 0) + 1 FROM library_items`).Scan(&seq)
	return seq, err
}

func insertItem(ctx context.Context, tx *sql.Tx, item model.Item) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO library_items (
			id, title, platform, igdb_platform_id, region, completeness, notes,
			igdb_game_id, cover_id, barcode, created_at, updated_at, deleted_at, sync_seq
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.Title, item.Platform, nullInt(item.IGDBPlatformID), nullStrPtr(item.Region),
		item.Completeness, item.Notes, nullInt(item.IGDBGameID), nullStrPtr(item.CoverID),
		nullStrPtr(item.Barcode),
		model.FormatTime(item.CreatedAt), model.FormatTime(item.UpdatedAt),
		nullTime(item.DeletedAt), item.SyncSeq,
	)
	return err
}

func updateItem(ctx context.Context, tx *sql.Tx, item model.Item) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE library_items SET
			title=?, platform=?, igdb_platform_id=?, region=?, completeness=?, notes=?,
			igdb_game_id=?, cover_id=?, barcode=?, created_at=?, updated_at=?, deleted_at=?, sync_seq=?
		WHERE id=?`,
		item.Title, item.Platform, nullInt(item.IGDBPlatformID), nullStrPtr(item.Region),
		item.Completeness, item.Notes, nullInt(item.IGDBGameID), nullStrPtr(item.CoverID),
		nullStrPtr(item.Barcode),
		model.FormatTime(item.CreatedAt), model.FormatTime(item.UpdatedAt),
		nullTime(item.DeletedAt), item.SyncSeq, item.ID,
	)
	return err
}

func nullInt(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullStrPtr(v *string) any {
	if v == nil || *v == "" {
		return nil
	}
	return *v
}

func nullStr(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func nullTime(v *time.Time) any {
	if v == nil {
		return nil
	}
	return model.FormatTime(*v)
}
