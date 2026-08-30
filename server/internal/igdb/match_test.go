package igdb

import "testing"

func TestMatchPlatformID(t *testing.T) {
	ps4 := "ps4"
	plats := []Platform{
		{ID: 7, Name: "PlayStation"},
		{ID: 48, Name: "PlayStation 4", Abbreviation: &ps4},
		{ID: 167, Name: "PlayStation 5"},
		{ID: 130, Name: "Nintendo Switch"},
		{ID: 508, Name: "Nintendo Switch 2"},
		{ID: 29, Name: "Sega Mega Drive/Genesis"},
		{ID: 33, Name: "Game Boy"},
		{ID: 24, Name: "Game Boy Advance"},
	}
	cases := []struct {
		in   string
		want int64
	}{
		{"PlayStation 4", 48},
		{"PlayStation", 7},
		{"PS5", 167},
		{"Nintendo Switch", 130},
		{"Nintendo Switch 2", 508},
		{"Genesis / Mega Drive", 29},
		{"Game Boy", 33},
		{"Game Boy Advance", 24},
	}
	for _, tc := range cases {
		got := MatchPlatformID(tc.in, plats)
		if got == nil || *got != tc.want {
			t.Errorf("%q: got %v want %d", tc.in, got, tc.want)
		}
	}
}

func TestPickGameExactNamePrefersPlatform(t *testing.T) {
	games := []Game{
		{ID: 1, Name: "God of War", Platforms: []Platform{{ID: 9, Name: "PlayStation 3"}}},
		{ID: 2, Name: "God of War", Platforms: []Platform{{ID: 48, Name: "PlayStation 4"}}},
		{ID: 3, Name: "God of War Saga", Platforms: []Platform{{ID: 9, Name: "PlayStation 3"}}},
	}
	got := PickGame("God of War", 48, games)
	if got == nil || got.ID != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestPickGameRejectsWeakMatch(t *testing.T) {
	games := []Game{
		{ID: 9, Name: "Unrelated Fighter", Platforms: []Platform{{ID: 48}}},
	}
	if PickGame("Metal Gear Solid V: The Phantom Pain", 48, games) != nil {
		t.Fatal("expected no match")
	}
}
