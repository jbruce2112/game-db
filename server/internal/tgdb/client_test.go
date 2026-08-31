package tgdb

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGamesByNameParsesFrontBoxart(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("apikey") != "k" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if !strings.Contains(r.URL.Path, "/Games/ByGameName") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("filter[platform]") != "2" {
			t.Errorf("platform filter %q", r.URL.Query().Get("filter[platform]"))
		}
		_, _ = io.WriteString(w, `{
			"code": 200,
			"data": {
				"count": 1,
				"games": [{"id": 42, "game_title": "Super Mario Sunshine", "platform": 2}]
			},
			"include": {
				"boxart": {
					"base_url": {
						"original": "https://cdn.example/original/",
						"large": "https://cdn.example/large/"
					},
					"data": {
						"42": [
							{"id": 1, "type": "boxart", "side": "back", "filename": "boxart/back/42-1.jpg", "resolution": "800x600"},
							{"id": 2, "type": "boxart", "side": "front", "filename": "boxart/front/42-1.jpg", "resolution": "600x800"},
							{"id": 3, "type": "screenshot", "side": "", "filename": "screenshots/42-1.jpg", "resolution": "1920x1080"}
						]
					}
				}
			}
		}`)
	}))
	t.Cleanup(srv.Close)

	c := New("k")
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL

	games, err := c.GamesByName(context.Background(), "Super Mario Sunshine", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 || games[0].Title != "Super Mario Sunshine" {
		t.Fatalf("%+v", games)
	}
	if games[0].FrontURL != "https://cdn.example/large/boxart/front/42-1.jpg" {
		t.Fatalf("front %q", games[0].FrontURL)
	}
	if games[0].SourceID != "tgdb:boxart/front/42-1.jpg" {
		t.Fatalf("source %q", games[0].SourceID)
	}
}

func TestPlatformsParsesMap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{
			"data": {
				"count": 2,
				"platforms": {
					"2": {"id": 2, "name": "Nintendo GameCube", "alias": "GC"},
					"4971": {"id": 4971, "name": "Nintendo Switch", "alias": "Switch"}
				}
			}
		}`)
	}))
	t.Cleanup(srv.Close)
	c := New("k")
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL
	plats, err := c.Platforms(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(plats) != 2 {
		t.Fatalf("%+v", plats)
	}
}

func TestSearchFrontPicksExactTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{
			"data": {
				"games": [
					{"id": 1, "game_title": "Mario Kart 64 (VC)", "platform": 3},
					{"id": 2, "game_title": "Mario Kart 64", "platform": 3}
				]
			},
			"include": {
				"boxart": {
					"base_url": {"large": "https://cdn.example/large/"},
					"data": {
						"1": [{"type": "boxart", "side": "front", "filename": "vc.jpg"}],
						"2": [{"type": "boxart", "side": "front", "filename": "retail.jpg"}]
					}
				}
			}
		}`)
	}))
	t.Cleanup(srv.Close)
	c := New("k")
	c.HTTP = srv.Client()
	c.BaseURL = srv.URL
	g, err := c.SearchFront(context.Background(), "Mario Kart 64", 3)
	if err != nil {
		t.Fatal(err)
	}
	if g.ID != 2 || !strings.HasSuffix(g.FrontURL, "/retail.jpg") {
		t.Fatalf("%+v", g)
	}
}
