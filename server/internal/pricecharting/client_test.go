package pricecharting

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseProductJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/products") {
			_, _ = w.Write([]byte(`{"status":"success","products":[{"id":"3584","product-name":"Super Mario Sunshine","console-name":"Gamecube"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","id":"3584","product-name":"Super Mario Sunshine","console-name":"Gamecube","loose-price":2495,"cib-price":5999,"new-price":12000}`))
	}))
	t.Cleanup(srv.Close)
	c := New("tok")
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL
	p, err := c.ProductByUPC(context.Background(), "045496590376")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Super Mario Sunshine" || p.Loose == nil || *p.Loose != 2495 {
		t.Fatalf("%+v", p)
	}
	list, err := c.Search(context.Background(), "Super Mario Sunshine")
	if err != nil || len(list) != 1 || list[0].ID != "3584" {
		t.Fatalf("%v %+v", err, list)
	}
}
