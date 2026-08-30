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
	hit := h.searchPick(ctx, item.Title, pid)
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

func (h *Handler) searchPick(ctx context.Context, title string, pid int64) *igdb.Game {
	seen := map[int64]struct{}{}
	var pool []igdb.Game
	add := func(q string, plat int64, contains bool) *igdb.Game {
		var games []igdb.Game
		var err error
		if contains {
			games, err = h.igdb.SearchGamesContains(ctx, q, plat)
		} else {
			games, err = h.igdb.SearchGames(ctx, q, plat)
		}
		if err != nil {
			h.log.Warn("igdb match search", "title", title, "q", q, "err", err)
			return igdb.PickGame(title, pid, pool)
		}
		for _, g := range games {
			if _, ok := seen[g.ID]; ok {
				continue
			}
			seen[g.ID] = struct{}{}
			pool = append(pool, g)
		}
		return igdb.PickGame(title, pid, pool)
	}

	queries := igdb.SearchTitles(title)
	for _, q := range queries {
		if hit := add(q, pid, false); hit != nil {
			return hit
		}
	}
	if pid > 0 {
		limit := 3
		if len(queries) < limit {
			limit = len(queries)
		}
		for _, q := range queries[:limit] {
			if hit := add(q, 0, false); hit != nil {
				return hit
			}
		}
	}
	contains := make([]string, 0, 4)
	if d, c := igdb.LeadingAcronym(title); c != "" {
		contains = append(contains, d, c)
	}
	if len(queries) > 1 {
		contains = append(contains, queries[1])
	}
	contains = append(contains, queries[0])
	for _, q := range contains {
		if q == "" {
			continue
		}
		if hit := add(q, pid, true); hit != nil {
			return hit
		}
	}
	if pid > 0 {
		for _, q := range contains {
			if q == "" {
				continue
			}
			if hit := add(q, 0, true); hit != nil {
				return hit
			}
		}
	}
	return nil
}
