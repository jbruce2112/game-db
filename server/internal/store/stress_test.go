package store

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"game-db/internal/model"
)

func TestStressConcurrentSync(t *testing.T) {
	scale := 1
	switch strings.ToLower(strings.TrimSpace(os.Getenv("STRESS"))) {
	case "1", "true", "yes", "extreme":
		scale = 8
	}
	workers := 8 * scale
	ops := 25 * scale
	s := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 28, 20, 0, 0, 0, time.UTC)

	errCh := make(chan error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			defer wg.Done()
			cursor := int64(0)
			for i := 0; i < ops; i++ {
				id := fmt.Sprintf("%08x-dddd-4ddd-8ddd-%012x", w, i)
				it := item(id, fmt.Sprintf("W%d-%d", w, i), now.Add(time.Duration(w*ops+i)*time.Second))
				res, err := s.Sync(ctx, cursor, []model.Item{it}, now.Add(time.Hour))
				if err != nil {
					errCh <- err
					return
				}
				cursor = res.Cursor
			}
			// Fight over one row: highest updated_at should win.
			shared := item("eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", fmt.Sprintf("winner-%d", w), now.Add(time.Duration(w)*time.Minute))
			if _, err := s.Sync(ctx, cursor, []model.Item{shared}, now.Add(time.Hour)); err != nil {
				errCh <- err
			}
		}(w)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}

	got, err := s.List(ctx, ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	want := workers*ops + 1
	if len(got) != want {
		t.Fatalf("list %d want %d", len(got), want)
	}
	var shared model.Item
	for _, it := range got {
		if it.ID == "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee" {
			shared = it
			break
		}
	}
	if shared.ID == "" || !strings.HasPrefix(shared.Title, "winner-") {
		t.Fatalf("shared %+v", shared)
	}
	// Highest worker index used the latest timestamp.
	if shared.Title != fmt.Sprintf("winner-%d", workers-1) {
		t.Fatalf("LWW title %q want winner-%d", shared.Title, workers-1)
	}
}
