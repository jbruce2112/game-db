package export

import (
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"game-db/internal/model"
)

func ParseLibraryCSV(data []byte) ([]model.Item, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("csv is empty")
	}
	idx := map[string]int{}
	for i, h := range records[0] {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := idx["title"]; !ok {
		return nil, fmt.Errorf("missing title column")
	}
	if _, ok := idx["platform"]; !ok {
		return nil, fmt.Errorf("missing platform column")
	}

	now := model.TimeUTC(time.Now())
	seen := map[string]struct{}{}
	items := make([]model.Item, 0, len(records)-1)
	for n, rec := range records[1:] {
		row := n + 2
		get := func(key string) string {
			i, ok := idx[key]
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		title := get("title")
		platform := get("platform")
		if title == "" && platform == "" && get("id") == "" {
			continue
		}
		if title == "" || platform == "" {
			return nil, fmt.Errorf("row %d: title and platform are required", row)
		}
		id := strings.ToLower(get("id"))
		if !validUUID(id) {
			id = newUUID()
		}
		if _, dup := seen[id]; dup {
			return nil, fmt.Errorf("row %d: duplicate id %s", row, id)
		}
		seen[id] = struct{}{}

		created := now
		if t, err := model.ParseTime(get("created_at")); err == nil && !t.IsZero() {
			created = t
		}
		updated := created
		if t, err := model.ParseTime(get("updated_at")); err == nil && !t.IsZero() {
			updated = t
		}
		item := model.Item{
			ID:             id,
			Title:          title,
			Platform:       platform,
			Region:         model.NormalizeRegion(get("region")),
			Completeness:   model.NormalizeCompleteness(get("completeness")),
			Notes:          get("notes"),
			IGDBGameID:     parseInt64(get("igdb_game_id")),
			IGDBPlatformID: parseInt64(get("igdb_platform_id")),
			Barcode:        parseBarcode(get("barcode")),
			CreatedAt:      created,
			UpdatedAt:      updated,
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("csv has no game rows")
	}
	return items, nil
}

func parseBarcode(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return nil
	}
	return &out
}

func parseInt64(s string) *int64 {
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

func validUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				return false
			}
		}
	}
	return true
}

func newUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hx := fmt.Sprintf("%x", b)
	return hx[0:8] + "-" + hx[8:12] + "-" + hx[12:16] + "-" + hx[16:20] + "-" + hx[20:32]
}
