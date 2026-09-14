package packing_test

import (
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"errors"
	"testing"
)

func TestTripPersonalCopiesAndPreparationReset(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Camping", "")
	sleeping := packing.NewItem("Sleeping bag", "Sleep")
	sleeping.Scope = "person"
	list.Items = []packing.PackingItem{sleeping, packing.NewItem("Tent", "Sleep")}
	list.Tasks = []packing.PreparationTask{{ID: "charge", Name: "Charge phone", Done: true, Scope: "person"}}
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	trip, err := store.CreatePackingSession(ctx, list.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if trip.Name == "" || trip.List.Name != "Camping" {
		t.Fatalf("trip name: %q", trip.Name)
	}
	if len(trip.List.Items) != 2 || trip.List.Items[0].Assignee != "owner" || trip.List.Tasks[0].Done {
		t.Fatalf("initial snapshot: %+v", trip.List)
	}
	if _, _, err = store.AddItem(ctx, list.ID, "owner", packing.NewItem("Added only to later template", "")); err != nil {
		t.Fatal(err)
	}
	link, err := store.CreateInvitation(ctx, "packing-session", trip.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RequestAccess(ctx, link, packing.Member{Subject: "guest", Name: "Guest"}); err != nil {
		t.Fatal(err)
	}
	if err = store.DecideInvitation(ctx, link.Kind, trip.ID, "owner", link.Hash(), true); err != nil {
		t.Fatal(err)
	}
	trip, err = store.GetPackingSession(ctx, trip.ID, "guest")
	if err != nil {
		t.Fatal(err)
	}
	if len(trip.List.Items) != 3 || len(trip.List.Tasks) != 2 {
		t.Fatalf("copies: %d items %d tasks", len(trip.List.Items), len(trip.List.Tasks))
	}
	_, err = store.SyncSessionItem(ctx, trip.ID, "guest", packing.PackingOperation{ID: "forbidden", ItemID: trip.List.Items[0].ID, Checked: true})
	if !errors.Is(err, packing.ErrForbidden) {
		t.Fatalf("other personal item: %v", err)
	}
	_, err = store.SetSessionPreparationTask(ctx, trip.ID, "guest", "charge", true, 0)
	if !errors.Is(err, packing.ErrForbidden) {
		t.Fatalf("other personal task: %v", err)
	}
}

func TestTripAdditionsAreIdempotentAndRemainSeparateFromTemplate(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Camping", "")
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	trip, err := store.CreatePackingSession(ctx, list.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	op := packing.PackingOperation{ID: "add-bag", ItemID: "new-bag", Action: "add", Name: "Sleeping bag", Category: "Sleep", Scope: "person"}
	trip, err = store.SyncSessionItem(ctx, trip.ID, "owner", op)
	if err != nil {
		t.Fatal(err)
	}
	trip, err = store.SyncSessionItem(ctx, trip.ID, "owner", op)
	if err != nil {
		t.Fatal(err)
	}
	if len(trip.List.Items) != 1 || trip.List.Items[0].Assignee != "owner" {
		t.Fatalf("addition %+v", trip.List.Items)
	}
	task := packing.PackingOperation{ID: "add-task", ItemID: "new-task", Action: "add", Kind: "task", Name: "Charge car"}
	trip, err = store.SyncSessionItem(ctx, trip.ID, "owner", task)
	if err != nil {
		t.Fatal(err)
	}
	trip, err = store.SyncSessionItem(ctx, trip.ID, "owner", packing.PackingOperation{ID: "done-task", ItemID: "new-task", Kind: "task", Checked: true})
	if err != nil {
		t.Fatal(err)
	}
	if !trip.List.Tasks[0].Done {
		t.Fatal("task not done")
	}
	template, err := store.GetPackingList(ctx, list.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if len(template.Items) != 0 || len(template.Tasks) != 0 {
		t.Fatal("trip addition changed template")
	}
}

func TestFutureSaveFailurePreservesTripAndCanBeRetriedAfterIndependentGrant(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Camping", "")
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	trip, err := store.CreatePackingSession(ctx, list.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	grant := func(kind, id string) {
		t.Helper()
		link, err := store.CreateInvitation(ctx, kind, id, "owner")
		if err != nil {
			t.Fatal(err)
		}
		if err = store.RequestAccess(ctx, link, packing.Member{Subject: "guest", Name: "Guest"}); err != nil {
			t.Fatal(err)
		}
		if err = store.DecideInvitation(ctx, kind, id, "owner", link.Hash(), true); err != nil {
			t.Fatal(err)
		}
	}
	grant("packing-session", trip.ID)
	op := packing.PackingOperation{ID: "new", ItemID: "bag", Action: "add", Name: "Bag", Scope: "mine", SaveForFuture: true}
	saved, err := store.SyncSessionItem(ctx, trip.ID, "guest", op)
	if !errors.Is(err, packing.ErrFutureSave) || len(saved.List.Items) != 1 {
		t.Fatalf("partial save: %v %+v", err, saved)
	}
	grant("packing-list", list.ID)
	if _, err = store.SyncSessionItem(ctx, trip.ID, "guest", op); err != nil {
		t.Fatal(err)
	}
	if _, err = store.SyncSessionItem(ctx, trip.ID, "guest", op); err != nil {
		t.Fatal(err)
	}
	template, err := store.GetPackingList(ctx, list.ID, "guest")
	if err != nil {
		t.Fatal(err)
	}
	if len(template.Items) != 1 || template.Items[0].Scope != "person" {
		t.Fatalf("future item %+v", template.Items)
	}
}

func TestPreparationFallbackUpdatesAttributionAfterAnotherParticipant(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Camping", "")
	list.Tasks = []packing.PreparationTask{{ID: "car", Name: "Charge car"}}
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	trip, err := store.CreatePackingSession(ctx, list.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	link, err := store.CreateInvitation(ctx, "packing-session", trip.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RequestAccess(ctx, link, packing.Member{Subject: "guest", Name: "Guest"}); err != nil {
		t.Fatal(err)
	}
	if err = store.DecideInvitation(ctx, link.Kind, trip.ID, "owner", link.Hash(), true); err != nil {
		t.Fatal(err)
	}
	trip, err = store.SyncSessionItem(ctx, trip.ID, "guest", packing.PackingOperation{ID: "guestdone", Kind: "task", ItemID: "car", Checked: true})
	if err != nil {
		t.Fatal(err)
	}
	trip, err = store.SetSessionPreparationTask(ctx, trip.ID, "owner", "car", false, trip.List.Tasks[0].Revision)
	if err != nil {
		t.Fatal(err)
	}
	if trip.List.Tasks[0].ChangedBy != "Trip owner" {
		t.Fatalf("stale attribution %s", trip.List.Tasks[0].ChangedBy)
	}
	for _, item := range trip.Snapshot("guest").List.Items {
		if item.ID == "car" && item.ChangedByID != "owner" {
			t.Fatalf("snapshot should name the author's account so devices can hide their own changes, got %q", item.ChangedByID)
		}
	}
}
