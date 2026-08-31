package store

import (
	"context"
	"os"
	"path/filepath"
)

type CacheClearResult struct {
	Prices   int `json:"prices"`
	Covers   int `json:"covers"`
	Barcodes int `json:"barcodes"`
	Games    int `json:"games"`
}

// ClearDerivedCache deletes prices, cover files, barcode lookups, and cached
// IGDB search rows. Library items (titles, dates, barcodes, IGDB ids) stay.
func (s *Store) ClearDerivedCache(ctx context.Context) (CacheClearResult, error) {
	var out CacheClearResult
	var err error
	if out.Prices, err = deleteCount(ctx, s, `DELETE FROM price_quotes`); err != nil {
		return out, err
	}
	if out.Barcodes, err = deleteCount(ctx, s, `DELETE FROM barcode_cache`); err != nil {
		return out, err
	}
	if out.Games, err = deleteCount(ctx, s, `DELETE FROM igdb_games`); err != nil {
		return out, err
	}
	if _, err = s.DB.ExecContext(ctx, `DELETE FROM covers`); err != nil {
		return out, err
	}
	dir := filepath.Join(s.DataDir, "covers")
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return out, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil && !os.IsNotExist(err) {
			return out, err
		}
		out.Covers++
	}
	return out, nil
}

func deleteCount(ctx context.Context, s *Store, q string) (int, error) {
	res, err := s.DB.ExecContext(ctx, q)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}
