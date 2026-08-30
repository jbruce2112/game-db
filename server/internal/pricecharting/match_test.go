package pricecharting

import "testing"

func TestConsoleNameRegion(t *testing.T) {
	us := ConsoleName("Nintendo GameCube", "us")
	if len(us) == 0 || us[0] != "Gamecube" {
		t.Fatalf("us %v", us)
	}
	eu := ConsoleName("GameCube", "eu")
	if len(eu) == 0 || eu[0] != "PAL Gamecube" {
		t.Fatalf("eu %v", eu)
	}
	jp := ConsoleName("PlayStation 5", "jp")
	if len(jp) == 0 || jp[0] != "JP Playstation 5" {
		t.Fatalf("jp %v", jp)
	}
	nesJP := ConsoleName("NES", "jp")
	if len(nesJP) == 0 || nesJP[0] != "Famicom" {
		t.Fatalf("famicom %v", nesJP)
	}
}

func TestPickBestPrefersRegionConsole(t *testing.T) {
	products := []Product{
		{ID: "1", Name: "Super Mario Sunshine", Console: "PAL Gamecube"},
		{ID: "2", Name: "Super Mario Sunshine", Console: "Gamecube"},
		{ID: "3", Name: "Super Mario Sunshine", Console: "JP Gamecube"},
		{ID: "4", Name: "Super Mario Sunshine [Player's Choice]", Console: "Gamecube"},
	}
	got, score := PickBest(products, "Super Mario Sunshine", "Nintendo GameCube", "us")
	if got.ID != "2" {
		t.Fatalf("got %+v score %d", got, score)
	}
	jp, _ := PickBest(products, "Super Mario Sunshine", "Nintendo GameCube", "jp")
	if jp.ID != "3" {
		t.Fatalf("jp %+v", jp)
	}
}

func TestSlug(t *testing.T) {
	if got := slug("Playstation 5"); got != "playstation-5" {
		t.Fatalf("%s", got)
	}
	if got := slug("Tetris Effect: Connected"); got != "tetris-effect-connected" {
		t.Fatalf("%s", got)
	}
}
