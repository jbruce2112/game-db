package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"game-db/internal/model"
)

type PriceQuote struct {
	ItemID      string
	QueryKey    string
	PCID        string
	ProductName string
	ConsoleName string
	URL         string
	Source      string
	Listings    int
	LooseCents  *int
	CIBCents    *int
	NewCents    *int
	Status      string
	FetchedAt   time.Time
}

func QuoteKey(item model.Item) string {
	barcode := ""
	if item.Barcode != nil {
		barcode = *item.Barcode
	}
	region := ""
	if item.Region != nil {
		region = *item.Region
	}
	return strings.Join([]string{barcode, item.Title, item.Platform, region}, "\x1f")
}

func (q PriceQuote) ToValue() *model.Value {
	if q.Status != "ok" || q.PCID == "" {
		return nil
	}
	return &model.Value{
		PCID:        q.PCID,
		ProductName: q.ProductName,
		ConsoleName: q.ConsoleName,
		URL:         q.URL,
		Source:      q.Source,
		Listings:    q.Listings,
		LooseCents:  q.LooseCents,
		CIBCents:    q.CIBCents,
		NewCents:    q.NewCents,
	}
}

func (s *Store) UpsertPriceQuote(ctx context.Context, q PriceQuote) error {
	if q.ItemID == "" {
		return nil
	}
	if q.FetchedAt.IsZero() {
		q.FetchedAt = time.Now()
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO price_quotes (
			item_id, query_key, pc_id, product_name, console_name, url, source, listings,
			loose_cents, cib_cents, new_cents, status, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(item_id) DO UPDATE SET
			query_key=excluded.query_key,
			pc_id=excluded.pc_id,
			product_name=excluded.product_name,
			console_name=excluded.console_name,
			url=excluded.url,
			source=excluded.source,
			listings=excluded.listings,
			loose_cents=excluded.loose_cents,
			cib_cents=excluded.cib_cents,
			new_cents=excluded.new_cents,
			status=excluded.status,
			fetched_at=excluded.fetched_at`,
		q.ItemID, q.QueryKey, nullStr(q.PCID), nullStr(q.ProductName), nullStr(q.ConsoleName),
		nullStr(q.URL), nullStr(q.Source), q.Listings,
		nullIntPtr(q.LooseCents), nullIntPtr(q.CIBCents), nullIntPtr(q.NewCents),
		q.Status, model.FormatTime(q.FetchedAt),
	)
	return err
}

func (s *Store) PriceQuote(ctx context.Context, itemID string) (PriceQuote, bool, error) {
	q, err := scanQuote(s.DB.QueryRowContext(ctx, `
		SELECT item_id, query_key, pc_id, product_name, console_name, url, source, listings,
			loose_cents, cib_cents, new_cents, status, fetched_at
		FROM price_quotes WHERE item_id = ?`, itemID))
	if err == sql.ErrNoRows {
		return PriceQuote{}, false, nil
	}
	if err != nil {
		return PriceQuote{}, false, err
	}
	return q, true, nil
}

func (s *Store) PriceQuotes(ctx context.Context, ids []string) (map[string]PriceQuote, error) {
	out := map[string]PriceQuote{}
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT item_id, query_key, pc_id, product_name, console_name, url, source, listings,
			loose_cents, cib_cents, new_cents, status, fetched_at
		FROM price_quotes WHERE item_id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		q, err := scanQuote(rows)
		if err != nil {
			return nil, err
		}
		out[q.ItemID] = q
	}
	return out, rows.Err()
}

func scanQuote(row scanner) (PriceQuote, error) {
	var (
		q                                                          PriceQuote
		pcID, name, console, urlStr, fetched, queryKey, sourceName sql.NullString
		loose, cib, neu, listings                                  sql.NullInt64
	)
	err := row.Scan(&q.ItemID, &queryKey, &pcID, &name, &console, &urlStr, &sourceName, &listings, &loose, &cib, &neu, &q.Status, &fetched)
	if err != nil {
		return PriceQuote{}, err
	}
	q.QueryKey = queryKey.String
	q.PCID = pcID.String
	q.ProductName = name.String
	q.ConsoleName = console.String
	q.URL = urlStr.String
	q.Source = sourceName.String
	if listings.Valid {
		q.Listings = int(listings.Int64)
	}
	if loose.Valid {
		v := int(loose.Int64)
		q.LooseCents = &v
	}
	if cib.Valid {
		v := int(cib.Int64)
		q.CIBCents = &v
	}
	if neu.Valid {
		v := int(neu.Int64)
		q.NewCents = &v
	}
	if fetched.Valid {
		q.FetchedAt, _ = model.ParseTime(fetched.String)
	}
	return q, nil
}

func nullIntPtr(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func QuoteStale(q PriceQuote, key string, now time.Time) bool {
	if q.QueryKey != key {
		return true
	}
	age := now.Sub(q.FetchedAt)
	if q.Status == "ok" {
		return age > 24*time.Hour
	}
	return age > time.Hour
}
