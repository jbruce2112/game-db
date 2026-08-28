package syncmerge

import (
	"time"

	"game-db/internal/model"
)

const FutureSkew = 5 * time.Minute

// ClampUpdatedAt rewrites timestamps unreasonably in the future so a wrong
// client clock cannot shadow every other device.
func ClampUpdatedAt(t, now time.Time) time.Time {
	now = model.TimeUTC(now)
	t = model.TimeUTC(t)
	if t.After(now.Add(FutureSkew)) {
		return now
	}
	return t
}

// IncomingWins is last-write-wins on updated_at. Equal timestamps keep the
// existing row (idempotent retries). The UUID comparison is only meaningful
// if two different records were compared; for the same id it is a no-op.
func IncomingWins(existing, incoming model.Item) bool {
	iu := incoming.UpdatedAt.UTC()
	eu := existing.UpdatedAt.UTC()
	if iu.After(eu) {
		return true
	}
	if iu.Before(eu) {
		return false
	}
	return incoming.ID > existing.ID
}
