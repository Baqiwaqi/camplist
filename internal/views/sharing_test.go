package views

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"

	"camplist/internal/packing"
)

func sharingFixture() (packing.SharingView, []packing.PackingSession) {
	expires := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)
	view := packing.SharingView{Kind: "packing-list", ID: "list", Owner: "user", Actor: "user", Sharing: packing.Sharing{
		Members: map[string]packing.Member{"sam": {Subject: "sam", Name: "Sam", Email: "sam@example.com"}},
		Invitations: []packing.Invitation{
			{Hash: "pending", Status: "pending", Expires: expires, Applicant: &packing.Member{Subject: "robin", Name: "Robin", Email: "robin@example.com"}},
			{Hash: "open", Status: "open", Expires: expires},
			{Hash: "revoked", Status: "revoked", Expires: expires},
		},
	}}
	return view, nil
}

// Each person row names the account, shows its status as a sentence-case tag
// and keeps the row's destructive action in a More menu, as a real form that
// posts without scripts.
func TestSharingPageRowsUseStatusTagsAndMenus(t *testing.T) {
	view, trips := sharingFixture()
	doc := renderDoc(t, SharingPage(view, "", "token", trips, ""))

	for hash, want := range map[string]string{"pending": "Waiting for approval", "open": "Not used yet", "revoked": "Revoked"} {
		row := findElement(doc, hasAttr("id", "invitation-"+hash))
		if row == nil {
			t.Fatalf("no row for %s invitation", hash)
		}
		tag := findElement(row, byClass("tag"))
		if tag == nil || text(tag) != want {
			t.Errorf("%s invitation tag = %v, want %q", hash, tag, want)
		}
		if strings.Contains(text(row), " · ") {
			t.Errorf("%s invitation still joins its facts with a middle dot: %q", hash, text(row))
		}
		revoke := findElement(row, hasAttr("value", "revoke"))
		if (revoke != nil) != (hash != "revoked") {
			t.Errorf("%s invitation revoke offered = %v", hash, revoke != nil)
		}
		if revoke != nil && !inMenuForm(revoke) {
			t.Errorf("%s invitation revoke is not a menu item in a posting form", hash)
		}
	}
	approve := findElement(findElement(doc, hasAttr("id", "invitation-pending")), hasAttr("value", "approve"))
	if approve == nil || !hasClass(approve, "btn-primary") {
		t.Error("a pending request lost its Approve button on the row")
	}

	members := findElement(doc, hasAttr("id", "members"))
	remove := findElement(members, func(n *html.Node) bool { return n.Data == "button" && text(n) == "Remove access" })
	if remove == nil || !inMenuForm(remove) {
		t.Fatal("Remove access is not a menu item in a posting form")
	}
	if count := findElement(members, byClass("section-count")); count == nil || text(count) != "1" {
		t.Error("members section does not count its members")
	}
	if !strings.Contains(text(members), "sam@example.com") || strings.Contains(text(members), "Sam · ") {
		t.Errorf("member row should show name and email on separate lines: %q", text(members))
	}
}

func inMenuForm(button *html.Node) bool {
	if v, _ := attr(button, "role"); v != "menuitem" {
		return false
	}
	var form, menu bool
	for n := button.Parent; n != nil; n = n.Parent {
		if n.Data == "form" {
			method, _ := attr(n, "method")
			_, action := attr(n, "action")
			form = form || method == "post" && action
		}
		if v, _ := attr(n, "role"); v == "menu" {
			menu = true
		}
	}
	return form && menu
}

func TestSharingPageShowsEmptyStatesWithoutRows(t *testing.T) {
	doc := renderDoc(t, SharingPage(packing.SharingView{Kind: "packing-session", ID: "trip", Owner: "user", Actor: "user"}, "", "token", nil, ""))
	rows := findElement(doc, hasAttr("id", "invitation-rows"))
	if rows == nil || rows.FirstChild != nil {
		t.Fatal("empty invitation rows must stay empty so the empty state shows and new rows can be appended")
	}
	if !strings.Contains(text(findElement(doc, hasAttr("id", "members"))), "No one else has access yet.") {
		t.Error("members section has no empty state")
	}
}

func TestApprovePromptFocusesTheFirstTrip(t *testing.T) {
	view, _ := sharingFixture()
	trips := []packing.PackingSession{{ID: "first"}, {ID: "second"}}
	doc := renderDoc(t, InvitationRow(view, view.Sharing.Invitations[0], trips, true, false, "token"))
	for _, trip := range trips {
		box := findElement(doc, hasAttr("value", trip.ID))
		if box == nil || hasAttrKey(box, "autofocus") != (trip.ID == "first") {
			t.Errorf("trip %s autofocus wrong", trip.ID)
		}
	}
}
