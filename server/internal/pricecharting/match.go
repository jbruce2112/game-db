package pricecharting

import (
	"strings"

	"game-db/internal/barcode"
)

// ConsoleName maps an IGDB/library platform + region to PriceCharting console names,
// most specific first.
func ConsoleName(platform, region string) []string {
	base := canonicalConsole(platform)
	if base == "" {
		return nil
	}
	region = strings.ToLower(strings.TrimSpace(region))
	jp, pal := japaneseName(base), "PAL "+base
	switch region {
	case "jp":
		return uniqueStrings(jp, base, pal)
	case "eu", "au":
		return uniqueStrings(pal, base, jp)
	default:
		return uniqueStrings(base, pal, jp)
	}
}

func canonicalConsole(platform string) string {
	p := fold(platform)
	switch {
	case p == "nintendo gamecube" || p == "gamecube" || p == "game cube":
		return "Gamecube"
	case p == "nintendo switch 2" || p == "switch 2":
		return "Nintendo Switch 2"
	case p == "nintendo switch" || p == "switch":
		return "Nintendo Switch"
	case p == "nintendo 64" || p == "n64":
		return "Nintendo 64"
	case p == "nintendo wii u" || p == "wii u":
		return "Wii U"
	case p == "nintendo wii" || p == "wii":
		return "Wii"
	case p == "nintendo entertainment system" || p == "nes":
		return "NES"
	case p == "super nintendo" || p == "super nintendo entertainment system" || p == "snes":
		return "Super Nintendo"
	case p == "game boy advance" || p == "gba":
		return "GameBoy Advance"
	case p == "game boy color" || p == "gbc":
		return "GameBoy Color"
	case p == "game boy" || p == "gb":
		return "GameBoy"
	case p == "nintendo ds" || p == "nds":
		return "Nintendo DS"
	case p == "nintendo 3ds" || p == "3ds":
		return "Nintendo 3DS"
	case p == "playstation 5" || p == "ps5":
		return "Playstation 5"
	case p == "playstation 4" || p == "ps4":
		return "Playstation 4"
	case p == "playstation 3" || p == "ps3":
		return "Playstation 3"
	case p == "playstation 2" || p == "ps2":
		return "Playstation 2"
	case p == "playstation vita" || p == "ps vita" || p == "vita":
		return "Playstation Vita"
	case p == "playstation" || p == "ps1" || p == "psx":
		return "Playstation"
	case p == "psp":
		return "PSP"
	case p == "xbox series x" || p == "xbox series s" || p == "xbox series x s":
		return "Xbox Series X"
	case p == "xbox one":
		return "Xbox One"
	case p == "xbox 360":
		return "Xbox 360"
	case p == "xbox":
		return "Xbox"
	case p == "sega genesis" || p == "genesis" || p == "mega drive":
		return "Sega Genesis"
	case p == "sega saturn" || p == "saturn":
		return "Sega Saturn"
	case p == "dreamcast" || p == "sega dreamcast":
		return "Sega Dreamcast"
	default:
		return ""
	}
}

func japaneseName(base string) string {
	switch base {
	case "NES":
		return "Famicom"
	case "Super Nintendo":
		return "Super Famicom"
	case "Sega Genesis":
		return "JP Mega Drive"
	default:
		return "JP " + base
	}
}

func uniqueStrings(in ...string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func fold(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

// PickBest chooses the PriceCharting row that best matches title + platform + region.
func PickBest(products []Product, title, platform, region string) (Product, int) {
	want := ConsoleName(platform, region)
	best := Product{}
	bestScore := 0
	for _, p := range products {
		score := barcode.NameScore(p.Name, title, nil, "")
		score += consoleScore(p.Console, want)
		if strings.Contains(strings.ToLower(p.Name), "[") && !strings.Contains(strings.ToLower(title), "[") {
			score -= 8
		}
		if score > bestScore {
			bestScore = score
			best = p
		}
	}
	if bestScore < 40 {
		return Product{}, bestScore
	}
	return best, bestScore
}

func consoleScore(got string, want []string) int {
	g := fold(got)
	for i, w := range want {
		if fold(w) == g {
			return 50 - i*8
		}
	}
	for _, w := range want {
		wf, gf := fold(w), g
		if strings.Contains(gf, wf) || strings.Contains(wf, gf) {
			return 18
		}
	}
	return 0
}
