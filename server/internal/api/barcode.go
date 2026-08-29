package api

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"game-db/internal/barcode"
	"game-db/internal/igdb"
	"game-db/internal/store"
)

type barcodeSearchResponse struct {
	Barcode      string          `json:"barcode"`
	ProductTitle string          `json:"product_title"`
	Query        string          `json:"query"`
	Source       string          `json:"source"`
	Platform     string          `json:"platform,omitempty"`
	PlatformHint string          `json:"platform_hint,omitempty"`
	LookupError  string          `json:"lookup_error,omitempty"`
	Games        []igdb.Game     `json:"games"`
	Owned        []store.OwnedCopy `json:"owned"`
}

func (h *Handler) searchBarcode(w http.ResponseWriter, r *http.Request) {
	code, err := barcode.Normalize(r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	variants := barcode.Variants(code)
	owned, err := h.store.ListByBarcodes(r.Context(), variants)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	out := barcodeSearchResponse{
		Barcode: code,
		Games:   []igdb.Game{},
		Owned:   owned,
	}

	cached, ok, err := h.store.GetBarcodeCache(r.Context(), variants)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	useCache := ok && (cached.Found || time.Since(cached.UpdatedAt) < 7*24*time.Hour)
	if useCache {
		out.ProductTitle = cached.ProductTitle
		out.Source = cached.Source
		if out.Source == "" && cached.Found {
			out.Source = "cache"
		}
	} else {
		p, lerr := h.lookupProduct(r.Context(), variants)
		if lerr != nil {
			out.LookupError = lerr.Error()
		} else if p.Title != "" {
			out.ProductTitle = p.Title
			out.Source = p.Source
		}
		_ = h.store.PutBarcodeCache(r.Context(), store.BarcodeCache{
			Barcode:      code,
			ProductTitle: out.ProductTitle,
			Query:        barcode.SearchQuery(out.ProductTitle),
			Source:       out.Source,
			Found:        out.ProductTitle != "",
		})
	}
	if out.ProductTitle != "" {
		out.Query = barcode.SearchQuery(out.ProductTitle)
	}
	out.PlatformHint = barcode.PlatformHint(out.ProductTitle)
	out.Platform = barcode.PlatformDisplay(out.ProductTitle)

	if out.ProductTitle != "" && h.igdb != nil && h.cfg.IGDBConfigured() {
		games, err := h.searchIGDBFallbacks(r, out.ProductTitle)
		if err != nil {
			if out.LookupError == "" {
				out.LookupError = err.Error()
			}
		} else {
			out.Games = rankGames(games, out.Query, out.PlatformHint)
		}
		if cached.IGDBGameID != nil {
			out.Games = prependGame(out.Games, r, h, *cached.IGDBGameID)
		}
	}

	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) lookupProduct(ctx context.Context, codes []string) (barcode.Product, error) {
	if h.productLookup != nil {
		return h.productLookup(ctx, codes)
	}
	return barcode.Lookup(ctx, barcode.HTTPClient(), codes)
}

func prependGame(games []igdb.Game, r *http.Request, h *Handler, id int64) []igdb.Game {
	g, err := h.igdb.Game(r.Context(), id)
	if err != nil {
		return games
	}
	out := []igdb.Game{g}
	for _, existing := range games {
		if existing.ID == g.ID {
			continue
		}
		out = append(out, existing)
	}
	return out
}

func (h *Handler) searchIGDBFallbacks(r *http.Request, productTitle string) ([]igdb.Game, error) {
	var last error
	seen := map[int64]struct{}{}
	var out []igdb.Game
	for _, q := range barcode.SearchQueries(productTitle) {
		games, err := h.igdb.SearchGames(r.Context(), q, 0)
		if err != nil {
			last = err
			continue
		}
		for _, g := range games {
			if _, ok := seen[g.ID]; ok {
				continue
			}
			seen[g.ID] = struct{}{}
			out = append(out, g)
		}
		if len(out) > 0 {
			break
		}
	}
	if len(out) == 0 && last != nil {
		return nil, last
	}
	return out, nil
}

func rankGames(games []igdb.Game, query, hint string) []igdb.Game {
	sort.SliceStable(games, func(i, j int) bool {
		return barcode.NameScore(games[i].Name, query, platformNames(games[i]), hint) >
			barcode.NameScore(games[j].Name, query, platformNames(games[j]), hint)
	})
	return games
}

func platformNames(g igdb.Game) []string {
	out := make([]string, 0, len(g.Platforms))
	for _, p := range g.Platforms {
		out = append(out, p.Name)
	}
	return out
}

func parseBarcodePtr(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	s := strings.TrimSpace(*raw)
	if s == "" {
		return nil, nil
	}
	code, err := barcode.Normalize(s)
	if err != nil {
		return nil, err
	}
	return &code, nil
}
