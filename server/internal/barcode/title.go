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
	"nintendo switch":   "Nintendo Switch",
	"nintendo gamecube": "Nintendo GameCube",
	"nintendo 64":       "Nintendo 64",
	"nintendo wii u":    "Wii U",
	"nintendo wii":      "Wii",
	"game boy advance":  "Game Boy Advance",
	"game boy color":    "Game Boy Color",
	"super nintendo":    "Super Nintendo",
	"playstation 5":     "PlayStation 5",
	"playstation 4":     "PlayStation 4",
	"playstation 3":     "PlayStation 3",
	"playstation 2":     "PlayStation 2",
	"playstation vita":  "PlayStation Vita",
	"xbox series x":     "Xbox Series X",
	"xbox series s":     "Xbox Series S",
	"xbox one":          "Xbox One",
	"xbox 360":          "Xbox 360",
	"sega genesis":      "Sega Genesis",
	"dreamcast":         "Dreamcast",
}

var publisherPhrases = []string{
	"square enix",
	"bandai namco",
	"electronic arts",
	"nis america",
	"limited run games",
	"super deluxe games",
	"superdeluxegames",
	"enhance games",
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
	s = removeWordPhrases(s, platformSuffixes)
	s = removeWordPhrases(s, publisherPhrases)
	var kept []string
	for _, f := range strings.Fields(s) {
		t := strings.Trim(f, ".,;:|-–")
		if t == "" || isBarcodeToken(t) || isNoiseToken(t) {
			continue
		}
		kept = append(kept, f)
	}
	return trimTrailingPrep(strings.Join(kept, " "))
}

func trimTrailingPrep(s string) string {
	for {
		lower := strings.ToLower(s)
		trimmed := false
		for _, suf := range []string{" for", " on"} {
			if strings.HasSuffix(lower, suf) {
				s = strings.TrimSpace(s[:len(s)-len(suf)])
				trimmed = true
				break
			}
		}
		if !trimmed {
			return s
		}
	}
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

func stripDiacritics(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case 'é', 'è', 'ê', 'ë', 'É', 'È', 'Ê', 'Ë':
			b.WriteByte('e')
		case 'á', 'à', 'ä', 'â', 'å', 'Á', 'À', 'Ä', 'Â':
			b.WriteByte('a')
		case 'í', 'ì', 'ï', 'î', 'Í', 'Ì':
			b.WriteByte('i')
		case 'ó', 'ò', 'ö', 'ô', 'Ó', 'Ö':
			b.WriteByte('o')
		case 'ú', 'ù', 'ü', 'û', 'Ú', 'Ü':
			b.WriteByte('u')
		case 'ñ', 'Ñ':
			b.WriteByte('n')
		case 'ç', 'Ç':
			b.WriteByte('c')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func foldName(s string) string {
	s = stripDiacritics(s)
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

func CompactName(s string) string {
	return strings.ReplaceAll(foldName(s), " ", "")
}

func canonToken(t string) string {
	switch t {
	case "vol":
		return "volume"
	case "colours", "colors", "colour":
		return "color"
	case "grey":
		return "gray"
	case "i":
		return "1"
	case "ii":
		return "2"
	case "iii":
		return "3"
	case "iv":
		return "4"
	case "v":
		return "5"
	case "vi":
		return "6"
	}
	if len(t) > 4 && strings.HasSuffix(t, "s") && !strings.HasSuffix(t, "ss") {
		return t[:len(t)-1]
	}
	return t
}

func tokenPrefix(a, b string) bool {
	at := strings.Fields(a)
	bt := strings.Fields(b)
	if len(at) == 0 || len(bt) == 0 {
		return false
	}
	short, long := at, bt
	if len(at) > len(bt) {
		short, long = bt, at
	}
	if len(short) < 2 {
		return false
	}
	for i, t := range short {
		if canonToken(t) != canonToken(long[i]) {
			return false
		}
	}
	return true
}

func tokenSet(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, t := range strings.Fields(foldName(s)) {
		out[canonToken(t)] = struct{}{}
	}
	return out
}

var stopTokens = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "of": {}, "and": {},
	"pack": {}, "bundle": {}, "collection": {}, "compilation": {},
	"double": {}, "combo": {},
}

// TokenCoverage is how many non-stop query tokens appear in name (0–1).
func TokenCoverage(query, name string) float64 {
	qset := tokenSet(query)
	nset := tokenSet(name)
	var need, hit int
	for t := range qset {
		if _, stop := stopTokens[t]; stop {
			continue
		}
		need++
		if _, ok := nset[t]; ok {
			hit++
		}
	}
	if need == 0 {
		return 0
	}
	return float64(hit) / float64(need)
}

// NameScore ranks an IGDB title against the catalog query. Higher is better.
func NameScore(name, query string, platforms []string, hint string) int {
	n := foldName(name)
	q := foldName(query)
	if n == "" || q == "" {
		return 0
	}
	score := 0
	if n == q || CompactName(name) == CompactName(query) {
		score += 100
	} else if tokenPrefix(n, q) {
		score += 70
	} else if cn, cq := CompactName(name), CompactName(query); len(cn) >= 8 && len(cq) >= 8 && (strings.HasPrefix(cq, cn) || strings.HasPrefix(cn, cq)) {
		score += 70
	} else if strings.Contains(n, q) {
		score += 50
	} else if strings.Contains(q, n) {
		score += 40
	}
	cover := TokenCoverage(query, name)
	score += int(cover * 80)
	ntoks := strings.Fields(n)
	qtoks := strings.Fields(q)
	score -= max(0, len(ntoks)-len(qtoks)) * 2
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
