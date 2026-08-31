package tgdb

import (
	"strings"
	"unicode"

	"game-db/internal/barcode"
)

// Known TheGamesDB platform ids used when the local platform table is empty
// or a collector name does not match a TGDB name exactly.
var knownPlatformIDs = []struct {
	keys []string
	id   int64
}{
	{[]string{"nintendo switch 2", "switch 2"}, 0},
	{[]string{"nintendo switch", "switch"}, 4971},
	{[]string{"nintendo 3ds", "3ds"}, 4912},
	{[]string{"nintendo 2ds", "2ds"}, 4912},
	{[]string{"nintendo wii u", "wii u"}, 38},
	{[]string{"nintendo wii", "wii"}, 9},
	{[]string{"nintendo gamecube", "gamecube", "game cube"}, 2},
	{[]string{"nintendo 64", "n64"}, 3},
	{[]string{"super nintendo entertainment system", "super nintendo", "snes", "super famicom"}, 6},
	{[]string{"nintendo entertainment system", "famicom", "nes"}, 7},
	{[]string{"nintendo ds", "nds", "ds"}, 8},
	{[]string{"game boy advance", "gba"}, 5},
	{[]string{"game boy color", "gbc"}, 41},
	{[]string{"game boy"}, 4},
	{[]string{"playstation 5", "ps5"}, 4980},
	{[]string{"playstation 4", "ps4"}, 4919},
	{[]string{"playstation 3", "ps3"}, 12},
	{[]string{"playstation 2", "ps2"}, 11},
	{[]string{"playstation vita", "ps vita", "vita"}, 39},
	{[]string{"playstation portable", "psp"}, 13},
	{[]string{"playstation", "ps1", "psx"}, 10},
	{[]string{"xbox series x", "xbox series s", "xbox series"}, 4981},
	{[]string{"xbox one"}, 4920},
	{[]string{"xbox 360"}, 15},
	{[]string{"xbox"}, 14},
	{[]string{"sega dreamcast", "dreamcast"}, 16},
	{[]string{"sega saturn", "saturn"}, 17},
	{[]string{"genesis mega drive", "mega drive", "sega genesis", "sega mega drive genesis", "genesis"}, 18},
	{[]string{"sega cd", "mega cd"}, 21},
	{[]string{"sega 32x", "32x"}, 33},
	{[]string{"sega master system", "master system"}, 35},
	{[]string{"game gear", "sega game gear"}, 20},
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

// MatchPlatformID maps a collector name onto a TheGamesDB platform id.
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
		ak := foldPlat(p.Alias)
		score := 0
		if pk == key || ak == key {
			score = 100
		} else {
			for _, row := range knownPlatformIDs {
				for _, k := range row.keys {
					if key != k {
						continue
					}
					if pk == k || ak == k || strings.HasSuffix(pk, " "+k) || strings.Contains(pk, " "+k+" ") {
						score = 80
					}
					if pk == k || ak == k {
						score = 95
					}
				}
			}
		}
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

// PickGame chooses a TGDB search hit for a library title. Exact name wins;
// otherwise a high NameScore. Nil if nothing is close.
func PickGame(title string, games []Game) *Game {
	if len(games) == 0 || strings.TrimSpace(title) == "" {
		return nil
	}
	var exact []Game
	for i := range games {
		if namesMatch(games[i].Title, title) {
			exact = append(exact, games[i])
		}
	}
	pool := games
	if len(exact) > 0 {
		pool = exact
	}
	best := pool[0]
	bestScore := scoreGame(best, title)
	for _, g := range pool[1:] {
		s := scoreGame(g, title)
		if s > bestScore {
			best = g
			bestScore = s
		}
	}
	if len(exact) == 0 && bestScore < 55 {
		return nil
	}
	out := best
	return &out
}

func namesMatch(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) ||
		foldPlat(a) == foldPlat(b) ||
		barcode.CompactName(a) == barcode.CompactName(b)
}

func scoreGame(g Game, title string) int {
	s := barcode.NameScore(g.Title, title, nil, "")
	if g.FrontURL == "" {
		s -= 30
	}
	n := strings.ToLower(g.Title)
	for _, extra := range []string{"(vc)", "virtual console", "not for resale", "switch online"} {
		if strings.Contains(n, extra) {
			s -= 15
		}
	}
	return s
}
