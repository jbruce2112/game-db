package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"game-db/internal/barcode"
	"game-db/internal/model"
	"game-db/internal/pricecharting"
	"game-db/internal/store"
)

type priceSource interface {
	ProductByUPC(ctx context.Context, upc string) (pricecharting.Product, error)
	Search(ctx context.Context, q string) ([]pricecharting.Product, error)
}

func (h *Handler) KickPriceBackfill() {
	if !h.cfg.PricesConfigured() {
		return
	}
	if !h.priceBackfillBusy.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer h.priceBackfillBusy.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		h.backfillPrices(ctx)
	}()
}

func (h *Handler) backfillPrices(ctx context.Context) {
	items, err := h.store.List(ctx, store.ListFilter{})
	if err != nil {
		h.log.Error("price backfill list", "err", err)
		return
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	quotes, err := h.store.PriceQuotes(ctx, ids)
	if err != nil {
		h.log.Error("price backfill quotes", "err", err)
		return
	}
	now := time.Now()
	missing, fetched, notFound := 0, 0, 0
	for i := range items {
		if ctx.Err() != nil {
			h.log.Warn("price backfill cancelled", "fetched", fetched)
			return
		}
		key := store.QuoteKey(items[i])
		q, ok := quotes[items[i].ID]
		if ok && !store.QuoteStale(q, key, now) {
			continue
		}
		missing++
		got, err := h.refreshPrice(ctx, items[i])
		if err != nil {
			h.log.Warn("price fetch", "id", items[i].ID, "err", err)
			continue
		}
		if got.Status == "ok" {
			fetched++
		} else {
			notFound++
		}
	}
	h.log.Info("price backfill", "items", len(items), "missing", missing, "fetched", fetched, "not_found", notFound)
}

func (h *Handler) attachValues(ctx context.Context, items []model.Item) {
	if len(items) == 0 {
		return
	}
	ids := make([]string, 0, len(items))
	for i := range items {
		ids = append(ids, items[i].ID)
	}
	quotes, err := h.store.PriceQuotes(ctx, ids)
	if err != nil {
		h.log.Warn("price attach", "err", err)
		return
	}
	for i := range items {
		if q, ok := quotes[items[i].ID]; ok {
			items[i].Value = q.ToValue()
		}
	}
}

func (h *Handler) ensurePrice(ctx context.Context, item *model.Item) {
	if item == nil || !h.cfg.PricesConfigured() {
		return
	}
	key := store.QuoteKey(*item)
	q, ok, err := h.store.PriceQuote(ctx, item.ID)
	if err != nil {
		h.log.Warn("price get", "id", item.ID, "err", err)
		return
	}
	if ok && !store.QuoteStale(q, key, time.Now()) {
		item.Value = q.ToValue()
		return
	}
	got, err := h.refreshPrice(ctx, *item)
	if err != nil {
		h.log.Warn("price fetch", "id", item.ID, "err", err)
		if ok {
			item.Value = q.ToValue()
		}
		return
	}
	item.Value = got.ToValue()
}

func (h *Handler) refreshPrice(ctx context.Context, item model.Item) (store.PriceQuote, error) {
	key := store.QuoteKey(item)
	q := store.PriceQuote{
		ItemID:   item.ID,
		QueryKey: key,
		Status:   "not_found",
	}
	prod, err := h.lookupPriceProduct(ctx, item)
	if err != nil {
		return q, err
	}
	if prod.ID != "" {
		q.Status = "ok"
		q.PCID = prod.ID
		q.ProductName = prod.Name
		q.ConsoleName = prod.Console
		q.URL = prod.URL
		q.Source = prod.Source
		q.Listings = prod.Listings
		q.LooseCents = prod.Loose
		q.CIBCents = prod.CIB
		q.NewCents = prod.New
	}
	if err := h.store.UpsertPriceQuote(ctx, q); err != nil {
		return q, err
	}
	return q, nil
}

type pricedHit struct {
	ID, Name, Console, URL, Source string
	Listings                       int
	Loose, CIB, New                *int
}

func (h *Handler) lookupPriceProduct(ctx context.Context, item model.Item) (pricedHit, error) {
	if h.ebay != nil {
		code := ""
		if item.Barcode != nil {
			code = *item.Barcode
		}
		got, err := h.ebay.Quote(ctx, item.Title, item.Platform, code)
		if err != nil {
			return pricedHit{}, err
		}
		if got.Listings > 0 && (got.Loose != nil || got.CIB != nil || got.New != nil) {
			return pricedHit{
				ID: "ebay", Name: got.Name, Console: got.Console, URL: got.URL,
				Source: "ebay", Listings: got.Listings,
				Loose: got.Loose, CIB: got.CIB, New: got.New,
			}, nil
		}
	}
	if h.pc == nil {
		return pricedHit{}, nil
	}
	if item.Barcode != nil && *item.Barcode != "" {
		for _, code := range barcode.Variants(*item.Barcode) {
			p, err := h.pc.ProductByUPC(ctx, code)
			if err != nil {
				return pricedHit{}, err
			}
			if p.ID != "" {
				return pcHit(p), nil
			}
		}
	}
	hits, err := h.pc.Search(ctx, item.Title)
	if err != nil {
		return pricedHit{}, err
	}
	region := ""
	if item.Region != nil {
		region = *item.Region
	}
	best, _ := pricecharting.PickBest(hits, item.Title, item.Platform, region)
	if best.ID == "" {
		return pricedHit{}, nil
	}
	return pcHit(best), nil
}

func pcHit(p pricecharting.Product) pricedHit {
	return pricedHit{
		ID: p.ID, Name: p.Name, Console: p.Console, URL: p.URL(),
		Source: "pricecharting", Loose: p.Loose, CIB: p.CIB, New: p.New,
	}
}

type priceCheckResponse struct {
	Title    string       `json:"title"`
	Platform string       `json:"platform"`
	Barcode  string       `json:"barcode,omitempty"`
	Status   string       `json:"status"`
	Value    *model.Value `json:"value"`
}

func (h *Handler) checkPrice(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.PricesConfigured() {
		writeErr(w, http.StatusServiceUnavailable, "pricing is not configured")
		return
	}
	title := strings.TrimSpace(r.URL.Query().Get("title"))
	platform := strings.TrimSpace(r.URL.Query().Get("platform"))
	region := strings.TrimSpace(r.URL.Query().Get("region"))
	rawCode := strings.TrimSpace(r.URL.Query().Get("barcode"))

	var code *string
	if rawCode != "" {
		normalized, err := barcode.Normalize(rawCode)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		code = &normalized
	}
	if title == "" && code == nil {
		writeErr(w, http.StatusBadRequest, "title or barcode is required")
		return
	}

	item := model.Item{Title: title, Platform: platform, Barcode: code}
	if region != "" {
		if n := model.NormalizeRegion(region); n != nil {
			item.Region = n
		}
	}

	prod, err := h.lookupPriceProduct(r.Context(), item)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}

	out := priceCheckResponse{
		Title:    title,
		Platform: platform,
		Status:   "not_found",
	}
	if code != nil {
		out.Barcode = *code
	}
	if prod.ID != "" {
		out.Status = "ok"
		out.Value = &model.Value{
			PCID:        prod.ID,
			ProductName: prod.Name,
			ConsoleName: prod.Console,
			URL:         prod.URL,
			Source:      prod.Source,
			Listings:    prod.Listings,
			LooseCents:  prod.Loose,
			CIBCents:    prod.CIB,
			NewCents:    prod.New,
		}
	}
	writeJSON(w, http.StatusOK, out)
}
