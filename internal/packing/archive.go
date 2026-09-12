package packing

import (
	"sort"
	"time"
)

const archiveDelay = 24 * time.Hour

// IsArchived reports whether a fully packed trip has been complete long enough
// to leave the active trips overview. The latest item update is when the trip
// reached its current fully packed state.
func (s PackingSession) IsArchived(now time.Time) bool {
	if len(s.List.Items) == 0 {
		return false
	}

	packedAt := s.CreatedAt
	for _, item := range s.List.Items {
		if !item.Checked {
			return false
		}
		updatedAt := item.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = item.CreatedAt
		}
		if updatedAt.After(packedAt) {
			packedAt = updatedAt
		}
	}

	return !packedAt.Add(archiveDelay).After(now)
}

// PartitionSessions separates current and archived trips and keeps both
// sections predictable, with the newest trip first.
func PartitionSessions(sessions []PackingSession, now time.Time) (active, archived []PackingSession) {
	for _, session := range sessions {
		if session.IsArchived(now) {
			archived = append(archived, session)
		} else {
			active = append(active, session)
		}
	}
	newestFirst := func(sessions []PackingSession) {
		sort.SliceStable(sessions, func(i, j int) bool {
			return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
		})
	}
	newestFirst(active)
	newestFirst(archived)
	return active, archived
}
