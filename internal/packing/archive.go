package packing

import (
	"context"
	"sort"
	"time"
)

const archiveDelay = 24 * time.Hour

// IsArchived reports whether a trip belongs in the archive: the owner archived
// it, or it has been fully packed long enough to leave the trips overview.
func (s PackingSession) IsArchived(now time.Time) bool {
	return s.ArchivedAt != nil || s.packedLongAgo(now)
}

// CanRestore reports whether Restore would bring the trip back to the trips
// overview. Only a manual archive can be undone; a trip that is also packed
// long enough would stay in the archive anyway.
func (s PackingSession) CanRestore(now time.Time) bool {
	return s.ArchivedAt != nil && !s.packedLongAgo(now)
}

// packedLongAgo reports whether a fully packed trip has been complete for the
// archive delay. The latest item update is when the trip reached its current
// fully packed state.
func (s PackingSession) packedLongAgo(now time.Time) bool {
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

// ArchiveTrip moves a trip to the archive for everyone who shares it. Only the
// owner can archive, and the write is conditioned on the checked ETag.
func (s *Store) ArchiveTrip(ctx context.Context, id, actor string) (PackingSession, error) {
	return s.setTripArchived(ctx, id, actor, true)
}

// RestoreTrip undoes a manual archive. Only the owner can restore.
func (s *Store) RestoreTrip(ctx context.Context, id, actor string) (PackingSession, error) {
	return s.setTripArchived(ctx, id, actor, false)
}

func (s *Store) setTripArchived(ctx context.Context, id, actor string, archived bool) (PackingSession, error) {
	return s.updateSession(ctx, id, actor, func(trip *PackingSession) (bool, error) {
		if trip.UserID != actor {
			return false, ErrForbidden
		}
		if (trip.ArchivedAt != nil) == archived {
			return false, nil
		}
		if archived {
			at := s.clock().UTC()
			trip.ArchivedAt = &at
		} else {
			trip.ArchivedAt = nil
		}
		return true, nil
	})
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
