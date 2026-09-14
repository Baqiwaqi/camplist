package packing_test

import (
	"context"
	"errors"
	"slices"
	"strings"
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

// renameFixture stores a camper's own lists, a list someone shared with them
// and a trip they already started, all using the typo "Fishnig".
func renameFixture(t *testing.T) (context.Context, *packing.Store, []packing.PackingList, packing.PackingList, packing.PackingSession) {
	t.Helper()
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	lake := packing.NewList("camper", "Lake", "")
	lake.Items = []packing.PackingItem{packing.NewItem("Rod", "Fishnig"), packing.NewItem("Net", "fishnig"), packing.NewItem("Tent", "Shelter")}
	river := packing.NewList("camper", "River", "")
	river.Items = []packing.PackingItem{packing.NewItem("Lures", "FISHNIG"), packing.NewItem("Bait", "Fishing")}
	dry := packing.NewList("camper", "Desert", "")
	dry.Items = []packing.PackingItem{packing.NewItem("Hat", "Clothing")}
	shared := packing.NewList("friend", "Friend's boat", "")
	shared.Items = []packing.PackingItem{packing.NewItem("Tackle box", "Fishnig")}
	for _, list := range []packing.PackingList{lake, river, dry, shared} {
		if err := store.SavePackingList(ctx, list); err != nil {
			t.Fatal(err)
		}
	}
	link, err := store.CreateInvitation(ctx, "packing-list", shared.ID, "friend")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RequestAccess(ctx, link, packing.Member{Subject: "camper", Name: "Sam"}); err != nil {
		t.Fatal(err)
	}
	if err := store.DecideInvitation(ctx, link.Kind, link.ID, "friend", link.Hash(), true); err != nil {
		t.Fatal(err)
	}
	trip, err := store.CreatePackingSession(ctx, lake.ID, "camper")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RememberCategory(ctx, "camper", "Fishnig"); err != nil {
		t.Fatal(err)
	}
	return ctx, store, []packing.PackingList{lake, river, dry}, shared, trip
}

func itemCategories(t *testing.T, store *packing.Store, listID, userID string) []string {
	t.Helper()
	list, err := store.GetPackingList(context.Background(), listID, userID)
	if err != nil {
		t.Fatal(err)
	}
	var categories []string
	for _, item := range list.Items {
		categories = append(categories, item.Category)
	}
	return categories
}

func TestRenameCategoryFixesOwnListsOnly(t *testing.T) {
	ctx, store, own, shared, trip := renameFixture(t)
	if err := store.RememberCategory(ctx, "camper", "Tarps"); err != nil {
		t.Fatal(err)
	}

	result, err := store.RenameCategory(ctx, "camper", " fishnig ", "Fly fishing")
	if err != nil {
		t.Fatal(err)
	}
	if result.Category != "Fly fishing" || result.Items != 3 || len(result.Lists) != 2 {
		t.Errorf("result = %q on %d items in %d lists", result.Category, result.Items, len(result.Lists))
	}
	for _, renamed := range result.Lists {
		if renamed.List.Revision() == "" || renamed.Replaced == "" || renamed.List.Revision() == renamed.Replaced {
			t.Errorf("list %s revisions %q replaced %q", renamed.List.Name, renamed.List.Revision(), renamed.Replaced)
		}
	}
	if got := itemCategories(t, store, own[0].ID, "camper"); !slices.Equal(got, []string{"Fly fishing", "Fly fishing", "Shelter"}) {
		t.Errorf("lake categories = %q", got)
	}
	if got := itemCategories(t, store, own[1].ID, "camper"); !slices.Equal(got, []string{"Fly fishing", "Fishing"}) {
		t.Errorf("river categories = %q", got)
	}
	if got := itemCategories(t, store, shared.ID, "friend"); !slices.Equal(got, []string{"Fishnig"}) {
		t.Errorf("shared-in list changed to %q", got)
	}
	started, err := store.GetPackingSession(ctx, trip.ID, "camper")
	if err != nil {
		t.Fatal(err)
	}
	if started.List.Items[0].Category != "Fishnig" || started.List.Items[1].Category != "fishnig" {
		t.Errorf("started trip changed to %q, %q", started.List.Items[0].Category, started.List.Items[1].Category)
	}
	if got, err := store.RememberedCategories(ctx, "camper"); err != nil || !slices.Equal(got, []string{"Fly fishing", "Fishing", "Tarps"}) {
		t.Errorf("remembered %q, %v", got, err)
	}
	// A trip started after the rename copies the fixed list.
	next, err := store.CreatePackingSession(ctx, own[0].ID, "camper")
	if err != nil || next.List.Items[0].Category != "Fly fishing" {
		t.Errorf("next trip category %q, %v", next.List.Items[0].Category, err)
	}
}

func TestRenameCategoryMergesIntoExistingCategory(t *testing.T) {
	ctx, store, own, _, _ := renameFixture(t)
	if err := store.RememberCategory(ctx, "camper", "Fishing"); err != nil {
		t.Fatal(err)
	}
	result, err := store.RenameCategory(ctx, "camper", "Fishnig", " FISHING")
	if err != nil {
		t.Fatal(err)
	}
	if result.Category != "Fishing" || result.Items != 3 {
		t.Errorf("merged into %q on %d items", result.Category, result.Items)
	}
	if got := itemCategories(t, store, own[1].ID, "camper"); !slices.Equal(got, []string{"Fishing", "Fishing"}) {
		t.Errorf("river categories = %q", got)
	}
	if got, err := store.RememberedCategories(ctx, "camper"); err != nil || !slices.Equal(got, []string{"Fishing"}) {
		t.Errorf("remembered %q, %v", got, err)
	}

	// Renaming into a default adopts its spelling and forgets the custom one.
	if err := store.RememberCategory(ctx, "camper", "Shleter"); err != nil {
		t.Fatal(err)
	}
	result, err = store.RenameCategory(ctx, "camper", "Shleter", "shelter")
	if err != nil || result.Category != "Shelter" {
		t.Errorf("merged into default %q, %v", result.Category, err)
	}
	if got, err := store.RememberedCategories(ctx, "camper"); err != nil || !slices.Equal(got, []string{"Fishing"}) {
		t.Errorf("remembered after default merge %q, %v", got, err)
	}
}

func TestRenameCategoryMergesIntoACategoryOnlyOnItems(t *testing.T) {
	ctx, store, own, _, _ := renameFixture(t)
	if err := store.ForgetCategory(ctx, "camper", "Fishing"); err != nil {
		t.Fatal(err)
	}
	result, err := store.RenameCategory(ctx, "camper", "Fishnig", "fishing")
	if err != nil {
		t.Fatal(err)
	}
	if result.Category != "Fishing" {
		t.Errorf("merged into %q", result.Category)
	}
	if got := itemCategories(t, store, own[1].ID, "camper"); !slices.Equal(got, []string{"Fishing", "Fishing"}) {
		t.Errorf("river categories = %q", got)
	}
	if got := itemCategories(t, store, own[0].ID, "camper"); !slices.Equal(got, []string{"Fishing", "Fishing", "Shelter"}) {
		t.Errorf("lake categories = %q", got)
	}
	if got, err := store.RememberedCategories(ctx, "camper"); err != nil || !slices.Equal(got, []string{"Fishing"}) {
		t.Errorf("remembered %q, %v", got, err)
	}
}

func TestRenameAndForgetRefuseDefaultsAndUnknownCategories(t *testing.T) {
	ctx, store, _, _, _ := renameFixture(t)
	for _, test := range []struct{ from, to string }{{"Shelter", "Tents"}, {" kitchen and cooking", "Cooking"}, {"Fishnig", "  "}, {"", "Fishing"}, {"Fishnig", strings.Repeat("x", 101)}} {
		if _, err := store.RenameCategory(ctx, "camper", test.from, test.to); !errors.Is(err, packing.ErrInvalid) {
			t.Errorf("rename %q to %q: %v, want ErrInvalid", test.from, test.to, err)
		}
	}
	if _, err := store.RenameCategory(ctx, "camper", "Paddling", "Kayaking"); !errors.Is(err, packing.ErrNotFound) {
		t.Errorf("rename unknown: %v, want ErrNotFound", err)
	}
	if _, err := store.RenameCategory(ctx, "stranger", "Fishnig", "Fishing"); !errors.Is(err, packing.ErrNotFound) {
		t.Errorf("a camper without the category renamed it: %v", err)
	}
	for _, category := range []string{"Shelter", "OTHER", ""} {
		if err := store.ForgetCategory(ctx, "camper", category); !errors.Is(err, packing.ErrInvalid) {
			t.Errorf("forget %q: %v, want ErrInvalid", category, err)
		}
	}
}

func TestForgetCategoryLeavesItemsUnchanged(t *testing.T) {
	ctx, store, own, _, _ := renameFixture(t)
	if err := store.ForgetCategory(ctx, "camper", "FISHNIG"); err != nil {
		t.Fatal(err)
	}
	if got, err := store.RememberedCategories(ctx, "camper"); err != nil || !slices.Equal(got, []string{"Fishing"}) {
		t.Errorf("remembered %q, %v", got, err)
	}
	if got := itemCategories(t, store, own[0].ID, "camper"); !slices.Equal(got, []string{"Fishnig", "fishnig", "Shelter"}) {
		t.Errorf("forgetting changed items to %q", got)
	}
	if err := store.ForgetCategory(ctx, "camper", "Fishnig"); err != nil {
		t.Errorf("forgetting twice: %v", err)
	}
}
