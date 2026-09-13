package packing_test

import (
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"encoding/json"
	"errors"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
	"testing"
	"time"
)

func TestInvitationRequiresApprovalAndGrantsOnlyItsResource(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Weekend", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "")}
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
	guest := packing.Member{Subject: "guest", Name: "Guest", Email: "guest@example.com"}
	if err := store.RequestAccess(ctx, link, guest); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPackingSession(ctx, trip.ID, "guest"); err == nil {
		t.Fatal("request alone granted access")
	}
	if err := store.DecideInvitation(ctx, link.Kind, link.ID, "owner", link.Hash(), true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPackingSession(ctx, trip.ID, "guest"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPackingList(ctx, list.ID, "guest"); err == nil {
		t.Fatal("trip membership granted template access")
	}
	if err := store.DeletePackingSession(ctx, trip.ID, "guest"); !errors.Is(err, packing.ErrForbidden) {
		t.Fatalf("member delete: %v", err)
	}
	if err := store.RemoveMember(ctx, link.Kind, link.ID, "owner", "guest"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPackingSession(ctx, trip.ID, "guest"); !errors.Is(err, packing.ErrAccessRemoved) {
		t.Fatalf("removed member: %v", err)
	}
}

func TestTemplateEditorCreatesPrivateSnapshotAndLosesWriteAccessOnRemoval(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Shared kit", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "")}
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	link, err := store.CreateInvitation(ctx, "packing-list", list.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RequestAccess(ctx, link, packing.Member{Subject: "editor", Name: "Editor"}); err != nil {
		t.Fatal(err)
	}
	if err = store.DecideInvitation(ctx, link.Kind, link.ID, "owner", link.Hash(), true); err != nil {
		t.Fatal(err)
	}
	lists, err := store.GetPackingLists(ctx, "editor")
	if err != nil || len(lists) != 1 {
		t.Fatalf("discovery: %v %v", lists, err)
	}
	editable, err := store.GetPackingList(ctx, list.ID, "editor")
	if err != nil {
		t.Fatal(err)
	}
	editable.Name = "Family kit"
	if err = store.SavePackingList(ctx, editable); err != nil {
		t.Fatal(err)
	}
	trip, err := store.CreatePackingSession(ctx, list.ID, "editor")
	if err != nil {
		t.Fatal(err)
	}
	if trip.UserID != "editor" {
		t.Fatal("trip inherited template owner")
	}
	if _, err = store.GetPackingSession(ctx, trip.ID, "owner"); err == nil {
		t.Fatal("template owner gained private trip access")
	}
	stale, err := store.GetPackingList(ctx, list.ID, "editor")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.RemoveMember(ctx, link.Kind, link.ID, "owner", "editor"); err != nil {
		t.Fatal(err)
	}
	stale.Name = "Forbidden"
	if err = store.SavePackingList(ctx, stale); !errors.Is(err, packing.ErrAccessRemoved) {
		t.Fatalf("stale write after removal: %v", err)
	}
	if err = store.AddItem(ctx, list.ID, "editor", packing.NewItem("Stove", "")); !errors.Is(err, packing.ErrAccessRemoved) {
		t.Fatalf("add after removal: %v", err)
	}
	if _, err = store.GetPackingSession(ctx, trip.ID, "editor"); err != nil {
		t.Fatal("removal erased private snapshot", err)
	}
}

func TestSharedSyncPreservesOtherMembersAndAcknowledgesMatchingIntent(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Trip", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", ""), packing.NewItem("Stove", "")}
	store.SavePackingList(ctx, list)
	trip, _ := store.CreatePackingSession(ctx, list.ID, "owner")
	link, _ := store.CreateInvitation(ctx, "packing-session", trip.ID, "owner")
	store.RequestAccess(ctx, link, packing.Member{Subject: "guest", Name: "Alex"})
	store.DecideInvitation(ctx, link.Kind, link.ID, "owner", link.Hash(), true)
	op := packing.PackingOperation{ID: "same-id", ItemID: list.Items[0].ID, Checked: true, ExpectedRevision: 0}
	first, err := store.SyncSessionItem(ctx, trip.ID, "owner", op)
	if err != nil {
		t.Fatal(err)
	}
	matching, err := store.SyncSessionItem(ctx, trip.ID, "guest", op)
	if err != nil {
		t.Fatal(err)
	}
	if matching.List.Items[0].Revision != 1 || matching.List.Items[0].ChangedBy != first.List.Items[0].ChangedBy {
		t.Fatal("matching intent rewrote attribution")
	}
	if matching.List.Items[0].ChangedByID != "owner" {
		t.Fatalf("attribution account = %q, want owner", matching.List.Items[0].ChangedByID)
	}
	stove := packing.PackingOperation{ID: "stove", ItemID: list.Items[1].ID, Checked: true, ExpectedRevision: 0}
	both, err := store.SyncSessionItem(ctx, trip.ID, "guest", stove)
	if err != nil || !both.List.Items[0].Checked || !both.List.Items[1].Checked {
		t.Fatalf("independent merge: %+v %v", both, err)
	}
	stale := packing.PackingOperation{ID: "unpack", ItemID: list.Items[0].ID, Checked: false, ExpectedRevision: 0}
	conflict, err := store.SyncSessionItem(ctx, trip.ID, "guest", stale)
	if !errors.Is(err, packing.ErrConflict) || !conflict.List.Items[0].Checked {
		t.Fatal("stale intent overwrote shared state", err)
	}
	store.RemoveMember(ctx, link.Kind, link.ID, "owner", "guest")
	if _, err = store.SyncSessionItem(ctx, trip.ID, "guest", stove); !errors.Is(err, packing.ErrAccessRemoved) {
		t.Fatal("receipt replay bypassed revocation", err)
	}
}

func TestSharedTemplateFormsRejectStaleEditsAndEditorsManagePreparation(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Kit", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "")}
	store.SavePackingList(ctx, list)
	link, _ := store.CreateInvitation(ctx, "packing-list", list.ID, "owner")
	store.RequestAccess(ctx, link, packing.Member{Subject: "editor"})
	store.DecideInvitation(ctx, link.Kind, link.ID, "owner", link.Hash(), true)
	before, _ := store.GetPackingList(ctx, list.ID, "editor")
	item := before.Items[0]
	item.Name = "New tent"
	owner, _ := store.GetPackingList(ctx, list.ID, "owner")
	owner.Description = "Changed"
	store.SavePackingList(ctx, owner)
	if err := store.UpdateItem(ctx, list.ID, "editor", item); !errors.Is(err, packing.ErrConflict) {
		t.Fatal("stale item form accepted", err)
	}
	current, _ := store.GetPackingList(ctx, list.ID, "editor")
	task := packing.PreparationTask{ID: "fuel", Name: "Buy fuel"}
	saved, err := store.EditPreparationTask(ctx, list.ID, "editor", task, false, current.Revision())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.EditPreparationTask(ctx, list.ID, "editor", task, true, current.Revision()); !errors.Is(err, packing.ErrConflict) {
		t.Fatal("stale task form accepted", err)
	}
	current, _ = store.GetPackingList(ctx, list.ID, "editor")
	if len(current.Tasks) != 1 {
		t.Fatal("task not saved")
	}
	if saved.Revision() == "" || saved.Revision() != current.Revision() {
		t.Fatalf("saved revision %q, stored %q", saved.Revision(), current.Revision())
	}
	if _, err := store.EditPreparationTask(ctx, list.ID, "editor", task, true, current.Revision()); err != nil {
		t.Fatal(err)
	}
}

func TestExpiredAndClaimedInvitationsCannotGrantAnotherAccountAccess(t *testing.T) {
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	store := packing.NewStore(testsupport.NewDocuments(), packing.WithClock(func() time.Time { return now }))
	ctx := context.Background()
	list := packing.NewList("owner", "Kit", "")
	store.SavePackingList(ctx, list)
	link, _ := store.CreateInvitation(ctx, "packing-list", list.ID, "owner")
	if err := store.RequestAccess(ctx, link, packing.Member{Subject: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RequestAccess(ctx, link, packing.Member{Subject: "second"}); err == nil {
		t.Fatal("second account claimed link")
	}
	now = now.Add(8 * 24 * time.Hour)
	if err := store.DecideInvitation(ctx, link.Kind, link.ID, "owner", link.Hash(), true); err == nil {
		t.Fatal("expired request approved")
	}
	if _, err := store.GetPackingList(ctx, list.ID, "first"); err == nil {
		t.Fatal("expired request granted access")
	}
}

type ttlCaptureDatabase struct {
	*testsupport.Documents
	shareLinkID string
	shareLink   map[string]any
}

func (d *ttlCaptureDatabase) CreateItem(ctx context.Context, pk azcosmos.PartitionKey, data []byte, options *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		return azcosmos.ItemResponse{}, err
	}
	if record["type"] == "share-link" {
		d.shareLink = record
		d.shareLinkID = record["id"].(string)
	}
	return d.Documents.CreateItem(ctx, pk, data, options)
}

func TestShareLinkUsesCosmosTTLAndStopsWorkingAfterAutomaticCleanup(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	db := &ttlCaptureDatabase{Documents: testsupport.NewDocuments()}
	store := packing.NewStore(db, packing.WithClock(func() time.Time { return now }))
	ctx := context.Background()
	list := packing.NewList("owner", "Kit", "")
	if err := store.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	link, err := store.CreateInvitation(ctx, "packing-list", list.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if db.shareLink["ttl"] != float64(7*24*60*60) {
		t.Fatalf("share link ttl = %v", db.shareLink["ttl"])
	}
	if db.shareLink["userId"] != "owner" || db.shareLink["resourceId"] != list.ID || db.shareLink["hash"] != link.Hash() {
		t.Fatalf("invalid share-link capability: %#v", db.shareLink)
	}
	if _, err = store.InvitationStatus(ctx, link, "guest"); err != nil {
		t.Fatal(err)
	}
	// Cosmos performs this deletion after ttl seconds. Once the capability item
	// is gone, stale invitation metadata cannot make the token usable again.
	if _, err = db.DeleteItem(ctx, azcosmos.NewPartitionKeyString("owner"), db.shareLinkID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = store.InvitationStatus(ctx, link, "guest"); !errors.Is(err, packing.ErrNotFound) {
		t.Fatalf("cleaned-up share link remains usable: %v", err)
	}
}

// The external database changes between authorization and the conditional commit.
type revocationDatabase struct {
	*testsupport.Documents
	before func()
}

func (d *revocationDatabase) ReplaceItem(ctx context.Context, pk azcosmos.PartitionKey, id string, data []byte, options *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	if action := d.before; action != nil {
		d.before = nil
		action()
	}
	return d.Documents.ReplaceItem(ctx, pk, id, data, options)
}
func TestRevocationBetweenAuthorizationAndCommitBlocksTheWrite(t *testing.T) {
	ctx := context.Background()
	db := &revocationDatabase{Documents: testsupport.NewDocuments()}
	store := packing.NewStore(db)
	list := packing.NewList("owner", "Kit", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "")}
	store.SavePackingList(ctx, list)
	trip, _ := store.CreatePackingSession(ctx, list.ID, "owner")
	link, _ := store.CreateInvitation(ctx, "packing-session", trip.ID, "owner")
	store.RequestAccess(ctx, link, packing.Member{Subject: "guest"})
	store.DecideInvitation(ctx, link.Kind, link.ID, "owner", link.Hash(), true)
	db.before = func() {
		if err := store.RemoveMember(ctx, link.Kind, link.ID, "owner", "guest"); err != nil {
			t.Fatal(err)
		}
	}
	_, err := store.SyncSessionItem(ctx, trip.ID, "guest", packing.PackingOperation{ID: "racing", ItemID: list.Items[0].ID, Checked: true, ExpectedRevision: 0})
	if !errors.Is(err, packing.ErrAccessRemoved) {
		t.Fatal("write did not reauthorize after conflict", err)
	}
	current, err := store.GetPackingSession(ctx, trip.ID, "owner")
	if err != nil || current.List.Items[0].Checked {
		t.Fatal("revoked write committed", err)
	}
}

func TestApplyingReviewRequiresIndependentTemplatePermission(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Kit", "")
	store.SavePackingList(ctx, list)
	trip, _ := store.CreatePackingSession(ctx, list.ID, "owner")
	_, err := store.AddReviewEntry(ctx, trip.ID, "owner", packing.ReviewEntry{ID: "matches", Name: "Matches", Forgotten: true, Action: packing.ReviewAdd})
	if err != nil {
		t.Fatal(err)
	}
	link, _ := store.CreateInvitation(ctx, "packing-session", trip.ID, "owner")
	store.RequestAccess(ctx, link, packing.Member{Subject: "guest"})
	store.DecideInvitation(ctx, link.Kind, link.ID, "owner", link.Hash(), true)
	template, _ := store.GetPackingList(ctx, list.ID, "owner")
	if _, err := store.ApplyReview(ctx, trip.ID, "guest", template.Revision(), []string{"matches"}); err == nil {
		t.Fatal("trip grant edited template")
	}
	listLink, _ := store.CreateInvitation(ctx, "packing-list", list.ID, "owner")
	store.RequestAccess(ctx, listLink, packing.Member{Subject: "guest"})
	store.DecideInvitation(ctx, listLink.Kind, listLink.ID, "owner", listLink.Hash(), true)
	template, _ = store.GetPackingList(ctx, list.ID, "guest")
	applied, err := store.ApplyReview(ctx, trip.ID, "guest", template.Revision(), []string{"matches"})
	if err != nil || len(applied.Items) != 1 {
		t.Fatal("both grants could not apply review", err)
	}
	store.RemoveMember(ctx, "packing-list", list.ID, "owner", "guest")
	if _, err := store.ApplyReview(ctx, trip.ID, "guest", applied.Revision(), []string{"matches"}); !errors.Is(err, packing.ErrAccessRemoved) {
		t.Fatal("idempotent apply bypassed permission", err)
	}
}

type discoveryFailureDatabase struct {
	*testsupport.Documents
	fail bool
}

func (d *discoveryFailureDatabase) CreateItem(ctx context.Context, pk azcosmos.PartitionKey, data []byte, options *azcosmos.ItemOptions) (azcosmos.ItemResponse, error) {
	var record struct {
		Type string `json:"type"`
	}
	json.Unmarshal(data, &record)
	if d.fail && record.Type == "shared-reference" {
		d.fail = false
		return azcosmos.ItemResponse{}, errors.New("temporary database failure")
	}
	return d.Documents.CreateItem(ctx, pk, data, options)
}
func TestDiscoveryFailureCanBeRetriedWithoutGrantingAccess(t *testing.T) {
	ctx := context.Background()
	db := &discoveryFailureDatabase{Documents: testsupport.NewDocuments(), fail: true}
	store := packing.NewStore(db)
	list := packing.NewList("owner", "Kit", "")
	store.SavePackingList(ctx, list)
	link, _ := store.CreateInvitation(ctx, "packing-list", list.ID, "owner")
	member := packing.Member{Subject: "guest"}
	if err := store.RequestAccess(ctx, link, member); err == nil {
		t.Fatal("injected outage was not surfaced")
	}
	if err := store.DecideInvitation(ctx, link.Kind, link.ID, "owner", link.Hash(), true); err == nil {
		t.Fatal("failed request became approvable")
	}
	if err := store.RequestAccess(ctx, link, member); err != nil {
		t.Fatal(err)
	}
	if err := store.DecideInvitation(ctx, link.Kind, link.ID, "owner", link.Hash(), true); err != nil {
		t.Fatal(err)
	}
	if err := store.DecideInvitation(ctx, link.Kind, link.ID, "owner", link.Hash(), true); err != nil {
		t.Fatal("approval retry failed", err)
	}
	lists, err := store.GetPackingLists(ctx, "guest")
	if err != nil || len(lists) != 1 {
		t.Fatal("retry did not repair discoverability", err)
	}
	store.RemoveMember(ctx, link.Kind, link.ID, "owner", "guest")
	store.DecideInvitation(ctx, link.Kind, link.ID, "owner", link.Hash(), true)
	if _, err := store.GetPackingList(ctx, list.ID, "guest"); !errors.Is(err, packing.ErrAccessRemoved) {
		t.Fatal("old approval resurrected removed access", err)
	}
}
