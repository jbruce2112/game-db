package syncmerge

import (
	"testing"
	"time"

	"game-db/internal/model"
)

func TestClampUpdatedAt(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	ok := now.Add(time.Minute)
	if got := ClampUpdatedAt(ok, now); !got.Equal(ok) {
		t.Fatalf("within skew: got %v", got)
	}
	future := now.Add(10 * time.Minute)
	if got := ClampUpdatedAt(future, now); !got.Equal(now) {
		t.Fatalf("clamped: got %v want %v", got, now)
	}
}

func TestIncomingWins(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := model.Item{ID: "aaa", UpdatedAt: base}
	newer := model.Item{ID: "aaa", UpdatedAt: base.Add(time.Second)}
	older := model.Item{ID: "aaa", UpdatedAt: base.Add(-time.Second)}
	same := model.Item{ID: "aaa", UpdatedAt: base}

	if !IncomingWins(existing, newer) {
		t.Fatal("newer should win")
	}
	if IncomingWins(existing, older) {
		t.Fatal("older should lose")
	}
	if IncomingWins(existing, same) {
		t.Fatal("equal timestamp + equal id should keep existing")
	}
}
