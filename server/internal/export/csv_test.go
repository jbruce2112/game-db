package export

import (
	"strings"
	"testing"
	"time"

	"game-db/internal/model"
)

func TestLibraryCSVEscapesQuotesAndCommas(t *testing.T) {
	notes := `hello, "world"`
	region := "us"
	igdb := int64(1029)
	items := []model.Item{{
		ID:           "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		Title:        "Zelda, Ocarina",
		Platform:     "Nintendo 64",
		Region:       &region,
		Completeness: "cib",
		Notes:        notes,
		IGDBGameID:   &igdb,
		CreatedAt:    time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
	}}
	raw, err := LibraryCSV(items)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.HasPrefix(s, "\uFEFF") {
		t.Fatal("expected UTF-8 BOM for Excel")
	}
	if !strings.Contains(s, `"Zelda, Ocarina"`) {
		t.Fatalf("title not quoted: %s", s)
	}
	if !strings.Contains(s, `"hello, ""world"""`) {
		t.Fatalf("notes not escaped: %s", s)
	}
	if !strings.Contains(s, "1029") {
		t.Fatalf("missing igdb id: %s", s)
	}
}

func TestFilename(t *testing.T) {
	got := Filename(time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC))
	if got != "game-db-2026-08-28.csv" {
		t.Fatal(got)
	}
}
