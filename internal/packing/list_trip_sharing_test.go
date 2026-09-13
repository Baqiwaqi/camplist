package packing_test

import (
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type listTripFixture struct {
	store             *packing.Store
	list              packing.PackingList
	first, second     packing.PackingSession
	packed, otherList packing.PackingSession
	link              packing.InvitationLink
}

// newListTripFixture has a list with two current trips, one archived trip, a
// trip from another list, and a pending list request from "guest".
func newListTripFixture(t *testing.T) listTripFixture {
	t.Helper()
	ctx := context.Background()
	// Trips packed now count as archived a day later.
	later := time.Now().Add(48 * time.Hour)
	store := packing.NewStore(testsupport.NewDocuments(), packing.WithClock(func() time.Time { return later }))
	f := listTripFixture{store: store}
	f.list = packing.NewList("owner", "Weekend", "")
	f.list.Items = []packing.PackingItem{packing.NewItem("Tent", "")}
	other := packing.NewList("owner", "Other", "")
	for _, list := range []packing.PackingList{f.list, other} {
		if err := store.SavePackingList(ctx, list); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	for _, trip := range []*packing.PackingSession{&f.first, &f.second, &f.packed} {
		if *trip, err = store.CreatePackingSession(ctx, f.list.ID, "owner"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = store.SetSessionItem(ctx, f.packed.ID, "owner", f.list.Items[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if f.otherList, err = store.CreatePackingSession(ctx, other.ID, "owner"); err != nil {
		t.Fatal(err)
	}
	if f.link, err = store.CreateInvitation(ctx, "packing-list", f.list.ID, "owner"); err != nil {
		t.Fatal(err)
	}
	if err = store.RequestAccess(ctx, f.link, packing.Member{Subject: "guest", Name: "Guest"}); err != nil {
		t.Fatal(err)
	}
	return f
}

func tripIDs(trips []packing.PackingSession) map[string]bool {
	ids := map[string]bool{}
	for _, trip := range trips {
		ids[trip.ID] = true
	}
	return ids
}

func TestListTripsOffersOnlyTheOwnersCurrentTripsFromTheList(t *testing.T) {
	f := newListTripFixture(t)
	trips, err := f.store.ListTrips(context.Background(), f.list.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	ids := tripIDs(trips)
	if len(ids) != 2 || !ids[f.first.ID] || !ids[f.second.ID] {
		t.Fatalf("offered trips %v, want the two current trips from the list", ids)
	}
	if _, err := f.store.ListTrips(context.Background(), f.list.ID, "guest"); err == nil {
		t.Fatal("a pending requester listed the owner's trips")
	}
}

func TestApprovingListRequestWithoutTripsSharesOnlyTheList(t *testing.T) {
	ctx := context.Background()
	f := newListTripFixture(t)
	if err := f.store.ApproveInvitationWithTrips(ctx, f.list.ID, "owner", f.link.Hash(), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.GetPackingList(ctx, f.list.ID, "guest"); err != nil {
		t.Fatalf("list not shared: %v", err)
	}
	if _, err := f.store.GetPackingSession(ctx, f.first.ID, "guest"); err == nil {
		t.Fatal("trip shared without being chosen")
	}
	sessions, err := f.store.ListPackingSession(ctx, "guest")
	if err != nil || len(sessions) != 0 {
		t.Fatalf("guest trips %v %v", sessions, err)
	}
}

func TestApprovingListRequestSharesChosenTripsWithTheSameAccount(t *testing.T) {
	ctx := context.Background()
	f := newListTripFixture(t)
	chosen := []string{f.first.ID, f.first.ID}
	if err := f.store.ApproveInvitationWithTrips(ctx, f.list.ID, "owner", f.link.Hash(), chosen); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.GetPackingList(ctx, f.list.ID, "guest"); err != nil {
		t.Fatalf("list not shared: %v", err)
	}
	trip, err := f.store.GetPackingSession(ctx, f.first.ID, "guest")
	if err != nil {
		t.Fatalf("chosen trip not shared: %v", err)
	}
	if trip.Sharing.Members["guest"].Name != "Guest" {
		t.Fatalf("trip member %+v", trip.Sharing.Members)
	}
	if _, err := f.store.GetPackingSession(ctx, f.second.ID, "guest"); err == nil {
		t.Fatal("unchosen trip shared")
	}
	sessions, err := f.store.ListPackingSession(ctx, "guest")
	if err != nil || len(sessions) != 1 || sessions[0].ID != f.first.ID {
		t.Fatalf("guest trip overview %v %v", sessions, err)
	}
	if _, err := f.store.SyncSessionItem(ctx, f.first.ID, "guest", packing.PackingOperation{ID: "op", ItemID: trip.List.Items[0].ID, Checked: true}); err != nil {
		t.Fatalf("member cannot pack shared trip: %v", err)
	}
	// A repeated submission is harmless and can add a trip missed earlier.
	if err := f.store.ApproveInvitationWithTrips(ctx, f.list.ID, "owner", f.link.Hash(), []string{f.first.ID, f.second.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.GetPackingSession(ctx, f.second.ID, "guest"); err != nil {
		t.Fatalf("retry did not share trip: %v", err)
	}
	// Trip and list membership stay independent.
	if err := f.store.RemoveMember(ctx, "packing-session", f.first.ID, "owner", "guest"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.GetPackingSession(ctx, f.first.ID, "guest"); !errors.Is(err, packing.ErrAccessRemoved) {
		t.Fatalf("removed trip member: %v", err)
	}
	if _, err := f.store.GetPackingList(ctx, f.list.ID, "guest"); err != nil {
		t.Fatalf("trip removal changed list access: %v", err)
	}
}

func addMembers(t *testing.T, store *packing.Store, kind, id string, subjects ...string) {
	t.Helper()
	ctx := context.Background()
	for _, subject := range subjects {
		link, err := store.CreateInvitation(ctx, kind, id, "owner")
		if err != nil {
			t.Fatal(err)
		}
		if err = store.RequestAccess(ctx, link, packing.Member{Subject: subject}); err != nil {
			t.Fatal(err)
		}
		if err = store.DecideInvitation(ctx, kind, id, "owner", link.Hash(), true); err != nil {
			t.Fatal(err)
		}
	}
}

func subjects(prefix string, n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("%s-%d", prefix, i)
	}
	return names
}

func TestApprovingAtTheListMemberLimitSharesNoTrips(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	store := packing.NewStore(testsupport.NewDocuments(), packing.WithClock(func() time.Time { return now }))
	list := packing.NewList("owner", "Weekend", "")
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	addMembers(t, store, "packing-list", list.ID, subjects("early", 19)...)
	// Approved links expire, which frees invitation slots while members stay.
	now = now.Add(8 * 24 * time.Hour)
	link, err := store.CreateInvitation(ctx, "packing-list", list.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RequestAccess(ctx, link, packing.Member{Subject: "guest"}); err != nil {
		t.Fatal(err)
	}
	addMembers(t, store, "packing-list", list.ID, "last")
	trip, err := store.CreatePackingSession(ctx, list.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.ApproveInvitationWithTrips(ctx, list.ID, "owner", link.Hash(), []string{trip.ID}); !errors.Is(err, packing.ErrInvalid) {
		t.Fatalf("approval over the member limit: %v", err)
	}
	if _, err = store.GetPackingSession(ctx, trip.ID, "guest"); err == nil {
		t.Fatal("trip shared although the list approval failed")
	}
	if _, err = store.GetPackingList(ctx, list.ID, "guest"); err == nil {
		t.Fatal("list shared over the member limit")
	}
}

func TestFailedTripGrantAfterListApprovalCanBeRetried(t *testing.T) {
	ctx := context.Background()
	f := newListTripFixture(t)
	addMembers(t, f.store, "packing-session", f.first.ID, subjects("packer", 20)...)
	if err := f.store.ApproveInvitationWithTrips(ctx, f.list.ID, "owner", f.link.Hash(), []string{f.first.ID}); !errors.Is(err, packing.ErrInvalid) {
		t.Fatalf("grant to a full trip: %v", err)
	}
	if _, err := f.store.GetPackingList(ctx, f.list.ID, "guest"); err != nil {
		t.Fatalf("list approval did not commit before the trip grant: %v", err)
	}
	if _, err := f.store.GetPackingSession(ctx, f.first.ID, "guest"); err == nil {
		t.Fatal("full trip gained a member")
	}
	if err := f.store.RemoveMember(ctx, "packing-session", f.first.ID, "owner", "packer-0"); err != nil {
		t.Fatal(err)
	}
	if err := f.store.ApproveInvitationWithTrips(ctx, f.list.ID, "owner", f.link.Hash(), []string{f.first.ID}); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if _, err := f.store.GetPackingSession(ctx, f.first.ID, "guest"); err != nil {
		t.Fatalf("retry did not share trip: %v", err)
	}
}

func TestApprovingWithTripsRequiresOwnerAndTheListsOwnTrips(t *testing.T) {
	ctx := context.Background()
	f := newListTripFixture(t)
	for name, test := range map[string]struct {
		actor string
		trips []string
		want  error
	}{
		"requester":         {"guest", []string{f.first.ID}, packing.ErrAccessRemoved},
		"other list's trip": {"owner", []string{f.first.ID, f.otherList.ID}, packing.ErrNotFound},
		"unknown trip":      {"owner", []string{"missing"}, packing.ErrNotFound},
	} {
		if err := f.store.ApproveInvitationWithTrips(ctx, f.list.ID, test.actor, f.link.Hash(), test.trips); !errors.Is(err, test.want) {
			t.Errorf("%s: got %v, want %v", name, err, test.want)
		}
	}
	if _, err := f.store.GetPackingSession(ctx, f.first.ID, "guest"); err == nil {
		t.Fatal("rejected approval shared a trip")
	}
	if _, err := f.store.GetPackingList(ctx, f.list.ID, "guest"); err == nil {
		t.Fatal("rejected approval shared the list")
	}

	// An editor of the list cannot share the owner's trips.
	editorLink, _ := f.store.CreateInvitation(ctx, "packing-list", f.list.ID, "owner")
	f.store.RequestAccess(ctx, editorLink, packing.Member{Subject: "editor"})
	if err := f.store.DecideInvitation(ctx, "packing-list", f.list.ID, "owner", editorLink.Hash(), true); err != nil {
		t.Fatal(err)
	}
	if err := f.store.ApproveInvitationWithTrips(ctx, f.list.ID, "editor", f.link.Hash(), []string{f.first.ID}); !errors.Is(err, packing.ErrForbidden) {
		t.Fatalf("editor approval: %v", err)
	}

	// A removed list member is not re-added, and gets no trips, by a stale approval.
	if err := f.store.ApproveInvitationWithTrips(ctx, f.list.ID, "owner", f.link.Hash(), nil); err != nil {
		t.Fatal(err)
	}
	if err := f.store.RemoveMember(ctx, "packing-list", f.list.ID, "owner", "guest"); err != nil {
		t.Fatal(err)
	}
	if err := f.store.ApproveInvitationWithTrips(ctx, f.list.ID, "owner", f.link.Hash(), []string{f.first.ID}); !errors.Is(err, packing.ErrInvalid) {
		t.Fatalf("stale approval: %v", err)
	}
	if _, err := f.store.GetPackingSession(ctx, f.first.ID, "guest"); err == nil {
		t.Fatal("removed list member received a trip")
	}

	// An open link nobody has requested names no account to share with.
	open, _ := f.store.CreateInvitation(ctx, "packing-list", f.list.ID, "owner")
	if err := f.store.ApproveInvitationWithTrips(ctx, f.list.ID, "owner", open.Hash(), []string{f.first.ID}); !errors.Is(err, packing.ErrInvalid) {
		t.Fatalf("open invitation: %v", err)
	}
}

func TestStartTripWithMembersGrantsOnlyChosenListMembers(t *testing.T) {
	f := newListTripFixture(t)
	ctx := context.Background()
	if err := f.store.DecideInvitation(ctx, "packing-list", f.list.ID, "owner", f.link.Hash(), true); err != nil {
		t.Fatal(err)
	}
	list, err := f.store.GetPackingList(ctx, f.list.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	list.Items = append(list.Items, packing.PackingItem{ID: "sleeping-bag", Name: "Sleeping bag", Scope: "person"})
	if err = f.store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	if got := list.TripMemberChoices("guest"); len(got) != 0 {
		t.Fatalf("a list member is offered %v", got)
	}
	if got := list.TripMemberChoices("owner"); len(got) != 1 || got[0].Subject != "guest" {
		t.Fatalf("owner is offered %v, want the guest", got)
	}
	for _, subjects := range [][]string{{"stranger"}, {"owner"}} {
		if _, err = f.store.StartTripWithMembers(ctx, f.list.ID, "owner", "Owner", subjects); !errors.Is(err, packing.ErrNotListMember) {
			t.Fatalf("%v: got %v, want ErrNotListMember", subjects, err)
		}
	}
	if _, err = f.store.StartTripWithMembers(ctx, f.list.ID, "guest", "Guest", []string{"guest"}); !errors.Is(err, packing.ErrForbidden) {
		t.Fatalf("a list member added trip members: %v", err)
	}
	trip, err := f.store.StartTripWithMembers(ctx, f.list.ID, "owner", "Owner", []string{"guest", "guest"})
	if err != nil {
		t.Fatal(err)
	}
	shared, err := f.store.GetPackingSession(ctx, trip.ID, "guest")
	if err != nil || len(shared.Sharing.Members) != 1 {
		t.Fatalf("guest cannot open the new trip (%v) or members are %v", err, shared.Sharing.Members)
	}
	bags := map[string]bool{}
	for _, item := range shared.List.Items {
		if item.SourceID == "sleeping-bag" {
			bags[item.Assignee] = true
		}
	}
	if !bags["owner"] || !bags["guest"] {
		t.Errorf("personal items were not copied for each person: %v", bags)
	}
}
