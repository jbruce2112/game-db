package barcode

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode"
)

func TestStressNoisyTitles(t *testing.T) {
	n := 400
	switch strings.ToLower(strings.TrimSpace(os.Getenv("STRESS"))) {
	case "1", "true", "yes", "extreme":
		n = 8000
	}
	publishers := []string{"Atlus", "Capcom", "Nintendo", "Sega", "Square Enix"}
	platforms := []string{"PlayStation 4", "Nintendo Switch", "Xbox One", "PlayStation 5", "Xbox 360"}
	fail := 0
	for i := 0; i < n; i++ {
		core := []string{
			"Shin Megami Tensei III: Nocturne HD Remaster",
			"Grand Theft Auto: The Trilogy - The Definitive Edition",
			"Super Mario Sunshine",
			"The Legend of Zelda: Breath of the Wild",
			"Persona 5 Royal",
		}[i%5]
		code := fmt.Sprintf("%012d", 700000000000+i)
		noisy := fmt.Sprintf("%s  %s  %s  [Physical]  (%s)  %s",
			core, publishers[i%len(publishers)], platforms[i%len(platforms)], []string{"New", "Used", "Digital"}[i%3], code)
		if i%4 == 0 {
			noisy = strings.ToUpper(noisy)
		}
		q := SearchQuery(noisy)
		if q == "" {
			t.Errorf("empty query from %q", noisy)
			fail++
			continue
		}
		if strings.Contains(strings.ToLower(q), "physical") || strings.Contains(q, "[") {
			t.Errorf("dirty brackets %q -> %q", noisy, q)
			fail++
		}
		if containsDigitRun(q, 8) {
			t.Errorf("barcode leaked %q -> %q", noisy, q)
			fail++
		}
		hint := PlatformHint(noisy)
		if hint == "" {
			t.Errorf("missing platform hint %q", noisy)
			fail++
		}
		if d := PlatformDisplay(noisy); d == "" {
			t.Errorf("missing platform display %q", noisy)
			fail++
		}
		qs := SearchQueries(noisy)
		if len(qs) == 0 || qs[0] != q {
			t.Errorf("SearchQueries[0] %v vs %q", qs, q)
			fail++
		}
		if fail > 20 {
			t.Fatal("too many noisy-title failures")
		}
	}
}

func containsDigitRun(s string, n int) bool {
	run := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			run++
			if run >= n {
				return true
			}
		} else {
			run = 0
		}
	}
	return false
}
