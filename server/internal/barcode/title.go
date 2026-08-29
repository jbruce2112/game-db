package barcode

import (
	"strings"
	"unicode"
)

var platformSuffixes = []string{
	"nintendo switch",
	"nintendo gamecube",
	"nintendo 64",
	"nintendo wii u",
	"nintendo wii",
	"nintendo ds",
	"nintendo 3ds",
	"game boy advance",
	"game boy color",
	"game boy",
	"super nintendo",
	"gamecube",
	"game cube",
	"wii u",
	"n64",
	"snes",
	"nes",
	"gba",
	"gbc",
	"3ds",
	"playstation 5",
	"playstation 4",
	"playstation 3",
	"playstation 2",
	"playstation vita",
	"playstation",
	"ps vita",
	"ps5",
	"ps4",
	"ps3",
	"ps2",
	"ps1",
	"psp",
	"xbox series x",
	"xbox series s",
	"xbox one",
	"xbox 360",
	"xbox",
	"sega genesis",
	"sega saturn",
	"mega drive",
	"dreamcast",
	"pc dvd",
	"pc cd-rom",
	"pc cd",
	"windows pc",
	"windows",
}

var platformHints = []string{
	"nintendo switch",
	"nintendo gamecube",
	"nintendo 64",
	"nintendo wii u",
	"nintendo wii",
	"game boy advance",
	"game boy color",
	"super nintendo",
	"playstation 5",
	"playstation 4",
	"playstation 3",
	"playstation 2",
	"playstation vita",
	"xbox series x",
	"xbox series s",
	"xbox one",
	"xbox 360",
	"sega genesis",
	"dreamcast",
}

var platformPretty = map[string]string{
	"nintendo switch":    "Nintendo Switch",
	"nintendo gamecube":  "Nintendo GameCube",
	"nintendo 64":        "Nintendo 64",
	"nintendo wii u":     "Wii U",
	"nintendo wii":       "Wii",
	"game boy advance":   "Game Boy Advance",
	"game boy color":     "Game Boy Color",
	"super nintendo":     "Super Nintendo",
	"playstation 5":      "PlayStation 5",
	"playstation 4":      "PlayStation 4",
	"playstation 3":      "PlayStation 3",
	"playstation 2":      "PlayStation 2",
	"playstation vita":   "PlayStation Vita",
	"xbox series x":      "Xbox Series X",
	"xbox series s":      "Xbox Series S",
	"xbox one":           "Xbox One",
	"xbox 360":           "Xbox 360",
	"sega genesis":       "Sega Genesis",
	"dreamcast":          "Dreamcast",
}

var publisherPhrases = []string{
	"square enix",
	"bandai namco",
	"electronic arts",
	"nis america",
	"limited run games",
	"warner bros",
	"rockstar games",
	"2k games",
	"atlus",
	"capcom",
	"konami",
	"nintendo",
	"sega",
	"sony",
	"activision",
	"ubisoft",
	"bethesda",
	"microsoft",
	"thq",
	"namco",
	"ea",
	"2k",
}

var retailNoise = map[string]struct{}{
	"physical": {}, "digital": {}, "new": {}, "sealed": {},
	"video": {}, "game": {}, "games": {}, "videogame": {},
	"only": {}, "used": {}, "refurbished": {},
}

var editionPhrases = []string{
	" digital deluxe edition",
	" deluxe edition",
	" definitive edition",
	" complete edition",
	" legendary edition",
	" master edition",
	" anniversary edition",
	" game of the year edition",
	" goty edition",
	" hd remaster",
	" hd collection",
	" remastered",
	" remaster",
}

// SearchQuery turns a retail product title into something IGDB can search.
func SearchQuery(product string) string {
	s := strings.TrimSpace(product)
	if s == "" {
		return ""
	}
	s = stripWrapped(s, '[', ']')
	s = stripWrapped(s, '(', ')')
	var kept []string
	for _, f := range strings.Fields(s) {
		t := strings.Trim(f, ".,;:|-–")
		if t == "" || isBarcodeToken(t) || isNoiseToken(t) {
			continue
		}
		kept = append(kept, f)
	}
	s = strings.Join(kept, " ")
	s = removeWordPhrases(s, platformSuffixes)
	s = removeWordPhrases(s, publisherPhrases)
	return strings.Join(strings.Fields(s), " ")
}

// SearchQueries is the primary cleaned title plus shorter fallbacks.
func SearchQueries(product string) []string {
	primary := SearchQuery(product)
	out := make([]string, 0, 6)
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
	add(primary)
	add(stripEditions(primary))
	words := strings.Fields(primary)
	for n := len(words) - 1; n >= 3; n-- {
		add(strings.Join(words[:n], " "))
	}
	return out
}

func stripWrapped(s string, open, close rune) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case open:
			depth++
			b.WriteByte(' ')
		case close:
			if depth > 0 {
				depth--
			}
			b.WriteByte(' ')
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func isBarcodeToken(s string) bool {
	if len(s) < 8 || len(s) > 14 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isNoiseToken(s string) bool {
	_, ok := retailNoise[strings.ToLower(s)]
	return ok
}

func removeWordPhrases(s string, phrases []string) string {
	type phrase []string
	parsed := make([]phrase, 0, len(phrases))
	for _, p := range phrases {
		w := strings.Fields(strings.ToLower(p))
		if len(w) > 0 {
			parsed = append(parsed, w)
		}
	}
	// Longest phrase first so "playstation 4" wins over "playstation".
	for i := 0; i < len(parsed); i++ {
		for j := i + 1; j < len(parsed); j++ {
			if len(parsed[j]) > len(parsed[i]) {
				parsed[i], parsed[j] = parsed[j], parsed[i]
			}
		}
	}
	words := strings.Fields(s)
	var kept []string
	for i := 0; i < len(words); {
		matched := false
		for _, p := range parsed {
			n := len(p)
			if i+n > len(words) {
				continue
			}
			ok := true
			for k := 0; k < n; k++ {
				if !strings.EqualFold(strings.Trim(words[i+k], ".,;:|-"), p[k]) {
					ok = false
					break
				}
			}
			if ok {
				i += n
				matched = true
				break
			}
		}
		if !matched {
			kept = append(kept, words[i])
			i++
		}
	}
	return strings.Join(kept, " ")
}

func stripEditions(s string) string {
	lower := strings.ToLower(s)
	for _, p := range editionPhrases {
		p = strings.TrimSpace(p)
		if p != "" && strings.HasSuffix(lower, p) {
			return strings.TrimSpace(s[:len(s)-len(p)])
		}
	}
	return s
}

func foldName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// NameScore ranks an IGDB title against the catalog query. Higher is better.
func NameScore(name, query string, platforms []string, hint string) int {
	n := foldName(name)
	q := foldName(query)
	if n == "" || q == "" {
		return 0
	}
	score := 0
	if n == q {
		score += 100
	} else if strings.HasPrefix(n, q) || strings.HasPrefix(q, n) {
		score += 70
	} else if strings.Contains(n, q) {
		score += 50
	} else if strings.Contains(q, n) {
		score += 40
	}
	ntoks := strings.Fields(n)
	qtoks := strings.Fields(q)
	set := map[string]struct{}{}
	for _, t := range ntoks {
		set[t] = struct{}{}
	}
	overlap := 0
	for _, t := range qtoks {
		if _, ok := set[t]; ok {
			overlap++
		}
	}
	score += overlap * 8
	score -= max(0, len(ntoks)-len(qtoks)) * 3
	if hint != "" {
		h := strings.ToLower(hint)
		for _, p := range platforms {
			pl := strings.ToLower(p)
			if pl == h || strings.Contains(pl, h) || strings.Contains(h, pl) {
				score += 20
				break
			}
		}
	}
	return score
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// PlatformHint is a console name pulled from the product title, if any.
func PlatformHint(product string) string {
	lower := strings.ToLower(product)
	for _, p := range platformHints {
		if strings.Contains(lower, p) {
			return p
		}
	}
	return ""
}

// PlatformDisplay is a user-facing platform name from the product title.
func PlatformDisplay(product string) string {
	h := PlatformHint(product)
	if h == "" {
		return ""
	}
	if d, ok := platformPretty[h]; ok {
		return d
	}
	parts := strings.Fields(h)
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}


