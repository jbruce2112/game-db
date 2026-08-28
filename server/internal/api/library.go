package api

import (
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
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
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
			h.attachCoverFromIGDB(r, &items[i])
		}
	}
	if err := h.store.ReplaceAll(r.Context(), items); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": len(items)})
}

func (h *Handler) attachCoverFromIGDB(r *http.Request, item *model.Item) {
	if item.IGDBGameID == nil {
		return
	}
	game, err := h.igdb.Game(r.Context(), *item.IGDBGameID)
	if err != nil || game.CoverImageID == "" {
		return
	}
	_ = h.cacheCover(r, item, game.CoverImageID)
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
		if err := h.cacheCover(r, item, game.CoverImageID); err != nil {
			h.log.Warn("cover cache", "err", err)
		}
	}
	return nil
}

func (h *Handler) cacheCover(r *http.Request, item *model.Item, imageID string) error {
	ct, data, err := h.igdb.DownloadCover(r.Context(), imageID)
	if err != nil {
		return err
	}
	id := newID()
	name, err := saveCoverFile(h.store.DataDir, id, data)
	if err != nil {
		return err
	}
	if err := h.store.SaveCover(r.Context(), id, imageID, ct, name); err != nil {
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
