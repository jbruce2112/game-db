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
			foldPlat(games[i].Name) == foldPlat(title) ||
			barcode.CompactName(games[i].Name) == barcode.CompactName(title) {
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
	bestScore := barcode.NameScore(best.Name, title, platformNames(best), "")
	bestCover := coverage(title, best.Name)
	for _, g := range pool[1:] {
		s := barcode.NameScore(g.Name, title, platformNames(g), "")
		c := coverage(title, g.Name)
		if s > bestScore || (s == bestScore && c > bestCover) {
			best = g
			bestScore = s
			bestCover = c
		}
	}
	if len(exact) == 0 && bestScore < 55 && bestCover < 0.8 {
		return nil
	}
	out := best
	return &out
}

func coverage(title, name string) float64 {
	c := barcode.TokenCoverage(title, name)
	words := strings.Fields(softenPunct(title))
	if len(words) >= 3 {
		shorter := strings.Join(words[:len(words)-1], " ")
		if alt := barcode.TokenCoverage(shorter, name); alt > c {
			c = alt
		}
	}
	return c
}

func softenPunct(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func romanSwap(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return s
	}
	i := len(fields) - 1
	switch strings.ToLower(fields[i]) {
	case "1":
		fields[i] = "I"
	case "2":
		fields[i] = "II"
	case "3":
		fields[i] = "III"
	case "4":
		fields[i] = "IV"
	case "5":
		fields[i] = "V"
	case "6":
		fields[i] = "VI"
	case "i":
		fields[i] = "1"
	case "ii":
		fields[i] = "2"
	case "iii":
		fields[i] = "3"
	case "iv":
		fields[i] = "4"
	case "v":
		fields[i] = "5"
	case "vi":
		fields[i] = "6"
	default:
		return s
	}
	return strings.Join(fields, " ")
}

func splitCompound(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return s
	}
	last := fields[len(fields)-1]
	low := strings.ToLower(last)
	for _, suf := range []string{"zone", "land", "world", "mania"} {
		if len(low) > len(suf)+2 && strings.HasSuffix(low, suf) {
			head := last[:len(last)-len(suf)]
			fields[len(fields)-1] = head
			fields = append(fields, last[len(last)-len(suf):])
			return strings.Join(fields, " ")
		}
	}
	return s
}

func applySpelling(s string) string {
	fields := strings.Fields(s)
	for i, f := range fields {
		low := strings.ToLower(f)
		switch low {
		case "colours":
			fields[i] = "Colors"
		case "colour":
			fields[i] = "Color"
		case "grey":
			fields[i] = "Gray"
		}
	}
	return strings.Join(fields, " ")
}

func collapseInitials(s string) string {
	fields := strings.Fields(s)
	var out []string
	for i := 0; i < len(fields); {
		if len([]rune(fields[i])) == 1 {
			j := i
			var run strings.Builder
			for j < len(fields) && len([]rune(fields[j])) == 1 {
				run.WriteString(fields[j])
				j++
			}
			if j-i >= 2 {
				out = append(out, run.String())
				i = j
				continue
			}
		}
		out = append(out, fields[i])
		i++
	}
	return strings.Join(out, " ")
}

// SearchTitles are IGDB search strings derived from a collector title
// (punctuation, spelling, shorter prefixes).
func SearchTitles(title string) []string {
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		for _, e := range out {
			if strings.EqualFold(e, s) {
				return
			}
		}
		out = append(out, s)
	}
	add(title)
	soft := softenPunct(title)
	add(soft)
	add(romanSwap(soft))
	add(splitCompound(soft))
	add(applySpelling(soft))
	collapsed := collapseInitials(soft)
	add(collapsed)
	if dotted, compact := leadingAcronym(soft); compact != "" {
		add(dotted)
		add(compact)
		rest := strings.TrimSpace(strings.TrimPrefix(collapsed, compact))
		if rest != "" {
			add(compact + " " + rest)
		}
	}
	for _, q := range barcode.SearchQueries(soft) {
		add(q)
	}
	for _, base := range []string{soft, collapsed} {
		words := strings.Fields(base)
		for n := len(words) - 1; n >= 2; n-- {
			add(strings.Join(words[:n], " "))
		}
		for n := 1; n <= len(words)-2; n++ {
			add(strings.Join(words[n:], " "))
		}
	}
	return out
}

// LeadingAcronym pulls a dotted form (F.E.A.R.) and compact form (FEAR)
// from a collector title that starts with initials.
func LeadingAcronym(title string) (dotted, compact string) {
	return leadingAcronym(softenPunct(title))
}

func leadingAcronym(soft string) (dotted, compact string) {
	fields := strings.Fields(soft)
	n := 0
	for n < len(fields) && len([]rune(fields[n])) == 1 {
		n++
	}
	if n < 2 {
		return "", ""
	}
	var d, c strings.Builder
	for _, f := range fields[:n] {
		u := strings.ToUpper(f)
		c.WriteString(u)
		d.WriteString(u)
		d.WriteByte('.')
	}
	return d.String(), c.String()
}

func platformNames(g Game) []string {
	out := make([]string, 0, len(g.Platforms))
	for _, p := range g.Platforms {
		out = append(out, p.Name)
	}
	return out
}
