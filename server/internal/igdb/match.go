package igdb

import (
	"strings"
	"unicode"

	"game-db/internal/barcode"
)

// Known console ids from IGDB. Used when the local platform table is empty
// or a CLZ name does not match an IGDB name exactly.
var knownPlatformIDs = []struct {
	keys []string
	id   int64
}{
	{[]string{"nintendo switch 2", "switch 2"}, 0},
	{[]string{"nintendo switch", "switch"}, 130},
	{[]string{"playstation 5", "ps5"}, 167},
	{[]string{"playstation 4", "ps4"}, 48},
	{[]string{"playstation 3", "ps3"}, 9},
	{[]string{"playstation 2", "ps2"}, 8},
	{[]string{"playstation vita", "ps vita", "vita"}, 46},
	{[]string{"playstation", "ps1", "psx"}, 7},
	{[]string{"nintendo 64", "n64"}, 4},
	{[]string{"game boy advance", "gba"}, 24},
	{[]string{"game boy color", "gbc"}, 22},
	{[]string{"game boy"}, 33},
	{[]string{"game gear", "sega game gear"}, 35},
	{[]string{"genesis mega drive", "mega drive", "sega genesis", "sega mega drive genesis", "genesis"}, 29},
}

func foldPlat(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// MatchPlatformID maps a collector name (CLZ "PlayStation 4", "Genesis / Mega Drive")
// onto an IGDB platform id.
func MatchPlatformID(name string, plats []Platform) *int64 {
	key := foldPlat(name)
	if key == "" {
		return nil
	}
	if id := matchPlatformInList(key, plats); id != nil {
		return id
	}
	for _, row := range knownPlatformIDs {
		for _, k := range row.keys {
			if key == k && row.id > 0 {
				id := row.id
				return &id
			}
		}
	}
	return nil
}

func matchPlatformInList(key string, plats []Platform) *int64 {
	type scored struct {
		id    int64
		score int
	}
	var best scored
	ties := 0
	for _, p := range plats {
		pk := foldPlat(p.Name)
		score := 0
		if pk == key {
			score = 100
		} else if p.Abbreviation != nil && foldPlat(*p.Abbreviation) == key {
			score = 90
		} else if p.Slug != nil && foldPlat(strings.ReplaceAll(*p.Slug, "-", " ")) == key {
			score = 88
		} else {
			for _, row := range knownPlatformIDs {
				for _, k := range row.keys {
					if key != k {
						continue
					}
					if pk == k || strings.HasSuffix(pk, " "+k) || strings.Contains(pk, " "+k+" ") {
						if len(k) >= len(key) || pk == foldPlat(p.Name) {
							score = 80
						}
					}
					if pk == k {
						score = 95
					}
				}
			}
		}
		// Prefer exact "playstation" over starting-with "playstation 4".
		if score == 0 && (strings.HasPrefix(pk, key+" ") || strings.HasPrefix(key, pk+" ")) {
			if len(pk) == len(key) {
				score = 70
			}
		}
		if score > best.score {
			best = scored{id: p.ID, score: score}
			ties = 1
		} else if score == best.score && score > 0 && p.ID != best.id {
			ties++
		}
	}
	if best.score >= 80 && ties == 1 {
		id := best.id
		return &id
	}
	return nil
}

// PickGame chooses an IGDB search hit for a library title. Exact name wins;
// otherwise a high NameScore with the right platform. Nil if nothing is close.
func PickGame(title string, platformID int64, games []Game) *Game {
	if len(games) == 0 || strings.TrimSpace(title) == "" {
		return nil
	}
	norm := barcode.SearchQuery(title)
	if norm == "" {
		norm = title
	}

	onPlatform := func(g Game) bool {
		if platformID <= 0 {
			return true
		}
		for _, p := range g.Platforms {
			if p.ID == platformID {
				return true
			}
		}
		return false
	}

	var exact []Game
	for i := range games {
		if strings.EqualFold(strings.TrimSpace(games[i].Name), strings.TrimSpace(title)) ||
			foldPlat(games[i].Name) == foldPlat(title) {
			exact = append(exact, games[i])
		}
	}
	pool := games
	if len(exact) > 0 {
		pool = exact
	}
	var filtered []Game
	for _, g := range pool {
		if onPlatform(g) {
			filtered = append(filtered, g)
		}
	}
	if len(filtered) > 0 {
		pool = filtered
	} else if len(exact) == 0 {
		return nil
	}

	best := pool[0]
	bestScore := barcode.NameScore(best.Name, norm, platformNames(best), "")
	for _, g := range pool[1:] {
		s := barcode.NameScore(g.Name, norm, platformNames(g), "")
		if s > bestScore {
			best = g
			bestScore = s
		}
	}
	if len(exact) == 0 && bestScore < 70 {
		return nil
	}
	out := best
	return &out
}

func platformNames(g Game) []string {
	out := make([]string, 0, len(g.Platforms))
	for _, p := range g.Platforms {
		out = append(out, p.Name)
	}
	return out
}
