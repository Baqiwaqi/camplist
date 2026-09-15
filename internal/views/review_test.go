package views

import (
	"bytes"
	"camplist/internal/packing"
	"context"
	"strings"
	"testing"
)

func TestTripObservationsDescribeChangesAsSentences(t *testing.T) {
	list := packing.NewList("user", "Camping", "")
	session := packing.NewPackingSession(list)
	session.Review = []packing.ReviewEntry{
		{ID: "remove", Action: packing.ReviewRemove, Name: "Thermos"},
		{ID: "add", Action: packing.ReviewAdd, Name: "Head torch"},
		{ID: "task", Action: packing.ReviewTask, Name: "Car", Task: "Check tyre pressure"},
	}
	list.AppliedReviews = []string{session.ID + ":remove"}
	var out bytes.Buffer
	if err := TripObservations(session, list, true, false, "token", nil).Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Remove Thermos from the list. Applied to future trips.",
		"Add Head torch to the list",
		"Add the preparation task Check tyre pressure",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("observations missing %q", want)
		}
	}
	for _, raw := range []string{"remove:", "add:", "task:"} {
		if strings.Contains(out.String(), raw) {
			t.Errorf("observations show raw action label %q", raw)
		}
	}
}

func TestCurrentReviewListIsASheetOfRows(t *testing.T) {
	list := packing.NewList("user", "Camping", "")
	list.Items = []packing.PackingItem{packing.NewItem("Stove", "Kitchen"), packing.NewItem("Matches", "")}
	list.Tasks = []packing.PreparationTask{{ID: "fuel", Name: "Buy fuel"}}
	doc := renderDoc(t, CurrentReviewList(list, nil))
	if strings.Contains(text(doc), "—") {
		t.Errorf("current list still joins names and categories with a dash: %q", text(doc))
	}
	if count := findElement(doc, byClass("section-count")); count == nil || text(count) != "2 items" {
		t.Error("current list does not count its items")
	}
	gear := findElement(doc, hasAttr("aria-label", "Gear"))
	if gear == nil || len(findAll(gear, byTag("li"))) != 2 || !strings.Contains(text(gear), "Kitchen") {
		t.Fatal("gear rows missing")
	}
	tasks := findElement(doc, hasAttr("aria-label", "Preparation tasks"))
	if tasks == nil || text(tasks) != "Buy fuel" {
		t.Error("task rows missing")
	}
}
