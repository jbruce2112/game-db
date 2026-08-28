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
