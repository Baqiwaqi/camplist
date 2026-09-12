package packing

import (
	"context"
	"errors"
	"testing"
)

func TestSyncRetriesDoNotReverseNewerChanges(t *testing.T) {
	ctx := context.Background()
	s, list, ses := seedTrip(t)
	first := PackingOperation{ID: "device-a-1", ItemID: list.Items[0].ID, Checked: true, ExpectedRevision: 0}
	packed, err := s.SyncSessionItem(ctx, ses.ID, "camper", first)
	if err != nil {
		t.Fatal(err)
	}
	if packed.List.Items[0].Revision != 1 || !packed.List.Items[0].Checked {
		t.Fatal("first operation not applied")
	}
	_, err = s.SyncSessionItem(ctx, ses.ID, "camper", PackingOperation{ID: "device-b-1", ItemID: first.ItemID, Checked: false, ExpectedRevision: 1})
	if err != nil {
		t.Fatal(err)
	}
	replay, err := s.SyncSessionItem(ctx, ses.ID, "camper", first)
	if err != nil {
		t.Fatal(err)
	}
	if replay.List.Items[0].Checked || replay.List.Items[0].Revision != 2 {
		t.Fatal("lost acknowledgement retry overwrote newer intent")
	}
	_, err = s.SyncSessionItem(ctx, ses.ID, "camper", PackingOperation{ID: "stale", ItemID: first.ItemID, Checked: true, ExpectedRevision: 0})
	if !errors.Is(err, ErrConflict) {
		t.Fatal("stale operation silently accepted")
	}
	_, err = s.SyncSessionItem(ctx, ses.ID, "other", first)
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("another owner changed session")
	}
}

func TestOnlinePackingAlsoAdvancesOfflineRevision(t *testing.T) {
	ctx := context.Background()
	s, list, ses := seedTrip(t)
	got, err := s.SetSessionItem(ctx, ses.ID, "camper", list.Items[0].ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.List.Items[0].Revision != 1 {
		t.Fatal("online change did not advance revision")
	}
	_, err = s.SyncSessionItem(ctx, ses.ID, "camper", PackingOperation{ID: "offline-old", ItemID: list.Items[0].ID, Checked: false, ExpectedRevision: 0})
	if !errors.Is(err, ErrConflict) {
		t.Fatal("offline change erased online change")
	}
}
