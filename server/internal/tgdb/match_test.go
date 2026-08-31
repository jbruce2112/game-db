package tgdb

import "testing"

func TestMatchPlatformIDKnownFallbacks(t *testing.T) {
	cases := []struct {
		name string
		id   int64
	}{
		{"Nintendo Switch", 4971},
		{"Switch", 4971},
		{"PlayStation 4", 4919},
		{"PS2", 11},
		{"Nintendo GameCube", 2},
		{"GameCube", 2},
		{"Nintendo 64", 3},
		{"NES", 7},
		{"Super Nintendo", 6},
		{"Game Boy Advance", 5},
		{"Game Boy Color", 41},
		{"Game Boy", 4},
		{"Xbox 360", 15},
		{"Xbox", 14},
		{"Genesis / Mega Drive", 18},
		{"Sega Genesis", 18},
		{"Game Gear", 20},
	}
	for _, c := range cases {
		got := MatchPlatformID(c.name, nil)
		if got == nil || *got != c.id {
			t.Errorf("%q: got %v want %d", c.name, got, c.id)
		}
	}
}

func TestMatchPlatformIDUsesLiveNames(t *testing.T) {
	plats := []Platform{
		{ID: 7, Name: "Nintendo Entertainment System (NES)", Alias: "NES"},
		{ID: 4971, Name: "Nintendo Switch", Alias: "Switch"},
		{ID: 10, Name: "Sony Playstation", Alias: "PS1"},
	}
	if id := MatchPlatformID("NES", plats); id == nil || *id != 7 {
		t.Fatalf("NES via alias: %v", id)
	}
	if id := MatchPlatformID("Nintendo Switch", plats); id == nil || *id != 4971 {
		t.Fatalf("Switch: %v", id)
	}
	if id := MatchPlatformID("PlayStation", plats); id == nil || *id != 10 {
		t.Fatalf("PS1: %v", id)
	}
}

func TestMatchPlatformIDDoesNotConfuseHandhelds(t *testing.T) {
	if id := MatchPlatformID("Game Boy Color", nil); id == nil || *id != 41 {
		t.Fatalf("GBC %v", id)
	}
	if id := MatchPlatformID("Game Boy", nil); id == nil || *id != 4 {
		t.Fatalf("GB %v", id)
	}
}

func TestPickGamePrefersExactRetailFront(t *testing.T) {
	games := []Game{
		{ID: 1, Title: "Mario Kart 64 (VC)", FrontURL: "http://x/vc.jpg"},
		{ID: 2, Title: "Mario Kart 64", FrontURL: "http://x/front.jpg"},
		{ID: 3, Title: "Mario Kart 64 [Not for Resale]", FrontURL: "http://x/nfr.jpg"},
	}
	got := PickGame("Mario Kart 64", games)
	if got == nil || got.ID != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestPickGameRequiresCloseName(t *testing.T) {
	games := []Game{
		{ID: 9, Title: "Unrelated Adventure", FrontURL: "http://x/a.jpg"},
	}
	if PickGame("Super Mario Sunshine", games) != nil {
		t.Fatal("expected miss")
	}
}
