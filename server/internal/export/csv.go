package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"time"

	"game-db/internal/model"
)

var csvHeader = []string{
	"id",
	"title",
	"platform",
	"region",
	"completeness",
	"notes",
	"igdb_game_id",
	"igdb_platform_id",
	"barcode",
	"created_at",
	"updated_at",
}

func LibraryCSV(items []model.Item) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\uFEFF")
	w := csv.NewWriter(&buf)
	if err := w.Write(csvHeader); err != nil {
		return nil, err
	}
	for _, it := range items {
		if err := w.Write([]string{
			it.ID,
			it.Title,
			it.Platform,
			deref(it.Region),
			it.Completeness,
			it.Notes,
			fmtInt(it.IGDBGameID),
			fmtInt(it.IGDBPlatformID),
			deref(it.Barcode),
			model.FormatTime(it.CreatedAt),
			model.FormatTime(it.UpdatedAt),
		}); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Filename(now time.Time) string {
	return fmt.Sprintf("game-db-%s.csv", now.UTC().Format("2006-01-02"))
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func fmtInt(n *int64) string {
	if n == nil {
		return ""
	}
	return fmt.Sprintf("%d", *n)
}
