package packing

import (
	"context"
	"testing"

	"camplist/internal/testsupport"
)

func TestUpdateItemChangesNameAndCategory(t *testing.T) {
	store := NewStore(testsupport.NewDocuments())
	ctx := context.Background()
	list := NewList("user", "Camping", "")
	list.Items = []PackingItem{NewItem("Tent", "Shelter"), NewItem("Stove", "Kitchen")}
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}

	edited := PackingItem{ID: list.Items[1].ID, Name: "Gas stove", Category: "Cooking"}
	if _, _, err := store.UpdateItem(ctx, list.ID, "user", edited); err != nil {
		t.Fatal(err)
	}

	saved, err := store.GetPackingList(ctx, list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	item, ok := saved.FindItem(edited.ID)
	if !ok || item.Name != "Gas stove" || item.Category != "Cooking" {
		t.Fatalf("item not updated: %+v", item)
	}
	if first, _ := saved.FindItem(list.Items[0].ID); first.Name != "Tent" || first.Category != "Shelter" {
		t.Fatalf("untouched item changed: %+v", first)
	}
	if _, _, err := store.UpdateItem(ctx, list.ID, "user", PackingItem{ID: "missing", Name: "Nothing"}); err == nil {
		t.Fatal("expected an error for an unknown item")
	}
}

// Item writes return the saved list. Private lists never conflict: a stale
// revision still saves. Adds report the revision they replaced.
func TestItemWritesReturnTheSavedRevision(t *testing.T) {
	ctx := context.Background()
	store := NewStore(testsupport.NewDocuments())
	list := NewList("user", "Weekend", "")
	list.Items = []PackingItem{NewItem("Tent", ""), NewItem("Stove", "")}
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	opened, _ := store.GetPackingList(ctx, list.ID, "user")

	added, replaced, err := store.AddItem(ctx, list.ID, "user", NewItem("Socks", ""))
	if err != nil || replaced != opened.Revision() || len(added.Items) != 3 || added.Revision() == opened.Revision() || added.Items[0].SourceRevision != added.Revision() {
		t.Fatalf("add: %v replaced=%q items=%d", err, replaced, len(added.Items))
	}
	if current, _ := store.GetPackingList(ctx, list.ID, "user"); current.Revision() != added.Revision() {
		t.Fatalf("add returned revision %q, stored %q", added.Revision(), current.Revision())
	}

	removed, replaced, err := store.RemoveItem(ctx, list.ID, "user", list.Items[0].ID, opened.Revision())
	if err != nil || replaced != added.Revision() || len(removed.Items) != 2 || removed.Revision() == added.Revision() {
		t.Fatalf("remove from a stale revision: %v %+v", err, removed.Items)
	}

	stale := removed.Items[0]
	stale.Name = "Gas stove"
	stale.SourceRevision = added.Revision()
	updated, replaced, err := store.UpdateItem(ctx, list.ID, "user", stale)
	if err != nil || replaced != removed.Revision() {
		t.Fatalf("update from a stale revision: %v", err)
	}
	if item, _ := updated.FindItem(stale.ID); item.Name != "Gas stove" {
		t.Fatalf("update from a stale revision not saved: %+v", updated.Items)
	}
	if _, _, err := store.RemoveItem(ctx, list.ID, "user", list.Items[1].ID, ""); err != nil {
		t.Fatalf("remove without a revision: %v", err)
	}
}
