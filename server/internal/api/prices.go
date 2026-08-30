package api

import (
	"context"
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
	if h.pc == nil || !h.cfg.PriceChartingConfigured() {
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
	if item == nil || h.pc == nil || !h.cfg.PriceChartingConfigured() {
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
		q.URL = prod.URL()
		q.LooseCents = prod.Loose
		q.CIBCents = prod.CIB
		q.NewCents = prod.New
	}
	if err := h.store.UpsertPriceQuote(ctx, q); err != nil {
		return q, err
	}
	return q, nil
}

func (h *Handler) lookupPriceProduct(ctx context.Context, item model.Item) (pricecharting.Product, error) {
	if item.Barcode != nil && *item.Barcode != "" {
		for _, code := range barcode.Variants(*item.Barcode) {
			p, err := h.pc.ProductByUPC(ctx, code)
			if err != nil {
				return pricecharting.Product{}, err
			}
			if p.ID != "" {
				return p, nil
			}
		}
	}
	hits, err := h.pc.Search(ctx, item.Title)
	if err != nil {
		return pricecharting.Product{}, err
	}
	region := ""
	if item.Region != nil {
		region = *item.Region
	}
	best, _ := pricecharting.PickBest(hits, item.Title, item.Platform, region)
	return best, nil
}
