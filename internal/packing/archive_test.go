package packing

import (
	"testing"
	"time"
)

func sessionCompletedAt(completedAt time.Time) PackingSession {
	list := NewList("camper", "Camping", "")
	list.Items = []PackingItem{
		NewItem("Tent", "Shelter"),
		NewItem("Stove", "Kitchen"),
	}
	session := NewPackingSession(list)
	session.CreatedAt = completedAt.Add(-24 * time.Hour)
	for i := range session.List.Items {
		session.List.Items[i].Checked = true
		session.List.Items[i].UpdatedAt = completedAt.Add(-time.Duration(i) * time.Hour)
	}
	return session
}

func TestCompletedSessionArchivesAfterOneDay(t *testing.T) {
	completedAt := time.Date(2026, time.September, 10, 12, 0, 0, 0, time.UTC)
	session := sessionCompletedAt(completedAt)

	if session.IsArchived(completedAt.Add(24*time.Hour - time.Second)) {
		t.Fatal("session archived before a full day passed")
	}
	if !session.IsArchived(completedAt.Add(24 * time.Hour)) {
		t.Fatal("session remained active after a full day")
	}
}

func TestIncompleteAndEmptySessionsRemainActive(t *testing.T) {
	now := time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC)
	incomplete := sessionCompletedAt(now.Add(-48 * time.Hour))
	incomplete.List.Items[0].Checked = false
	empty := NewPackingSession(NewList("camper", "Empty", ""))

	for _, session := range []PackingSession{incomplete, empty} {
		if session.IsArchived(now) {
			t.Fatalf("session %q should remain active", session.DisplayName())
		}
	}
}

func TestPartitionSessionsSeparatesArchiveAndSortsNewestFirst(t *testing.T) {
	now := time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC)
	archived := sessionCompletedAt(now.Add(-48 * time.Hour))
	archived.Name = "Archived"
	activeOlder := sessionCompletedAt(now.Add(-12 * time.Hour))
	activeOlder.Name = "Active older"
	activeOlder.CreatedAt = now.Add(-3 * time.Hour)
	activeNewer := sessionCompletedAt(now.Add(-6 * time.Hour))
	activeNewer.Name = "Active newer"
	activeNewer.CreatedAt = now.Add(-time.Hour)

	active, archive := PartitionSessions([]PackingSession{activeOlder, archived, activeNewer}, now)
	if len(active) != 2 || active[0].ID != activeNewer.ID || active[1].ID != activeOlder.ID {
		t.Fatalf("unexpected active sessions: %+v", active)
	}
	if len(archive) != 1 || archive[0].ID != archived.ID {
		t.Fatalf("unexpected archive: %+v", archive)
	}
}

func TestManualArchiveCanBeRestoredUnlessThePackedTripWouldStayArchived(t *testing.T) {
	now := time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC)
	session := NewPackingSession(NewList("camper", "Camping", ""))
	session.ArchivedAt = &now
	if !session.IsArchived(now) || !session.CanRestore(now) {
		t.Fatal("manually archived trip is not archived and restorable")
	}

	packed := sessionCompletedAt(now.Add(-48 * time.Hour))
	if packed.CanRestore(now) {
		t.Fatal("automatically archived trip offers Restore")
	}
	packed.ArchivedAt = &now
	if packed.CanRestore(now) {
		t.Fatal("restore offered for a trip that would stay archived")
	}
}
