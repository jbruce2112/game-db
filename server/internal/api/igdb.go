package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"game-db/internal/igdb"
	"game-db/internal/store"
)

func (h *Handler) refreshPlatformsIfStale(ctx context.Context) {
	if h.igdb == nil || !h.cfg.IGDBConfigured() {
		return
	}
	stale, err := h.store.PlatformsStale(ctx, 24*time.Hour)
	if err != nil || !stale {
		return
	}
	plats, err := h.igdb.Platforms(ctx)
	if err != nil {
		h.log.Warn("igdb platforms", "err", err)
		return
	}
	mapped := make([]store.Platform, 0, len(plats))
	for _, p := range plats {
		mapped = append(mapped, store.Platform{
			ID: p.ID, Name: p.Name, Slug: p.Slug, Abbreviation: p.Abbreviation,
		})
	}
	if err := h.store.UpsertPlatforms(ctx, mapped); err != nil {
		h.log.Warn("upsert platforms", "err", err)
	}
}

func (h *Handler) platforms(w http.ResponseWriter, r *http.Request) {
	if !h.requireIGDB(w) {
		return
	}
	h.refreshPlatformsIfStale(r.Context())
	list, err := h.store.ListPlatforms(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []store.Platform{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"platforms": list})
}

func (h *Handler) searchGames(w http.ResponseWriter, r *http.Request) {
	if !h.requireIGDB(w) {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeErr(w, http.StatusBadRequest, "q is required")
		return
	}
	games, err := h.igdb.SearchGames(r.Context(), q, intQuery(r, "platform"))
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if games == nil {
		games = []igdb.Game{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"games": games})
}

func (h *Handler) cover(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	ct, path, err := h.store.GetCover(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, path)
}
