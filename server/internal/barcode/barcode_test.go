package barcode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	got, err := Normalize("045496590376")
	if err != nil || got != "045496590376" {
		t.Fatalf("got %q err %v", got, err)
	}
	got, err = Normalize("0 45496 59037 6")
	if err != nil || got != "045496590376" {
		t.Fatalf("spaces %q err %v", got, err)
	}
	got, err = Normalize("0045496590376")
	if err != nil || got != "045496590376" {
		t.Fatalf("ean13 %q err %v", got, err)
	}
	if _, err := Normalize("123"); err == nil {
		t.Fatal("expected error")
	}
}

func TestVariants(t *testing.T) {
	v := Variants("045496590376")
	if len(v) != 2 || v[0] != "045496590376" || v[1] != "0045496590376" {
		t.Fatalf("%v", v)
	}
}

func TestSearchQuery(t *testing.T) {
	cases := map[string]string{
		"Battlefield Bad Company 2 (PC DVD)":                                                           "Battlefield Bad Company 2",
		"The Legend of Zelda: Breath of the Wild Master Edition - Nintendo Switch":                     "The Legend of Zelda: Breath of the Wild Master Edition",
		"Super Mario Sunshine (Gamecube)":                                                              "Super Mario Sunshine",
		"Shin Megami Tensei III: Nocturne HD Remaster  Atlus  PlayStation 4  [Physical]  730865220366": "Shin Megami Tensei III: Nocturne HD Remaster",
		"Super Deluxe Games Tetris Effect: Connected For PlayStation 5":                                "Tetris Effect: Connected",
	}
	for in, want := range cases {
		if got := SearchQuery(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestSearchQueriesFallback(t *testing.T) {
	qs := SearchQueries("Shin Megami Tensei III: Nocturne HD Remaster - Nintendo Switch")
	if len(qs) < 1 || !strings.Contains(strings.ToLower(qs[0]), "nocturne") {
		t.Fatalf("%v", qs)
	}
}

func TestFoldNameStripsAccentsAndPunct(t *testing.T) {
	if CompactName("New Pokémon Snap") != CompactName("New Pokemon Snap") {
		t.Fatalf("%q vs %q", CompactName("New Pokémon Snap"), CompactName("New Pokemon Snap"))
	}
	if CompactName("WWF Warzone") != CompactName("WWF War Zone") {
		t.Fatal(CompactName("WWF Warzone"), CompactName("WWF War Zone"))
	}
}

func TestTokenCoverageDoublePack(t *testing.T) {
	if TokenCoverage("Inside - Limbo Double Pack", "Inside & Limbo Bundle") < 0.8 {
		t.Fatalf("coverage %v", TokenCoverage("Inside - Limbo Double Pack", "Inside & Limbo Bundle"))
	}
}

func TestTokenCoverageCloseTitles(t *testing.T) {
	if TokenCoverage("Sonic Triple Trouble", "Sonic the Hedgehog: Triple Trouble") < 0.8 {
		t.Fatal("sonic triple")
	}
	if TokenCoverage("Deadly Premonitions: The Director's Cut", "Deadly Premonition: Director's Cut") < 0.8 {
		t.Fatal("premonition")
	}
}

func TestNameScorePrefersExactOverPack(t *testing.T) {
	q := "Shin Megami Tensei III Nocturne HD Remaster"
	base := NameScore("Shin Megami Tensei III: Nocturne - HD Remaster", q, []string{"Nintendo Switch"}, "nintendo switch")
	pack := NameScore("Shin Megami Tensei III: Nocturne - HD Remaster: Maniax Pack", q, []string{"Nintendo Switch"}, "nintendo switch")
	if base <= pack {
		t.Fatalf("base %d pack %d", base, pack)
	}
}

func TestPlatformHint(t *testing.T) {
	h := PlatformHint("Breath of the Wild Master Edition - Nintendo Switch")
	if h != "nintendo switch" {
		t.Fatalf("%q", h)
	}
}

func TestLookupUPCitemdb(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("upc") != "014633190366" {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"OK","items":[{"title":"Battlefield Bad Company 2 (PC DVD)","brand":"EA"}]}`))
	}))
	t.Cleanup(srv.Close)
	old := UPCItemDBURL
	oldOFF := OpenProductsURL
	UPCItemDBURL = srv.URL
	OpenProductsURL = srv.URL + "/off/"
	t.Cleanup(func() {
		UPCItemDBURL = old
		OpenProductsURL = oldOFF
	})

	p, err := Lookup(context.Background(), srv.Client(), []string{"014633190366"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Battlefield Bad Company 2 (PC DVD)" || p.Source != "upcitemdb" {
		t.Fatalf("%+v", p)
	}
}

func TestLookupFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/off/") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":1,"product":{"product_name":"Paper Mario","brands":"Nintendo"}}`))
			return
		}
		w.WriteHeader(404)
	}))
	t.Cleanup(srv.Close)
	old := UPCItemDBURL
	oldOFF := OpenProductsURL
	UPCItemDBURL = srv.URL + "/upc"
	OpenProductsURL = srv.URL + "/off/"
	t.Cleanup(func() {
		UPCItemDBURL = old
		OpenProductsURL = oldOFF
	})
	p, err := Lookup(context.Background(), srv.Client(), []string{"0045496000000"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Paper Mario" || p.Source != "openproductsfacts" {
		t.Fatalf("%+v", p)
	}
}

func TestLookupFallsBackToGoUPC(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/upc" || strings.HasPrefix(r.URL.Path, "/off/") {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><h1 class="product-name">Super Deluxe Games Tetris Effect: Connected For PlayStation 5</h1></html>`))
	}))
	t.Cleanup(srv.Close)
	old := UPCItemDBURL
	oldOFF := OpenProductsURL
	oldGo := GoUPCURL
	UPCItemDBURL = srv.URL + "/upc"
	OpenProductsURL = srv.URL + "/off/"
	GoUPCURL = srv.URL
	t.Cleanup(func() {
		UPCItemDBURL = old
		OpenProductsURL = oldOFF
		GoUPCURL = oldGo
	})
	p, err := Lookup(context.Background(), srv.Client(), []string{"4570101050335"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Super Deluxe Games Tetris Effect: Connected For PlayStation 5" || p.Source != "goupc" {
		t.Fatalf("%+v", p)
	}
}

func TestParseGoUPC(t *testing.T) {
	html := `<title>Super Deluxe Games Tetris Effect: Connected For PlayStation 5 — EAN 4570101050335 — Go-UPC</title>`
	if got := parseGoUPC(html); got != "Super Deluxe Games Tetris Effect: Connected For PlayStation 5" {
		t.Fatalf("%q", got)
	}
	if got := parseGoUPC(`<title>Invalid Value — Go-UPC</title>`); got != "" {
		t.Fatalf("invalid %q", got)
	}
}

func TestColoursCoverage(t *testing.T) {
	if TokenCoverage("Sonic Colours: Ultimate", "Sonic Colors: Ultimate") < 0.8 {
		t.Fatalf("coverage %v", TokenCoverage("Sonic Colours: Ultimate", "Sonic Colors: Ultimate"))
	}
}
