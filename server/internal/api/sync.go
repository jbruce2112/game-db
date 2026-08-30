package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"game-db/internal/model"
	"game-db/internal/store"
)

func (h *Handler) sync(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Cursor  int64        `json:"cursor"`
		Changes []model.Item `json:"changes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Changes == nil {
		body.Changes = []model.Item{}
	}
	res, err := h.store.Sync(r.Context(), body.Cursor, body.Changes, time.Now())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.attachValues(r.Context(), res.Changes)
	h.KickCoverBackfill()
	h.KickPriceBackfill()
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) KickCoverBackfill() {
	if h.igdb == nil || !h.cfg.IGDBConfigured() {
		return
	}
	if !h.coverBackfillBusy.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer h.coverBackfillBusy.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		h.backfillMissingCovers(ctx)
	}()
}

func (h *Handler) backfillMissingCovers(ctx context.Context) {
	items, err := h.store.List(ctx, store.ListFilter{})
	if err != nil {
		h.log.Error("cover backfill list", "err", err)
		return
	}
	missing, noIGDB, fetched := 0, 0, 0
	for i := range items {
		if ctx.Err() != nil {
			h.log.Warn("cover backfill cancelled", "fetched", fetched)
			return
		}
		if h.store.CoverExists(ctx, items[i].CoverID) {
			continue
		}
		missing++
		if items[i].IGDBGameID == nil {
			noIGDB++
			continue
		}
		h.ensureCoverFromIGDB(ctx, &items[i])
		if h.store.CoverExists(ctx, items[i].CoverID) {
			fetched++
		}
	}
	h.log.Info("cover backfill", "items", len(items), "missing", missing, "no_igdb_id", noIGDB, "fetched", fetched)
}
