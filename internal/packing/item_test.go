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
	if err := store.UpdateItem(ctx, list.ID, "user", edited); err != nil {
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
	if err := store.UpdateItem(ctx, list.ID, "user", PackingItem{ID: "missing", Name: "Nothing"}); err == nil {
		t.Fatal("expected an error for an unknown item")
	}
}
