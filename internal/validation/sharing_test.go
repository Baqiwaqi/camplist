package validation

import (
	"camplist/internal/packing"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCosmosSharing(t *testing.T) {
	owner := fmt.Sprintf("camplist-validation-sharing-%d", time.Now().UnixNano())
	member := owner + "-other"
	store := validationStore(t, owner)
	ctx := context.Background()
	list := packing.NewList(owner, "Shared validation kit", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter"), packing.NewItem("Stove", "Kitchen")}
	must(t, store.SavePackingList(ctx, list))
	invite, err := store.CreateInvitation(ctx, "packing-list", list.ID, owner)
	must(t, err)
	must(t, store.RequestAccess(ctx, invite, packing.Member{Subject: member, Name: "Second camper", Email: "validation@example.invalid"}))
	if _, err = store.GetPackingList(ctx, list.ID, member); err == nil {
		t.Fatal("pending request read list")
	}
	must(t, store.DecideInvitation(ctx, invite.Kind, invite.ID, owner, invite.Hash(), true))
	lists, err := store.GetPackingLists(ctx, member)
	must(t, err)
	if len(lists) != 1 {
		t.Fatal("shared template not discovered")
	}
	editable, err := store.GetPackingList(ctx, list.ID, member)
	must(t, err)
	editable.Name = "Updated shared kit"
	must(t, store.SavePackingList(ctx, editable))
	private, err := store.CreatePackingSession(ctx, list.ID, member)
	must(t, err)
	if private.UserID != member {
		t.Fatal("private snapshot has wrong owner")
	}
	if _, err = store.GetPackingSession(ctx, private.ID, owner); err == nil {
		t.Fatal("template owner read another camper's trip")
	}
	trip, err := store.CreatePackingSession(ctx, list.ID, owner)
	must(t, err)
	invitation, err := store.CreateInvitation(ctx, "packing-session", trip.ID, owner)
	must(t, err)
	must(t, store.RequestAccess(ctx, invitation, packing.Member{Subject: member, Name: "Second camper"}))
	must(t, store.DecideInvitation(ctx, invitation.Kind, trip.ID, owner, invitation.Hash(), true))
	var wg sync.WaitGroup
	failures := make(chan error, 2)
	for i, actor := range []string{owner, member} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.SyncSessionItem(ctx, trip.ID, actor, packing.PackingOperation{ID: "parallel", ItemID: list.Items[i].ID, Checked: true, ExpectedRevision: 0})
			failures <- err
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		must(t, err)
	}
	both, err := store.GetPackingSession(ctx, trip.ID, member)
	must(t, err)
	if both.List.CountChecked() != 2 {
		t.Fatal("different-item changes did not merge")
	}
	matching, err := store.SyncSessionItem(ctx, trip.ID, member, packing.PackingOperation{ID: "match", ItemID: list.Items[0].ID, Checked: true, ExpectedRevision: 0})
	must(t, err)
	if matching.List.Items[0].Revision != 1 || matching.List.Items[0].ChangedBy != both.List.Items[0].ChangedBy {
		t.Fatal("matching intent rewrote item")
	}
	if _, err = store.SyncSessionItem(ctx, trip.ID, member, packing.PackingOperation{ID: "stale", ItemID: list.Items[0].ID, Checked: false, ExpectedRevision: 0}); !errors.Is(err, packing.ErrConflict) {
		t.Fatal("stale unpack silently accepted", err)
	}
	must(t, store.RemoveMember(ctx, invitation.Kind, trip.ID, owner, member))
	if _, err = store.SyncSessionItem(ctx, trip.ID, member, packing.PackingOperation{ID: "match", ItemID: list.Items[0].ID, Checked: true, ExpectedRevision: 0}); !errors.Is(err, packing.ErrAccessRemoved) {
		t.Fatal("removed member replay accepted", err)
	}
	stillPrivate, err := store.GetPackingSession(ctx, private.ID, member)
	must(t, err)
	if stillPrivate.ID != private.ID {
		t.Fatal("independent snapshot lost")
	}
}
