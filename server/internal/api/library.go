package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"game-db/internal/export"
	"game-db/internal/model"
	"game-db/internal/store"
)

type writeBody struct {
	ID             *string `json:"id"`
	Title          *string `json:"title"`
	Platform       *string `json:"platform"`
	IGDBPlatformID *int64  `json:"igdb_platform_id"`
	Region         *string `json:"region"`
	Completeness   *string `json:"completeness"`
	Notes          *string `json:"notes"`
	IGDBGameID     *int64  `json:"igdb_game_id"`
	Barcode        *string `json:"barcode"`
	CreatedAt      *string `json:"created_at"`
	UpdatedAt      *string `json:"updated_at"`
}

func (h *Handler) listLibrary(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.List(r.Context(), store.ListFilter{
		Platform: r.URL.Query().Get("platform"),
		Query:    r.URL.Query().Get("q"),
		Sort:     r.URL.Query().Get("sort"),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.attachValues(r.Context(), items)
	h.KickCoverBackfill()
	h.KickPriceBackfill()
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) libraryCover(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := h.store.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && item.DeletedAt != nil) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !h.store.CoverExists(r.Context(), item.CoverID) {
		h.ensureCoverFromIGDB(r.Context(), &item)
	}
	if !h.store.CoverExists(r.Context(), item.CoverID) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	h.serveCoverFile(w, r, *item.CoverID)
}

func (h *Handler) serveCoverFile(w http.ResponseWriter, r *http.Request, coverID string) {
	ct, path, err := h.store.GetCover(r.Context(), coverID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, path)
}

func (h *Handler) ensureCoverFromIGDB(ctx context.Context, item *model.Item) {
	if item == nil || h.store.CoverExists(ctx, item.CoverID) {
		return
	}
	if h.igdb == nil || !h.cfg.IGDBConfigured() || item.IGDBGameID == nil {
		return
	}
	chI, loaded := h.coverInflight.LoadOrStore(item.ID, make(chan struct{}))
	ch := chI.(chan struct{})
	if loaded {
		select {
		case <-ch:
		case <-ctx.Done():
			return
		}
		if fresh, err := h.store.Get(ctx, item.ID); err == nil {
			*item = fresh
		}
		return
	}
	defer func() {
		close(ch)
		h.coverInflight.Delete(item.ID)
	}()
	before := ""
	if item.CoverID != nil {
		before = *item.CoverID
	}
	h.log.Info("cover fetch", "id", item.ID, "igdb", *item.IGDBGameID)
	h.attachCoverFromIGDBCtx(ctx, item)
	if item.CoverID == nil || !h.store.CoverExists(ctx, item.CoverID) {
		return
	}
	if *item.CoverID != before {
		if err := h.store.SetCoverID(ctx, item.ID, *item.CoverID); err != nil {
			h.log.Warn("cover set id", "id", item.ID, "err", err)
		}
	}
}

func (h *Handler) importLibraryCSV(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "could not read body")
		return
	}
	items, err := export.ParseLibraryCSV(raw)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if h.igdb != nil && h.cfg.IGDBConfigured() {
		for i := range items {
			h.attachCoverFromIGDBCtx(r.Context(), &items[i])
		}
	}
	if err := h.store.ReplaceAll(r.Context(), items); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": len(items)})
}

func (h *Handler) attachCoverFromIGDBCtx(ctx context.Context, item *model.Item) {
	if h.igdb == nil || item.IGDBGameID == nil {
		return
	}
	game, err := h.igdb.Game(ctx, *item.IGDBGameID)
	if err != nil {
		h.log.Warn("cover igdb game", "id", item.ID, "igdb", *item.IGDBGameID, "err", err)
		return
	}
	if game.CoverImageID == "" {
		return
	}
	if err := h.cacheCover(ctx, item, game.CoverImageID); err != nil {
		h.log.Warn("cover cache", "id", item.ID, "err", err)
	}
}

func (h *Handler) exportLibraryCSV(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.List(r.Context(), store.ListFilter{
		Platform: r.URL.Query().Get("platform"),
		Query:    r.URL.Query().Get("q"),
		Sort:     r.URL.Query().Get("sort"),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	raw, err := export.LibraryCSV(items)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	name := export.Filename(time.Now())
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (h *Handler) getLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := h.store.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && item.DeletedAt != nil) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.ensurePrice(r.Context(), &item)
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) createLibrary(w http.ResponseWriter, r *http.Request) {
	var body writeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	now := model.TimeUTC(time.Now())
	item := model.Item{
		Completeness: model.CompletenessUnknown,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if body.ID != nil && *body.ID != "" {
		id, err := parseID(*body.ID)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid id")
			return
		}
		item.ID = id
	} else {
		item.ID = newID()
	}
	if body.CreatedAt != nil {
		if t, err := model.ParseTime(*body.CreatedAt); err == nil && !t.IsZero() {
			item.CreatedAt = t
		}
	}
	if body.UpdatedAt != nil {
		if t, err := model.ParseTime(*body.UpdatedAt); err == nil && !t.IsZero() {
			item.UpdatedAt = t
		}
	}
	if body.Notes != nil {
		item.Notes = *body.Notes
	}
	if body.Barcode != nil {
		code, err := parseBarcodePtr(body.Barcode)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		item.Barcode = code
	}
	if body.Completeness != nil {
		item.Completeness = model.NormalizeCompleteness(*body.Completeness)
	}
	if body.Region != nil {
		item.Region = model.NormalizeRegion(*body.Region)
	}
	if body.IGDBPlatformID != nil {
		item.IGDBPlatformID = body.IGDBPlatformID
	}

	if body.IGDBGameID != nil && h.cfg.IGDBConfigured() {
		if err := h.snapshotFromIGDB(r, &item, *body.IGDBGameID, body); err != nil {
			h.log.Error("igdb snapshot", "err", err)
			writeErr(w, http.StatusBadGateway, "IGDB lookup failed")
			return
		}
	}

	if body.Title != nil && strings.TrimSpace(*body.Title) != "" {
		item.Title = strings.TrimSpace(*body.Title)
	}
	if body.Platform != nil && strings.TrimSpace(*body.Platform) != "" {
		item.Platform = strings.TrimSpace(*body.Platform)
	}
	if item.Title == "" || item.Platform == "" {
		writeErr(w, http.StatusBadRequest, "title and platform are required")
		return
	}

	out, err := h.store.Insert(r.Context(), item)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if out.Barcode != nil && out.IGDBGameID != nil {
		_ = h.store.RememberBarcodeGame(r.Context(), *out.Barcode, *out.IGDBGameID)
	}
	h.ensurePrice(r.Context(), &out)
	writeJSON(w, http.StatusCreated, out)
}

func (h *Handler) snapshotFromIGDB(r *http.Request, item *model.Item, gameID int64, body writeBody) error {
	game, err := h.igdb.Game(r.Context(), gameID)
	if err != nil {
		return err
	}
	item.IGDBGameID = &gameID
	item.Title = game.Name
	if body.IGDBPlatformID != nil {
		for _, p := range game.Platforms {
			if p.ID == *body.IGDBPlatformID {
				item.Platform = p.Name
				item.IGDBPlatformID = body.IGDBPlatformID
				break
			}
		}
		if item.Platform == "" {
			name, err := h.store.PlatformName(r.Context(), *body.IGDBPlatformID)
			if err == nil {
				item.Platform = name
			}
		}
	}
	if item.Platform == "" && len(game.Platforms) > 0 {
		item.Platform = game.Platforms[0].Name
		id := game.Platforms[0].ID
		item.IGDBPlatformID = &id
	}
	if game.CoverImageID != "" {
		if err := h.cacheCover(r.Context(), item, game.CoverImageID); err != nil {
			h.log.Warn("cover cache", "err", err)
		}
	}
	return nil
}

func (h *Handler) cacheCover(ctx context.Context, item *model.Item, imageID string) error {
	ct, data, err := h.igdb.DownloadCover(ctx, imageID)
	if err != nil {
		return err
	}
	id := newID()
	if item.CoverID != nil && *item.CoverID != "" {
		id = *item.CoverID
	}
	name, err := saveCoverFile(h.store.DataDir, id, data)
	if err != nil {
		return err
	}
	if err := h.store.SaveCover(ctx, id, imageID, ct, name); err != nil {
		_ = os.Remove(filepath.Join(h.store.DataDir, "covers", name))
		return err
	}
	item.CoverID = &id
	return nil
}

func (h *Handler) patchLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := h.store.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && item.DeletedAt != nil) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var body writeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.Title != nil {
		item.Title = strings.TrimSpace(*body.Title)
	}
	if body.Platform != nil {
		item.Platform = strings.TrimSpace(*body.Platform)
	}
	if body.IGDBPlatformID != nil {
		item.IGDBPlatformID = body.IGDBPlatformID
	}
	if body.Region != nil {
		if *body.Region == "" {
			item.Region = nil
		} else {
			item.Region = model.NormalizeRegion(*body.Region)
		}
	}
	if body.Completeness != nil {
		item.Completeness = model.NormalizeCompleteness(*body.Completeness)
	}
	if body.Notes != nil {
		item.Notes = *body.Notes
	}
	if body.Barcode != nil {
		code, err := parseBarcodePtr(body.Barcode)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		item.Barcode = code
	}
	if item.Title == "" || item.Platform == "" {
		writeErr(w, http.StatusBadRequest, "title and platform are required")
		return
	}
	item.UpdatedAt = model.TimeUTC(time.Now())
	out, err := h.store.Replace(r.Context(), item)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.KickPriceBackfill()
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) deleteLibrary(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	item, err := h.store.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && item.DeletedAt != nil) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	now := model.TimeUTC(time.Now())
	item.DeletedAt = &now
	item.UpdatedAt = now
	out, err := h.store.Replace(r.Context(), item)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
