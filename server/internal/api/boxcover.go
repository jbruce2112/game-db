package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"game-db/internal/model"
	"game-db/internal/store"
	"game-db/internal/tgdb"
)

type boxArtSource interface {
	Platforms(ctx context.Context) ([]tgdb.Platform, error)
	SearchFront(ctx context.Context, title string, platformID int64) (tgdb.Game, error)
	Download(ctx context.Context, imageURL string) (string, []byte, error)
}

func (h *Handler) libraryBoxCover(w http.ResponseWriter, r *http.Request) {
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
	if !h.store.CoverExists(r.Context(), item.BoxCoverID) {
		h.ensureBoxCover(r.Context(), &item)
	}
	if !h.store.CoverExists(r.Context(), item.BoxCoverID) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	h.serveCoverFile(w, r, *item.BoxCoverID)
}

func (h *Handler) KickBoxCoverBackfill() {
	if h.tgdb == nil || !h.cfg.TGDBConfigured() {
		return
	}
	if !h.boxBackfillBusy.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer h.boxBackfillBusy.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
		defer cancel()
		h.backfillMissingBoxCovers(ctx)
	}()
}

func (h *Handler) backfillMissingBoxCovers(ctx context.Context) {
	h.refreshTGDBPlatformsIfStale(ctx)
	items, err := h.store.List(ctx, store.ListFilter{})
	if err != nil {
		h.log.Error("box cover backfill list", "err", err)
		return
	}
	missing, fetched, missed := 0, 0, 0
	for i := range items {
		if ctx.Err() != nil {
			h.log.Warn("box cover backfill cancelled", "fetched", fetched, "missed", missed)
			return
		}
		if h.store.CoverExists(ctx, items[i].BoxCoverID) {
			continue
		}
		if h.store.BoxCoverMissed(ctx, items[i]) {
			continue
		}
		missing++
		h.ensureBoxCover(ctx, &items[i])
		if h.store.CoverExists(ctx, items[i].BoxCoverID) {
			fetched++
		} else {
			missed++
		}
		if (fetched+missed)%25 == 0 && (fetched+missed) > 0 {
			h.log.Info("box cover backfill progress", "fetched", fetched, "missed", missed, "left", missing-fetched-missed)
		}
	}
	h.log.Info("box cover backfill", "items", len(items), "tried", missing, "fetched", fetched, "missed", missed)
}

func (h *Handler) ensureBoxCover(ctx context.Context, item *model.Item) {
	if item == nil || h.store.CoverExists(ctx, item.BoxCoverID) {
		return
	}
	if h.tgdb == nil || !h.cfg.TGDBConfigured() {
		return
	}
	if h.store.BoxCoverMissed(ctx, *item) {
		return
	}
	chI, loaded := h.boxInflight.LoadOrStore(item.ID, make(chan struct{}))
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
		h.boxInflight.Delete(item.ID)
	}()

	h.refreshTGDBPlatformsIfStale(ctx)
	platID := h.tgdbPlatformID(ctx, item.Platform)
	if platID == nil {
		h.log.Info("box cover no platform", "id", item.ID, "platform", item.Platform)
		_ = h.store.RememberBoxCoverMiss(ctx, *item)
		return
	}

	h.log.Info("box cover fetch", "id", item.ID, "title", item.Title, "platform", item.Platform, "tgdb_plat", *platID)
	game, err := h.tgdb.SearchFront(ctx, item.Title, *platID)
	if err != nil {
		if errors.Is(err, tgdb.ErrNoFront) {
			_ = h.store.RememberBoxCoverMiss(ctx, *item)
			return
		}
		h.log.Warn("box cover search", "id", item.ID, "err", err)
		return
	}
	if err := h.cacheBoxCover(ctx, item, game.SourceID, game.FrontURL); err != nil {
		h.log.Warn("box cover cache", "id", item.ID, "err", err)
		return
	}
	if item.BoxCoverID == nil || !h.store.CoverExists(ctx, item.BoxCoverID) {
		return
	}
	if err := h.store.SetBoxCoverID(ctx, item.ID, *item.BoxCoverID); err != nil {
		h.log.Warn("box cover set id", "id", item.ID, "err", err)
	}
	_ = h.store.ClearBoxCoverMiss(ctx, item.ID)
}

func (h *Handler) cacheBoxCover(ctx context.Context, item *model.Item, sourceID, imageURL string) error {
	ct, data, err := h.tgdb.Download(ctx, imageURL)
	if err != nil {
		return err
	}
	id := newID()
	if item.BoxCoverID != nil && *item.BoxCoverID != "" {
		id = *item.BoxCoverID
	}
	name, err := saveCoverFile(h.store.DataDir, id, data)
	if err != nil {
		return err
	}
	if err := h.store.SaveCover(ctx, id, sourceID, ct, name); err != nil {
		_ = os.Remove(filepath.Join(h.store.DataDir, "covers", name))
		return err
	}
	item.BoxCoverID = &id
	item.BoxCoverURL = model.CoverURL(&id)
	return nil
}

func (h *Handler) decorateBoxCover(ctx context.Context, item *model.Item) {
	if item == nil {
		return
	}
	list := []model.Item{*item}
	h.attachBoxCoverOnDemand(ctx, list)
	item.BoxCoverURL = list[0].BoxCoverURL
}

func (h *Handler) attachBoxCoverOnDemand(ctx context.Context, items []model.Item) {
	if h.tgdb == nil || !h.cfg.TGDBConfigured() {
		return
	}
	for i := range items {
		if items[i].BoxCoverURL != nil {
			continue
		}
		if h.store.BoxCoverMissed(ctx, items[i]) {
			continue
		}
		u := "/v1/library/" + items[i].ID + "/box-cover"
		items[i].BoxCoverURL = &u
	}
}

func (h *Handler) refreshTGDBPlatformsIfStale(ctx context.Context) {
	if h.tgdb == nil || !h.cfg.TGDBConfigured() {
		return
	}
	stale, err := h.store.TGDBPlatformsStale(ctx, 24*time.Hour)
	if err != nil || !stale {
		return
	}
	plats, err := h.tgdb.Platforms(ctx)
	if err != nil {
		h.log.Warn("tgdb platforms", "err", err)
		return
	}
	mapped := make([]store.TGDBPlatform, 0, len(plats))
	for _, p := range plats {
		mapped = append(mapped, store.TGDBPlatform{ID: p.ID, Name: p.Name, Alias: p.Alias})
	}
	if err := h.store.UpsertTGDBPlatforms(ctx, mapped); err != nil {
		h.log.Warn("upsert tgdb platforms", "err", err)
	}
}

func (h *Handler) tgdbPlatformID(ctx context.Context, platform string) *int64 {
	plats, err := h.store.ListTGDBPlatforms(ctx)
	if err != nil {
		plats = nil
	}
	mapped := make([]tgdb.Platform, 0, len(plats))
	for _, p := range plats {
		mapped = append(mapped, tgdb.Platform{ID: p.ID, Name: p.Name, Alias: p.Alias})
	}
	return tgdb.MatchPlatformID(platform, mapped)
}
