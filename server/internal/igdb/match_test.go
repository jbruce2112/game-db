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

func TestPickGameDoublePackUsesBundle(t *testing.T) {
	ps4 := []Platform{{ID: 48, Name: "PlayStation 4"}}
	games := []Game{
		{ID: 7342, Name: "Inside", Platforms: ps4},
		{ID: 165866, Name: "Inside & Limbo Bundle", Platforms: ps4},
		{ID: 1331, Name: "Limbo", Platforms: ps4},
	}
	got := PickGame("Inside - Limbo Double Pack", 48, games)
	if got == nil || got.ID != 165866 {
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

func TestPickGameCLZMismatches(t *testing.T) {
	gg := []Platform{{ID: 35, Name: "Sega Game Gear"}}
	ps4 := []Platform{{ID: 48, Name: "PlayStation 4"}}
	ps3 := []Platform{{ID: 9, Name: "PlayStation 3"}}
	ps5 := []Platform{{ID: 167, Name: "PlayStation 5"}}
	gb := []Platform{{ID: 33, Name: "Game Boy"}}
	nsw := []Platform{{ID: 130, Name: "Nintendo Switch"}}
	gen := []Platform{{ID: 29, Name: "Sega Mega Drive/Genesis"}}

	cases := []struct {
		title string
		pid   int64
		games []Game
		want  int64
	}{
		{
			"Crash Bandicoot N.Sane Trilogy", 48,
			[]Game{{ID: 26196, Name: "Crash Bandicoot N. Sane Trilogy", Platforms: ps4}},
			26196,
		},
		{
			"Deadly Premonitions: The Director's Cut", 9,
			[]Game{{ID: 9245, Name: "Deadly Premonition: Director's Cut", Platforms: ps3}},
			9245,
		},
		{
			"Metal Gear Solid: Master Collection Vol. 1", 167,
			[]Game{{ID: 250635, Name: "Metal Gear Solid Master Collection: Volume 1", Platforms: ps5}},
			250635,
		},
		{
			"Sonic Triple Trouble", 35,
			[]Game{{ID: 19556, Name: "Sonic the Hedgehog: Triple Trouble", Platforms: gg}},
			19556,
		},
		{
			"WWF Warzone", 33,
			[]Game{{ID: 206032, Name: "WWF War Zone", Platforms: gb}},
			206032,
		},
		{
			"New Pokemon Snap", 130,
			[]Game{{ID: 135142, Name: "New Pokémon Snap", Platforms: nsw}},
			135142,
		},
		{
			"Sonic Colours: Ultimate", 48,
			[]Game{{ID: 150005, Name: "Sonic Colors: Ultimate", Platforms: ps4}},
			150005,
		},
		{
			"Odin Sphere Leifthraiser", 46,
			[]Game{{ID: 15709, Name: "Odin Sphere: Leifthrasir", Platforms: []Platform{{ID: 46, Name: "PlayStation Vita"}}}},
			15709,
		},
		{
			"Battletech", 29,
			[]Game{
				{ID: 19186, Name: "MechWarrior 3050", Platforms: gen},
				{ID: 888, Name: "BattleTech: A Game of Armored Combat", Platforms: gen},
			},
			888,
		},
	}
	for _, tc := range cases {
		got := PickGame(tc.title, tc.pid, tc.games)
		if got == nil || got.ID != tc.want {
			t.Errorf("%q: got %+v want %d", tc.title, got, tc.want)
		}
	}
}

func TestSearchTitlesRomanNumerals(t *testing.T) {
	qs := SearchTitles("Shin Megami Tensei 5")
	found := false
	for _, q := range qs {
		if q == "Shin Megami Tensei V" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%v", qs)
	}
	games := []Game{{
		ID: 26775, Name: "Shin Megami Tensei V",
		Platforms: []Platform{{ID: 130, Name: "Nintendo Switch"}},
	}}
	got := PickGame("Shin Megami Tensei 5", 130, games)
	if got == nil || got.ID != 26775 {
		t.Fatalf("got %+v", got)
	}
}

func TestSearchTitlesDropsLeadingFranchise(t *testing.T) {
	qs := SearchTitles("Spintires: MudRunner - American Wilds")
	found := false
	for _, q := range qs {
		if q == "MudRunner American Wilds" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%v", qs)
	}
	ps4 := []Platform{{ID: 48, Name: "PlayStation 4"}}
	got := PickGame("Spintires: MudRunner - American Wilds", 48, []Game{
		{ID: 54789, Name: "MudRunner", Platforms: ps4},
		{ID: 119105, Name: "MudRunner: American Wilds Edition", Platforms: ps4},
	})
	if got == nil || got.ID != 119105 {
		t.Fatalf("got %+v", got)
	}
}

func TestSearchTitlesSoftensPunctuation(t *testing.T) {
	qs := SearchTitles("Crash Bandicoot N.Sane Trilogy")
	found := false
	for _, q := range qs {
		if q == "Crash Bandicoot N Sane Trilogy" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%v", qs)
	}
}

func TestSearchTitlesFEAR(t *testing.T) {
	qs := SearchTitles("F.E.A.R.: First Encounter Assault Recon")
	t.Log(qs)
	want := map[string]bool{"FEAR First Encounter Assault Recon": false, "FEAR": false, "F.E.A.R.": false}
	for _, q := range qs {
		if _, ok := want[q]; ok {
			want[q] = true
		}
	}
	for q, ok := range want {
		if !ok {
			t.Errorf("missing %q in %v", q, qs)
		}
	}
}

func TestPickGameFEARNotFearEffect(t *testing.T) {
	ps3 := []Platform{{ID: 9, Name: "PlayStation 3"}}
	games := []Game{
		{ID: 8600, Name: "Fear Effect", Platforms: ps3},
		{ID: 202149, Name: "F.E.A.R.", Platforms: ps3},
		{ID: 514, Name: "F.E.A.R. 3", Platforms: ps3},
		{ID: 520, Name: "F.E.A.R. 2: Project Origin", Platforms: ps3},
	}
	got := PickGame("F.E.A.R.: First Encounter Assault Recon", 9, games)
	if got == nil || got.ID != 202149 {
		t.Fatalf("got %+v", got)
	}
}
