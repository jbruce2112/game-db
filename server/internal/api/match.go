package api

import (
	"context"

	"game-db/internal/igdb"
	"game-db/internal/model"
	"game-db/internal/store"
)

func toIGDBPlatforms(plats []store.Platform) []igdb.Platform {
	out := make([]igdb.Platform, 0, len(plats))
	for _, p := range plats {
		out = append(out, igdb.Platform{
			ID: p.ID, Name: p.Name, Slug: p.Slug, Abbreviation: p.Abbreviation,
		})
	}
	return out
}

func (h *Handler) matchLibraryItem(ctx context.Context, item *model.Item, plats []igdb.Platform) bool {
	if h.igdb == nil || item == nil || item.Title == "" {
		return false
	}
	if item.IGDBPlatformID == nil {
		item.IGDBPlatformID = igdb.MatchPlatformID(item.Platform, plats)
	}
	var pid int64
	if item.IGDBPlatformID != nil {
		pid = *item.IGDBPlatformID
	}
	games, err := h.igdb.SearchGames(ctx, item.Title, pid)
	if err != nil {
		h.log.Warn("igdb match search", "title", item.Title, "err", err)
		return false
	}
	if len(games) == 0 && pid > 0 {
		games, err = h.igdb.SearchGames(ctx, item.Title, 0)
		if err != nil {
			h.log.Warn("igdb match search all", "title", item.Title, "err", err)
			return false
		}
	}
	hit := igdb.PickGame(item.Title, pid, games)
	if hit == nil {
		return false
	}
	item.IGDBGameID = &hit.ID
	if err := h.store.LinkIGDB(ctx, item.ID, hit.ID, item.IGDBPlatformID); err != nil {
		h.log.Warn("igdb match save", "id", item.ID, "err", err)
		return false
	}
	if hit.CoverImageID != "" {
		if err := h.cacheCover(ctx, item, hit.CoverImageID); err != nil {
			h.log.Warn("igdb match cover", "id", item.ID, "err", err)
		} else if item.CoverID != nil {
			if err := h.store.SetCoverID(ctx, item.ID, *item.CoverID); err != nil {
				h.log.Warn("igdb match cover id", "id", item.ID, "err", err)
			}
		}
	}
	return true
}
