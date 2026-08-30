package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"game-db/internal/model"

	_ "modernc.org/sqlite"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func item(id, title string, updated time.Time) model.Item {
	return model.Item{
		ID:           id,
		Title:        title,
		Platform:     "Nintendo 64",
		Completeness: "cib",
		Notes:        "",
		CreatedAt:    updated,
		UpdatedAt:    updated,
	}
}

func TestMigrateAddsBarcodeColumn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "game-db.sqlite")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE library_items (
			id TEXT PRIMARY KEY NOT NULL,
			title TEXT NOT NULL,
			platform TEXT NOT NULL,
			igdb_platform_id INTEGER,
			region TEXT,
			completeness TEXT NOT NULL DEFAULT 'unknown',
			notes TEXT NOT NULL DEFAULT '',
			igdb_game_id INTEGER,
			cover_id TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			deleted_at TEXT,
			sync_seq INTEGER NOT NULL DEFAULT 0
		)`)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	code := "045496590376"
	it := item("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Zelda", time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC))
	it.Barcode = &code
	if _, err := s.Insert(context.Background(), it); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(context.Background(), it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Barcode == nil || *got.Barcode != code {
		t.Fatalf("%+v", got)
	}
}

func TestSyncInsertAndPull(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	a := item("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Zelda", now)

	res, err := s.Sync(ctx, 0, []model.Item{a}, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Cursor != 1 {
		t.Fatalf("cursor %d", res.Cursor)
	}
	if len(res.Changes) != 1 || res.Changes[0].Title != "Zelda" {
		t.Fatalf("changes %+v", res.Changes)
	}

	res, err = s.Sync(ctx, 1, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 0 {
		t.Fatalf("expected empty pull, got %+v", res.Changes)
	}
}

func TestSyncLastWriteWins(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	t1 := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	id := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	if _, err := s.Sync(ctx, 0, []model.Item{item(id, "Old", t1)}, t1); err != nil {
		t.Fatal(err)
	}

	t2 := t1.Add(time.Minute)
	res, err := s.Sync(ctx, 1, []model.Item{item(id, "New", t2)}, t2)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 1 || res.Changes[0].Title != "New" {
		t.Fatalf("winner %+v", res.Changes)
	}

	t0 := t1.Add(-time.Minute)
	res, err = s.Sync(ctx, 2, []model.Item{item(id, "Stale", t0)}, t2)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range res.Changes {
		if c.ID == id {
			found = true
			if c.Title != "New" {
				t.Fatalf("stale write won: %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("losing client should receive the current winner")
	}
}

func TestSyncTombstone(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	id := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	if _, err := s.Sync(ctx, 0, []model.Item{item(id, "Gone", now)}, now); err != nil {
		t.Fatal(err)
	}
	later := now.Add(time.Second)
	del := item(id, "Gone", later)
	del.DeletedAt = &later
	res, err := s.Sync(ctx, 1, []model.Item{del}, later)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Changes) != 1 || res.Changes[0].DeletedAt == nil {
		t.Fatalf("tombstone %+v", res.Changes)
	}
	list, err := s.List(ctx, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("list should hide tombstones, got %d", len(list))
	}
}

func TestSyncClockClamp(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	far := item("dddddddd-dddd-dddd-dddd-dddddddddddd", "Future", now.Add(10*time.Minute))
	res, err := s.Sync(ctx, 0, []model.Item{far}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changes[0].UpdatedAt.Equal(now) {
		t.Fatalf("wanted clamp to now, got %v", res.Changes[0].UpdatedAt)
	}
}

func TestCRUD(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	it, err := s.Insert(ctx, item("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", "Mario", now))
	if err != nil {
		t.Fatal(err)
	}
	if it.SyncSeq != 1 {
		t.Fatalf("seq %d", it.SyncSeq)
	}
	it.Title = "Mario 64"
	it.UpdatedAt = now.Add(time.Second)
	it, err = s.Replace(ctx, it)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Mario 64" {
		t.Fatalf("got %s", got.Title)
	}
}

func TestAttachCoverURLFallsBackToOnDemand(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	igdbID := int64(42)
	it := item("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Zelda", now)
	it.IGDBGameID = &igdbID
	got, err := s.Insert(ctx, it)
	if err != nil {
		t.Fatal(err)
	}
	if got.CoverURL == nil || *got.CoverURL != "/v1/library/"+it.ID+"/cover" {
		t.Fatalf("cover url %+v", got.CoverURL)
	}

	updatedAt := got.UpdatedAt
	seq := got.SyncSeq
	coverID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	if err := s.SetCoverID(ctx, it.ID, coverID); err != nil {
		t.Fatal(err)
	}
	after, err := s.Get(ctx, it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.CoverID == nil || *after.CoverID != coverID {
		t.Fatalf("cover id %+v", after.CoverID)
	}
	if !after.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("updated_at changed %s -> %s", updatedAt, after.UpdatedAt)
	}
	if after.SyncSeq <= seq {
		t.Fatalf("sync_seq %d -> %d", seq, after.SyncSeq)
	}
	if after.CoverURL == nil || *after.CoverURL != "/v1/library/"+it.ID+"/cover" {
		t.Fatalf("still on-demand until file exists, got %+v", after.CoverURL)
	}
}
