package export

import (
	"strings"
	"testing"
	"time"

	"game-db/internal/model"
)

func TestParseRoundTrip(t *testing.T) {
	region := "us"
	igdb := int64(1075)
	src := []model.Item{{
		ID:           "c8b62233-31b7-46a2-94d1-5cd4460437f7",
		Title:        `Super Mario, "Sunshine"`,
		Platform:     "Nintendo GameCube",
		Region:       &region,
		Completeness: "cib",
		Notes:        "amazing",
		IGDBGameID:   &igdb,
		CreatedAt:    time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 8, 24, 1, 0, 0, 0, time.UTC),
	}}
	raw, err := LibraryCSV(src)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseLibraryCSV(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len %d", len(got))
	}
	if got[0].Title != src[0].Title || got[0].Platform != src[0].Platform {
		t.Fatalf("%+v", got[0])
	}
	if got[0].ID != src[0].ID {
		t.Fatalf("id %s", got[0].ID)
	}
	if got[0].IGDBGameID == nil || *got[0].IGDBGameID != 1075 {
		t.Fatalf("igdb %+v", got[0].IGDBGameID)
	}
	if got[0].Barcode != nil {
		t.Fatalf("barcode %+v", got[0].Barcode)
	}
}

func TestParseBarcodeColumn(t *testing.T) {
	code := "045496590376"
	src := []model.Item{{
		ID:           "c8b62233-31b7-46a2-94d1-5cd4460437f7",
		Title:        "Sunshine",
		Platform:     "Nintendo GameCube",
		Completeness: "unknown",
		Barcode:      &code,
		CreatedAt:    time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 8, 24, 1, 0, 0, 0, time.UTC),
	}}
	raw, err := LibraryCSV(src)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseLibraryCSV(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Barcode == nil || *got[0].Barcode != code {
		t.Fatalf("barcode %+v", got[0].Barcode)
	}
}

func TestParseCLZExport(t *testing.T) {
	raw := []byte(`Platform,Title,"Release Date",Publisher,Developer,Genre,"Added Date",Barcode,Region
"PlayStation 4","Shin Megami Tensei III Nocturne HD Remaster","May 25, 2021",Atlus,Atlus,RPG,"Aug 29, 2026",730865220366,USA
"PlayStation 5","Tetris Effect: Connected","Nov 17, 2023","Limited Run Games","Enhance Games",Puzzle,"Aug 30, 2026",4570101050335,Japan
`)
	got, err := ParseLibraryCSV(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len %d", len(got))
	}
	if got[0].Title != "Shin Megami Tensei III Nocturne HD Remaster" || got[0].Platform != "PlayStation 4" {
		t.Fatalf("%+v", got[0])
	}
	if got[0].Region == nil || *got[0].Region != "us" {
		t.Fatalf("region %+v", got[0].Region)
	}
	if got[0].Barcode == nil || *got[0].Barcode != "730865220366" {
		t.Fatalf("barcode %+v", got[0].Barcode)
	}
	if got[1].Region == nil || *got[1].Region != "jp" {
		t.Fatalf("jp region %+v", got[1].Region)
	}
	if got[0].CreatedAt.IsZero() || got[0].CreatedAt.Year() != 2026 || got[0].CreatedAt.Month() != time.August || got[0].CreatedAt.Day() != 29 {
		t.Fatalf("added date %v", got[0].CreatedAt)
	}
	if got[1].CreatedAt.Month() != time.August || got[1].CreatedAt.Day() != 30 {
		t.Fatalf("tetris added date %v", got[1].CreatedAt)
	}
}

func TestParseRequiresTitleAndPlatform(t *testing.T) {
	_, err := ParseLibraryCSV([]byte("title,notes\nOnlyTitle,x\n"))
	if err == nil || !strings.Contains(err.Error(), "platform") {
		t.Fatalf("err %v", err)
	}
}

func TestParseSkipsBlankRows(t *testing.T) {
	raw := []byte("title,platform\nZelda,N64\n,\n")
	got, err := ParseLibraryCSV(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len %d", len(got))
	}
}
