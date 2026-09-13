package packing_test

import (
	"context"
	"slices"
	"testing"

	"camplist/internal/packing"
	"camplist/internal/testsupport"
)

func TestCategorySuggestionsStartWithDefaultsAndDropDuplicates(t *testing.T) {
	got := packing.CategorySuggestions([]string{" shelter ", "Tarps", "", "tarps"}, []string{" Fishing", "TARPS"})
	want := append(slices.Clone(packing.DefaultCategories), "Tarps", "Fishing")
	if !slices.Equal(got, want) {
		t.Errorf("suggestions = %q, want %q", got, want)
	}
}

func TestMatchCategoryAdoptsDefaultSpellingOnly(t *testing.T) {
	for typed, want := range map[string]string{" kitchen AND cooking ": "Kitchen and cooking", " tarps": "tarps", " FISHING ": "FISHING", "": ""} {
		if got := packing.MatchCategory(typed); got != want {
			t.Errorf("MatchCategory(%q) = %q, want %q", typed, got, want)
		}
	}
}

func TestItemCategoriesKeepFirstUse(t *testing.T) {
	items := []packing.PackingItem{packing.NewItem("Tent", "Shelter"), packing.NewItem("Lamp", ""), packing.NewItem("Tarp", "shelter"), packing.NewItem("Rod", "Fishing")}
	if got := packing.ItemCategories(items); !slices.Equal(got, []string{"Shelter", "Fishing"}) {
		t.Errorf("item categories = %q", got)
	}
}

func TestRememberedCategoriesSeedFromOwnListsUntilOneIsSaved(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	own := packing.NewList("camper", "Weekend", "")
	own.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter"), packing.NewItem("Rod", "Fishing")}
	other := packing.NewList("someone", "Theirs", "")
	other.Items = []packing.PackingItem{packing.NewItem("Kayak", "Paddling")}
	for _, list := range []packing.PackingList{own, other} {
		if err := store.SavePackingList(ctx, list); err != nil {
			t.Fatal(err)
		}
	}

	got, err := store.RememberedCategories(ctx, "camper")
	if err != nil || !slices.Equal(got, []string{"Fishing"}) {
		t.Fatalf("seeded categories = %q, %v", got, err)
	}

	for _, category := range []string{" Tarps ", "Tarps", "Kitchen and cooking", "", "FISHING"} {
		if err := store.RememberCategory(ctx, "camper", category); err != nil {
			t.Fatalf("remember %q: %v", category, err)
		}
	}
	// A list deleted later no longer seeds, but what was remembered stays.
	if err := store.DeletePackingList(ctx, own.ID, "camper"); err != nil {
		t.Fatal(err)
	}
	got, err = store.RememberedCategories(ctx, "camper")
	if err != nil || !slices.Equal(got, []string{"FISHING", "Tarps"}) {
		t.Errorf("remembered categories = %q, %v", got, err)
	}
	if got, err := store.RememberedCategories(ctx, "someone"); err != nil || !slices.Equal(got, []string{"Paddling"}) {
		t.Errorf("another camper sees %q, %v", got, err)
	}
	// The seed is read once: a list deleted after the first read no longer
	// changes what is offered.
	if err := store.DeletePackingList(ctx, other.ID, "someone"); err != nil {
		t.Fatal(err)
	}
	if got, err := store.RememberedCategories(ctx, "someone"); err != nil || !slices.Equal(got, []string{"Paddling"}) {
		t.Errorf("after deleting the seeded list another camper sees %q, %v", got, err)
	}
	if got, err := store.RememberedCategories(ctx, "newcomer"); err != nil || len(got) != 0 {
		t.Errorf("newcomer remembers %q, %v", got, err)
	}
	if got, err := store.RememberedCategories(ctx, "newcomer"); err != nil || len(got) != 0 {
		t.Errorf("newcomer remembers %q on a second read, %v", got, err)
	}
	lists, err := store.GetPackingLists(ctx, "camper")
	if err != nil || len(lists) != 0 {
		t.Errorf("remembered categories leaked into the list overview: %+v, %v", lists, err)
	}
}
