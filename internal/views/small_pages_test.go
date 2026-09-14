package views

import (
	"testing"

	"camplist/internal/auth"
	"camplist/internal/packing"

	"golang.org/x/net/html"
)

// The join page stays a page without scripts: a page head, the signed-in
// account, then the one action its status allows.
func TestJoinPageUsesPageHeadAndOneActionPerStatus(t *testing.T) {
	link := packing.InvitationLink{Kind: "packing-session", ID: "trip-1", Owner: "owner", Token: "token"}
	profile := auth.Profile{Subject: "sub-1", Name: "Sam Rivera", Email: "sam@example.com"}

	for _, tc := range []struct {
		status, action, href string
		post                 bool
	}{
		{status: "open", action: "Request access", post: true},
		{status: "pending", action: "Check approval", href: link.Path()},
		{status: "approved", action: "Open shared resource", href: "/packing-session/trip-1"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			doc := renderDoc(t, JoinPage(link, tc.status, profile, "csrf-token"))

			if len(findAll(doc, byTag("script"))) != 0 {
				t.Error("join page loads scripts")
			}
			if heads := findAll(doc, byClass("page-head")); len(heads) != 1 {
				t.Fatalf("page heads %d, want 1", len(heads))
			}
			if len(findAll(doc, byTag("h1"))) != 1 {
				t.Error("join page does not have one h1")
			}
			referrer := findAll(doc, func(n *html.Node) bool {
				name, _ := attr(n, "name")
				content, _ := attr(n, "content")
				return n.Data == "meta" && name == "referrer" && content == "no-referrer"
			})
			if len(referrer) != 1 {
				t.Error("join page lost its no-referrer meta")
			}

			actions := findAll(doc, func(n *html.Node) bool {
				return (n.Data == "a" || n.Data == "button") && hasClass(n, "btn")
			})
			if len(actions) != 1 || text(actions[0]) != tc.action {
				t.Fatalf("actions %v, want one reading %q", findAllText(doc), tc.action)
			}
			if tc.post {
				forms := findAll(doc, byTag("form"))
				if len(forms) != 1 {
					t.Fatalf("forms %d, want 1", len(forms))
				}
				if action, _ := attr(forms[0], "action"); action != link.Path() {
					t.Errorf("form action %q, want %q", action, link.Path())
				}
				csrf := findAll(forms[0], func(n *html.Node) bool {
					name, _ := attr(n, "name")
					return n.Data == "input" && name == "_csrf"
				})
				if len(csrf) != 1 {
					t.Error("request form has no _csrf field")
				}
			} else if href, _ := attr(actions[0], "href"); href != tc.href {
				t.Errorf("action href %q, want %q", href, tc.href)
			}
		})
	}
}

// The edit item fallback leads back to its list through the shared page head.
func TestEditItemPageHeadLeadsBackToList(t *testing.T) {
	list := packing.NewList("owner", "Weekend car camping", "")
	item := packing.NewItem("Tent", "Shelter")
	list.Items = []packing.PackingItem{item}

	doc := renderDoc(t, EditItemPage(list, packing.EditItemForm(list.ID, item), "csrf-token"))

	back := findAll(doc, byClass("back-link"))
	if len(back) != 1 {
		t.Fatalf("back links %d, want 1", len(back))
	}
	if href, _ := attr(back[0], "href"); href != "/packing-lists/"+list.ID {
		t.Errorf("back link href %q, want the list", href)
	}
	if heads := findAll(doc, byTag("h1")); len(heads) != 1 || text(heads[0]) != "Edit item" {
		t.Errorf("h1 %v, want one reading Edit item", heads)
	}
	meta := findAll(doc, byClass("page-meta"))
	if len(meta) != 1 || text(meta[0]) != "On Weekend car camping" {
		t.Errorf("meta %v, want the list name", meta)
	}
}
