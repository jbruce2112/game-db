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
		"Battlefield Bad Company 2 (PC DVD)": "Battlefield Bad Company 2",
		"The Legend of Zelda: Breath of the Wild Master Edition - Nintendo Switch": "The Legend of Zelda: Breath of the Wild Master Edition",
		"Super Mario Sunshine (Gamecube)": "Super Mario Sunshine",
		"Shin Megami Tensei III: Nocturne HD Remaster  Atlus  PlayStation 4  [Physical]  730865220366": "Shin Megami Tensei III: Nocturne HD Remaster",
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
