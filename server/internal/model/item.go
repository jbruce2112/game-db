package model

import (
	"strings"
	"time"
)

const CompletenessUnknown = "unknown"
const CompletenessLoose = "loose"
const CompletenessCIB = "cib"
const CompletenessNew = "new"

type Item struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Platform       string     `json:"platform"`
	IGDBPlatformID *int64     `json:"igdb_platform_id"`
	Region         *string    `json:"region"`
	Completeness   string     `json:"completeness"`
	Notes          string     `json:"notes"`
	IGDBGameID     *int64     `json:"igdb_game_id"`
	CoverID        *string    `json:"cover_id"`
	CoverURL       *string    `json:"cover_url,omitempty"`
	Barcode        *string    `json:"barcode"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at"`
	SyncSeq        int64      `json:"sync_seq"`
	Value          *Value     `json:"value,omitempty"`
}

// Value is a market snapshot (eBay asking prices, or PriceCharting if configured).
// Server-derived; not stored on the item row.
type Value struct {
	PCID        string `json:"pc_id"`
	ProductName string `json:"product_name"`
	ConsoleName string `json:"console_name"`
	URL         string `json:"url"`
	Source      string `json:"source,omitempty"`
	Listings    int    `json:"listings,omitempty"`
	LooseCents  *int   `json:"loose_cents"`
	CIBCents    *int   `json:"cib_cents"`
	NewCents    *int   `json:"new_cents"`
}

func NormalizeCompleteness(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case CompletenessLoose, CompletenessCIB, CompletenessNew:
		return strings.ToLower(s)
	default:
		return CompletenessUnknown
	}
}

func NormalizeRegion(s string) *string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "us", "eu", "jp", "au", "other":
		return &s
	default:
		return nil
	}
}

func CoverURL(coverID *string) *string {
	if coverID == nil || *coverID == "" {
		return nil
	}
	u := "/v1/covers/" + *coverID
	return &u
}

func TimeUTC(t time.Time) time.Time {
	return t.UTC().Truncate(time.Second)
}

func ParseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return TimeUTC(t), nil
	}
	return time.Parse(time.RFC3339Nano, s)
}

func FormatTime(t time.Time) string {
	return TimeUTC(t).Format(time.RFC3339)
}
