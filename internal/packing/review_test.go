package packing

import (
	"camplist/internal/testsupport"
	"context"
	"errors"
	"testing"
)

func newTestStore() *Store { return NewStore(testsupport.NewDocuments()) }
func seedTrip(t *testing.T) (*Store, PackingList, PackingSession) {
	t.Helper()
	s := newTestStore()
	list := NewList("camper", "Weekend", "")
	list.Items = []PackingItem{NewItem("Stove", "Kitchen"), NewItem("First aid", "Safety")}
	if err := s.SavePackingList(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	list, err := s.GetPackingList(context.Background(), list.ID, "camper")
	if err != nil {
		t.Fatal(err)
	}
	ses, err := s.CreatePackingSession(context.Background(), list.ID, "camper")
	if err != nil {
		t.Fatal(err)
	}
	return s, list, ses
}
func TestReviewCreatesPreparationWithoutChangingTripHistory(t *testing.T) {
	ctx := context.Background()
	s, list, ses := seedTrip(t)
	entry := ReviewEntry{ID: "fuel-review", ItemID: list.Items[0].ID, Name: "Stove", NeedsAttention: true, Note: "Ran out of fuel", Action: "task", Task: "Replace gas canister"}
	if _, err := s.AddReviewEntry(ctx, ses.ID, "camper", entry); err != nil {
		t.Fatal(err)
	}
	got, err := s.ApplyReview(ctx, ses.ID, "camper", list.Revision(), []string{entry.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tasks) != 1 || got.Tasks[0].Name != "Replace gas canister" || got.Tasks[0].Done {
		t.Fatalf("missing preparation: %+v", got.Tasks)
	}
	// Retrying the submitted selection must be harmless, even with its old revision.
	again, err := s.ApplyReview(ctx, ses.ID, "camper", list.Revision(), []string{entry.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Tasks) != 1 {
		t.Fatal("review duplicated a task")
	}
	history, err := s.GetPackingSession(ctx, ses.ID, "camper")
	if err != nil {
		t.Fatal(err)
	}
	if len(history.List.Tasks) != 0 || history.List.Items[0].Checked || len(history.Review) != 1 {
		t.Fatal("historical packing snapshot changed")
	}
	if _, err = s.GetPackingSession(ctx, ses.ID, "other"); !errors.Is(err, ErrNotFound) {
		t.Fatal("other camper read session")
	}
}

func TestReviewRequiresCurrentTemplateAndExplicitRemoval(t *testing.T) {
	ctx := context.Background()
	s, list, ses := seedTrip(t)
	unused := ReviewEntry{ID: "unused", ItemID: list.Items[1].ID, Name: "First aid", Unused: true, Action: "none"}
	add := ReviewEntry{ID: "forgot", Name: "Matches", Forgotten: true, Action: "add"}
	for _, entry := range []ReviewEntry{unused, add} {
		if _, err := s.AddReviewEntry(ctx, ses.ID, "camper", entry); err != nil {
			t.Fatal(err)
		}
	}
	list.Name = "Autumn camping"
	if err := s.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyReview(ctx, ses.ID, "camper", list.Revision(), []string{add.ID}); !errors.Is(err, ErrConflict) {
		t.Fatal("stale template accepted")
	}
	current, err := s.GetPackingList(ctx, list.ID, "camper")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := s.ApplyReview(ctx, ses.ID, "camper", current.Revision(), []string{add.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Items) != 3 || updated.Items[1].Name != "First aid" {
		t.Fatal("unused safety gear removed without selection")
	}
	if _, err = s.SetPreparationTask(ctx, list.ID, "camper", "missing", true, current.Revision()); !errors.Is(err, ErrNotFound) {
		t.Fatal("unknown task accepted")
	}
}

func TestDeletedTemplateCanBeRecoveredWithoutDuplicatingOrChangingHistory(t *testing.T) {
	ctx := context.Background()
	s, list, ses := seedTrip(t)
	list.DeletedAt = &list.CreatedAt
	if err := s.SavePackingList(ctx, list); err != nil {
		t.Fatal(err)
	}
	first, err := s.RecoverReviewTemplate(ctx, ses.ID, "camper")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.RecoverReviewTemplate(ctx, ses.ID, "camper")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == list.ID || first.ID != second.ID {
		t.Fatal("recovery must create exactly one new list")
	}
	got, err := s.GetPackingSession(ctx, ses.ID, "camper")
	if err != nil {
		t.Fatal(err)
	}
	if got.List.ID != list.ID || got.TemplateID() != first.ID {
		t.Fatal("recovery changed history or cannot apply future review")
	}
}
