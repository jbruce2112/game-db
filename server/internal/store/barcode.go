package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"game-db/internal/model"
)

type OwnedCopy struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Platform string `json:"platform"`
}

func (s *Store) ListByBarcodes(ctx context.Context, codes []string) ([]OwnedCopy, error) {
	if len(codes) == 0 {
		return []OwnedCopy{}, nil
	}
	placeholders := make([]string, len(codes))
	args := make([]any, len(codes))
	for i, c := range codes {
		placeholders[i] = "?"
		args[i] = c
	}
	q := `SELECT id, title, platform FROM library_items
		WHERE deleted_at IS NULL AND barcode IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY created_at`
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OwnedCopy{}
	for rows.Next() {
		var o OwnedCopy
		if err := rows.Scan(&o.ID, &o.Title, &o.Platform); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

type BarcodeCache struct {
	Barcode      string
	ProductTitle string
	Query        string
	Source       string
	IGDBGameID   *int64
	UpdatedAt    time.Time
	Found        bool
}

func (s *Store) GetBarcodeCache(ctx context.Context, codes []string) (BarcodeCache, bool, error) {
	for _, code := range codes {
		var (
			c      BarcodeCache
			igdb   sql.NullInt64
			upd    string
			title  sql.NullString
			query  sql.NullString
			source sql.NullString
		)
		err := s.DB.QueryRowContext(ctx, `
			SELECT barcode, product_title, query, source, igdb_game_id, updated_at
			FROM barcode_cache WHERE barcode = ?`, code).
			Scan(&c.Barcode, &title, &query, &source, &igdb, &upd)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return BarcodeCache{}, false, err
		}
		c.ProductTitle = title.String
		c.Query = query.String
		c.Source = source.String
		if igdb.Valid {
			v := igdb.Int64
			c.IGDBGameID = &v
		}
		c.UpdatedAt, _ = model.ParseTime(upd)
		c.Found = c.ProductTitle != ""
		return c, true, nil
	}
	return BarcodeCache{}, false, nil
}

func (s *Store) PutBarcodeCache(ctx context.Context, c BarcodeCache) error {
	if c.Barcode == "" {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO barcode_cache (barcode, product_title, query, source, igdb_game_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(barcode) DO UPDATE SET
			product_title=excluded.product_title,
			query=excluded.query,
			source=excluded.source,
			igdb_game_id=COALESCE(excluded.igdb_game_id, barcode_cache.igdb_game_id),
			updated_at=excluded.updated_at`,
		c.Barcode, nullStr(c.ProductTitle), nullStr(c.Query), nullStr(c.Source),
		nullInt(c.IGDBGameID), model.FormatTime(time.Now()),
	)
	return err
}

func (s *Store) RememberBarcodeGame(ctx context.Context, code string, igdbGameID int64) error {
	if code == "" || igdbGameID == 0 {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO barcode_cache (barcode, product_title, query, source, igdb_game_id, updated_at)
		VALUES (?, NULL, NULL, 'library', ?, ?)
		ON CONFLICT(barcode) DO UPDATE SET
			igdb_game_id=excluded.igdb_game_id,
			updated_at=excluded.updated_at`,
		code, igdbGameID, model.FormatTime(time.Now()),
	)
	return err
}
