package ebay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQuoteFromBrowse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/token") {
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":7200}`))
			return
		}
		if r.URL.Query().Get("gtin") == "" && r.URL.Query().Get("q") == "" {
			t.Errorf("missing search %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"itemSummaries":[
			{"title":"Super Mario Sunshine Nintendo GameCube CIB","condition":"Used","conditionId":"3000","price":{"value":"54.99","currency":"USD"}},
			{"title":"Super Mario Sunshine GameCube disc only","condition":"Used","conditionId":"3000","price":{"value":"19.00","currency":"USD"}},
			{"title":"Super Mario Sunshine Nintendo GameCube","condition":"New","conditionId":"1000","price":{"value":"120.00","currency":"USD"}},
			{"title":"Random Zelda lot of 10","condition":"Used","conditionId":"3000","price":{"value":"5.00","currency":"USD"}},
			{"title":"Super Mario Sunshine GameCube CIB signed exclusive","condition":"Used","conditionId":"3000","price":{"value":"999.00","currency":"USD"}}
		]}`))
	}))
	t.Cleanup(srv.Close)
	c := New("id", "secret", "EBAY_US")
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL
	c.TokenURL = srv.URL + "/identity/v1/oauth2/token"
	q, err := c.Quote(context.Background(), "Super Mario Sunshine", "Nintendo GameCube", "")
	if err != nil {
		t.Fatal(err)
	}
	if q.Listings != 3 {
		t.Fatalf("listings %d", q.Listings)
	}
	if q.New == nil || *q.New != 12000 {
		t.Fatalf("new %+v", q.New)
	}
	if q.Loose == nil || *q.Loose != 1900 {
		t.Fatalf("loose %+v", q.Loose)
	}
	if q.CIB == nil || *q.CIB != 5499 {
		t.Fatalf("cib %+v", q.CIB)
	}
}

func TestSkipSignedAndExclusiveListings(t *testing.T) {
	in := []listing{
		{Title: "The Last of Us Firefly Edition PS3 CIB", Cents: 8000},
		{Title: "The Last of Us Firefly Edition PS3 CIB signed exclusive", Cents: 99900},
		{Title: "The Last of Us Firefly Edition autographed", Cents: 50000},
		{Title: "The Last of Us Firefly Edition numbered 12/100", Cents: 40000},
	}
	got := filterListings(in, "The Last of Us Firefly Edition", "PlayStation 3")
	if len(got) != 1 || got[0].Cents != 8000 {
		t.Fatalf("%+v", got)
	}
	if skipListingTitle("The Last of Us Firefly Edition unsigned CIB") {
		t.Fatal("unsigned")
	}
}

func TestMedianAveragesEvenCounts(t *testing.T) {
	got := medianPtr([]int{4000, 99900})
	if got == nil || *got != 51950 {
		t.Fatalf("%v", got)
	}
	got = medianPtr([]int{1900})
	if got == nil || *got != 1900 {
		t.Fatalf("%v", got)
	}
	if medianPtr(nil) != nil {
		t.Fatal("empty")
	}
}

func TestParseUSDCents(t *testing.T) {
	cents, ok := parseUSDCents("24.99", "USD")
	if !ok || cents != 2499 {
		t.Fatalf("%d %v", cents, ok)
	}
	if _, ok := parseUSDCents("10.00", "EUR"); ok {
		t.Fatal("eur")
	}
}
