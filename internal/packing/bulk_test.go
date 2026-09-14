package packing_test

import (
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func saveList(t *testing.T, store *packing.Store, list packing.PackingList) {
	t.Helper()
	if err := store.SavePackingList(context.Background(), list); err != nil {
		t.Fatal(err)
	}
}

func itemNames(items []packing.PackingItem) []string {
	names := []string{}
	for _, item := range items {
		names = append(names, item.Name+"/"+item.Category)
	}
	return names
}

func TestAddItemsSkipsNamesAlreadyOnTheListOrRepeated(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("camper", "Climbing", "")
	list.Items = []packing.PackingItem{packing.NewItem("Passport", "Documents")}
	saveList(t, store, list)

	got, err := store.AddItems(ctx, list.ID, "camper", []packing.PackingItem{
		{Name: " Climbing shoes ", Category: "climbing"},
		{Name: "passport", Category: "documents"},
		{Name: "Driving licence", Category: "documents"},
		{Name: "CLIMBING SHOES", Category: "Climbing"},
		{Name: "Tarp", Category: "shelter"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Climbing shoes/climbing", "Driving licence/Documents", "Tarp/Shelter"}; !reflect.DeepEqual(itemNames(got.Added), want) {
		t.Fatalf("added %v, want %v", itemNames(got.Added), want)
	}
	if want := []string{"passport", "CLIMBING SHOES"}; !reflect.DeepEqual(got.Skipped, want) {
		t.Fatalf("skipped %v, want %v", got.Skipped, want)
	}
	saved, err := store.GetPackingList(ctx, list.ID, "camper")
	if err != nil || len(saved.Items) != 4 || saved.Revision() != got.List.Revision() || len(got.List.Items) != 4 {
		t.Fatalf("saved %d items at %q, result list %d at %q: %v", len(saved.Items), saved.Revision(), len(got.List.Items), got.List.Revision(), err)
	}

	again, err := store.AddItems(ctx, list.ID, "camper", []packing.PackingItem{{Name: "Climbing shoes"}, {Name: "Tarp"}})
	if err != nil || len(again.Added) != 0 || len(again.Skipped) != 2 {
		t.Fatalf("resubmitted add: %+v, %v", again, err)
	}
	if unchanged, _ := store.GetPackingList(ctx, list.ID, "camper"); unchanged.Revision() != saved.Revision() {
		t.Fatal("an add that skipped everything still wrote the list")
	}
}

func TestAddItemsRejectsOversizedInput(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("camper", "Weekend", "")
	saveList(t, store, list)

	many := make([]packing.PackingItem, packing.MaxItemsPerAdd+1)
	for i := range many {
		many[i] = packing.PackingItem{Name: fmt.Sprintf("Item %d", i)}
	}
	for name, items := range map[string][]packing.PackingItem{
		"nothing":       nil,
		"blank name":    {{Name: "  "}},
		"long name":     {{Name: strings.Repeat("a", packing.MaxItemNameLength+1)}},
		"long category": {{Name: "Tent", Category: strings.Repeat("c", packing.MaxCategoryLength+1)}},
		"bad scope":     {{Name: "Tent", Scope: "everyone"}},
		"too many":      many,
	} {
		if _, err := store.AddItems(ctx, list.ID, "camper", items); !errors.Is(err, packing.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := store.AddItems(ctx, list.ID, "camper", many[:packing.MaxItemsPerAdd]); err != nil {
		t.Fatalf("a full paste was rejected: %v", err)
	}
	if err := store.AddItem(ctx, list.ID, "camper", packing.NewItem(strings.Repeat("a", packing.MaxItemNameLength+1), "")); !errors.Is(err, packing.ErrInvalid) {
		t.Fatalf("single add accepted a long name: %v", err)
	}
}

func TestAddsStopAtTheListEntryLimit(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("camper", "Expedition", "")
	for i := 0; i < packing.MaxListEntries-2; i++ {
		list.Items = append(list.Items, packing.NewItem(fmt.Sprintf("Item %d", i), ""))
	}
	list.Tasks = []packing.PreparationTask{{ID: "charge", Name: "Charge lamps"}}
	saveList(t, store, list)

	if _, err := store.AddItems(ctx, list.ID, "camper", []packing.PackingItem{{Name: "One"}, {Name: "Two"}}); !errors.Is(err, packing.ErrListFull) {
		t.Fatalf("bulk add past the limit: %v", err)
	}
	// Skipped names do not count towards the limit.
	if got, err := store.AddItems(ctx, list.ID, "camper", []packing.PackingItem{{Name: "Item 1"}, {Name: "One"}}); err != nil || len(got.Added) != 1 {
		t.Fatalf("bulk add up to the limit: %+v, %v", got, err)
	}
	if err := store.AddItem(ctx, list.ID, "camper", packing.NewItem("Two", "")); !errors.Is(err, packing.ErrListFull) {
		t.Fatalf("single add past the limit: %v", err)
	}
	saved, _ := store.GetPackingList(ctx, list.ID, "camper")
	if len(saved.Items)+len(saved.Tasks) != packing.MaxListEntries {
		t.Fatalf("list holds %d entries", len(saved.Items)+len(saved.Tasks))
	}
}

func TestCopyItemsCopiesSelectedItemsFromASharedSource(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	climbing := packing.NewList("owner", "Climbing", "")
	shoes, chalk, pad := packing.NewItem("Climbing shoes", "Climbing"), packing.NewItem("Chalk bag", "Climbing"), packing.NewItem("Crash pad", "Climbing")
	shoes.Scope = "person"
	climbing.Items = []packing.PackingItem{shoes, chalk, pad}
	climbing.Tasks = []packing.PreparationTask{{ID: "tape", Name: "Buy tape"}}
	saveList(t, store, climbing)
	link, _ := store.CreateInvitation(ctx, "packing-list", climbing.ID, "owner")
	if err := store.RequestAccess(ctx, link, packing.Member{Subject: "member"}); err != nil {
		t.Fatal(err)
	}
	if err := store.DecideInvitation(ctx, link.Kind, link.ID, "owner", link.Hash(), true); err != nil {
		t.Fatal(err)
	}
	weekend := packing.NewList("member", "Weekend", "")
	weekend.Items = []packing.PackingItem{packing.NewItem("chalk bag", "")}
	saveList(t, store, weekend)

	selection := []string{pad.ID, shoes.ID, chalk.ID, "gone"}
	got, err := store.CopyItems(ctx, weekend.ID, climbing.ID, "member", selection)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Climbing shoes/Climbing", "Crash pad/Climbing"}; !reflect.DeepEqual(itemNames(got.Added), want) {
		t.Fatalf("copied %v, want %v in source order", itemNames(got.Added), want)
	}
	if !reflect.DeepEqual(got.Skipped, []string{"Chalk bag"}) {
		t.Fatalf("skipped %v", got.Skipped)
	}
	if got.Added[0].ID == shoes.ID || got.Added[0].Scope != "person" || len(got.List.Tasks) != 0 {
		t.Fatalf("copy kept the source ID, lost its scope or copied tasks: %+v", got.List)
	}

	// A double submit copies nothing more.
	again, err := store.CopyItems(ctx, weekend.ID, climbing.ID, "member", selection)
	if err != nil || len(again.Added) != 0 {
		t.Fatalf("second copy: %+v, %v", again, err)
	}

	// Later source edits stay on the source.
	current, _ := store.GetPackingList(ctx, climbing.ID, "owner")
	if err := store.UpdateItem(ctx, climbing.ID, "owner", packing.PackingItem{ID: pad.ID, Name: "Big crash pad", Category: "Climbing", SourceRevision: current.Revision()}); err != nil {
		t.Fatal(err)
	}
	if saved, _ := store.GetPackingList(ctx, weekend.ID, "member"); saved.Items[2].Name != "Crash pad" {
		t.Fatalf("source edit reached the copy: %+v", saved.Items)
	}
}

func TestCopyItemsNeedsReadOnTheSourceAndEditOnTheDestination(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	source := packing.NewList("owner", "Documents", "")
	passport := packing.NewItem("Passport", "Documents")
	source.Items = []packing.PackingItem{passport}
	saveList(t, store, source)
	destination := packing.NewList("owner", "Weekend", "")
	saveList(t, store, destination)
	private := packing.NewList("stranger", "Stranger's list", "")
	saveList(t, store, private)

	// A trip member is not a template editor.
	trip, _ := store.CreatePackingSession(ctx, destination.ID, "owner")
	link, _ := store.CreateInvitation(ctx, "packing-session", trip.ID, "owner")
	store.RequestAccess(ctx, link, packing.Member{Subject: "packer"})
	store.DecideInvitation(ctx, link.Kind, link.ID, "owner", link.Hash(), true)
	if _, err := store.CopyItems(ctx, destination.ID, private.ID, "packer", []string{passport.ID}); !errors.Is(err, packing.ErrNotFound) {
		t.Fatalf("copy from an unreadable source: %v", err)
	}
	own := packing.NewList("packer", "Packer's documents", "")
	own.Items = []packing.PackingItem{packing.NewItem("Passport", "")}
	saveList(t, store, own)
	if _, err := store.CopyItems(ctx, destination.ID, own.ID, "packer", []string{own.Items[0].ID}); !errors.Is(err, packing.ErrNotFound) {
		t.Fatalf("copy into a list the actor cannot edit: %v", err)
	}
	if _, err := store.CopyItems(ctx, source.ID, source.ID, "owner", []string{passport.ID}); !errors.Is(err, packing.ErrInvalid) {
		t.Fatalf("copy into the same list: %v", err)
	}
	if _, err := store.CopyItems(ctx, destination.ID, source.ID, "owner", []string{"gone"}); !errors.Is(err, packing.ErrItemsGone) {
		t.Fatalf("copy with nothing selected: %v", err)
	}
	if saved, _ := store.GetPackingList(ctx, destination.ID, "owner"); len(saved.Items) != 0 {
		t.Fatalf("rejected copies wrote items: %+v", saved.Items)
	}
}

func TestParseItemLines(t *testing.T) {
	type line = packing.ItemLine
	for name, test := range map[string]struct {
		text string
		want []line
	}{
		"notes app": {
			text: "Climbing:\n- Climbing shoes\n- Chalk bag\n- Crash pad\n\nDocuments:\n- Passport\n- Driving licence\n- Paper map (phone may die)\n",
			want: []line{{2, "Climbing shoes", "Climbing"}, {3, "Chalk bag", "Climbing"}, {4, "Crash pad", "Climbing"}, {7, "Passport", "Documents"}, {8, "Driving licence", "Documents"}, {9, "Paper map (phone may die)", "Documents"}},
		},
		"reminders": {
			text: "◦ Harness\r\n◦ Belay device\r\n☐ Passport",
			want: []line{{1, "Harness", ""}, {2, "Belay device", ""}, {3, "Passport", ""}},
		},
		"markdown": {
			text: "## Clothing\n- [ ] Shoes\n- [x] clothes\n* Rain jacket\n---\n1. Chalk\n2) Tape",
			want: []line{{2, "Shoes", "Clothing"}, {3, "clothes", "Clothing"}, {4, "Rain jacket", "Clothing"}, {6, "Chalk", "Clothing"}, {7, "Tape", "Clothing"}},
		},
		"tricky": {
			text: "Map 1:50,000\nNote: bring cash\nTime: 10:30:\n1.5L bottle\n-5 bag\n  shoes  \n:\n",
			want: []line{{1, "Map 1:50,000", ""}, {2, "Note: bring cash", ""}, {3, "Time: 10:30:", ""}, {4, "1.5L bottle", ""}, {5, "-5 bag", ""}, {6, "shoes", ""}},
		},
	} {
		if got := packing.ParseItemLines(test.text); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s:\n got %v\nwant %v", name, got, test.want)
		}
	}
}
