package views

import (
	"bytes"
	"camplist/internal/auth"
	"camplist/internal/packing"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"golang.org/x/net/html"
)

func TestListDeleteURL(t *testing.T) {
	list := packing.NewList("user", "Camping", "")
	var out bytes.Buffer
	if err := PackingListPage("Lists", []packing.PackingList{list}, "token").Render(context.WithValue(context.Background(), auth.USER_ID_KEY, "user"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `hx-delete="/packing-lists/`+list.ID+`"`) {
		t.Fatal("delete URL does not match route")
	}
	card := `id="` + PackingListCardID(list.ID) + `"`
	for _, want := range []string{card, `hx-target="#` + PackingListCardID(list.ID) + `"`, `hx-swap="delete"`, `hx-sync="this:drop"`, `id="packing-lists-empty"`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("lists page missing %q for in-place delete", want)
		}
	}
}

func TestSessionsCanBeReopened(t *testing.T) {
	session := packing.NewPackingSession(packing.NewList("user", "Camping", ""))
	var out bytes.Buffer
	if err := PackingSessionsOverviewPage("Sessions", []packing.PackingSession{session}, 0, "token").Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `href="/trips/`+session.ID+`"`) {
		t.Fatal("no link to reopen session")
	}
}

func TestTripPagesSwitchWithLinkTabsAndCounts(t *testing.T) {
	session := packing.NewPackingSession(packing.NewList("user", "Camping", ""))
	var overview bytes.Buffer
	if err := PackingSessionsOverviewPage("Sessions", nil, 2, "token").Render(context.Background(), &overview); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<a class="tab" href="/trips" aria-current="page">In progress <span class="tab-count">0</span></a>`,
		`<a class="tab" href="/trips/archive">Archive <span class="tab-count">2</span></a>`,
	} {
		if !strings.Contains(overview.String(), want) {
			t.Errorf("overview missing %q", want)
		}
	}

	var archive bytes.Buffer
	if err := ArchivedPackingSessionsPage("Trip archive", []packing.PackingSession{session}, 3, "token").Render(context.Background(), &archive); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<a class="tab" href="/trips">In progress <span class="tab-count">3</span></a>`,
		`<a class="tab" href="/trips/archive" aria-current="page">Archive <span class="tab-count">1</span></a>`,
		session.DisplayName(),
	} {
		if !strings.Contains(archive.String(), want) {
			t.Errorf("archive missing %q", want)
		}
	}
}

func TestTripCardTitleDropsTheStartDateTheMetaLineShows(t *testing.T) {
	session := packing.NewPackingSession(packing.NewList("user", "Camping", ""))
	date := session.CreatedAt.Format("Jan 2, 2006")
	for name, want := range map[string]string{
		"Camping – " + date: "Camping",
		"Lake weekend":      "Lake weekend",
		date:                date,
		" – " + date:        " – " + date,
	} {
		session.Name = name
		if got := tripCardTitle(session); got != want {
			t.Errorf("tripCardTitle(%q) = %q, want %q", name, got, want)
		}
	}
	session.Name = "Camping – " + date
	var out bytes.Buffer
	if err := PackingSessionCards([]packing.PackingSession{session}, false, "token").Render(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), ">Camping</a></h2>") || !strings.Contains(out.String(), "Started <time") {
		t.Errorf("card does not show the short title and the start date: %s", out.String())
	}
}

// A card opens through one link, its title; More stays a separate button and
// the status (tags, then progress) closes the card.
func TestCardsOpenThroughTheirTitleLink(t *testing.T) {
	list := packing.NewList("user", "Camping", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter")}
	list.Sharing.Members = map[string]packing.Member{"friend": {Subject: "friend", Name: "Sam"}}
	trip := packing.NewPackingSession(list)
	trip.Sharing.Members = list.Sharing.Members
	trip.List.Items[0].Checked = true
	for name, tc := range map[string]struct {
		card   templ.Component
		href   string
		status string
	}{
		"trip": {PackingSessionCards([]packing.PackingSession{trip}, false, "token"), "/trips/" + trip.ID, "All packed"},
		"list": {PackingListCard(list, "token"), "/packing-lists/" + list.ID, ""},
	} {
		t.Run(name, func(t *testing.T) {
			cards := findAll(renderDoc(t, tc.card), byTag("article"))
			if len(cards) != 1 {
				t.Fatalf("cards %d, want 1", len(cards))
			}
			card := cards[0]
			// Everything in the card in document order: the menu must follow
			// the link, so it paints above the link's cover.
			all := findAll(card, func(*html.Node) bool { return true })
			var links []*html.Node
			link, menu := -1, -1
			for i, n := range all {
				if n.Data == "a" && !hasAttr("role", "menuitem")(n) {
					links = append(links, n)
					link = i
				}
				if hasClass(n, "menu") && menu < 0 {
					menu = i
				}
			}
			if len(links) != 1 || !hasClass(links[0], "card-link") || text(links[0]) != "Camping" {
				t.Fatalf("card links %v, want the title as the one card-link", links)
			}
			if href, _ := attr(links[0], "href"); href != tc.href {
				t.Errorf("card link href %q, want %q", href, tc.href)
			}
			if menu < link {
				t.Error("More menu comes before the title link")
			}
			if buttons := findAll(card, byClass("btn")); len(buttons) != 1 || text(buttons[0]) != "More" {
				t.Errorf("card buttons %d, want only More", len(buttons))
			}
			if tags := findAll(card, byClass("tag")); len(tags) == 0 || text(tags[0]) != "Shared" {
				t.Error("shared card does not start its tag row with Shared")
			}
			if tc.status != "" {
				if labels := findAll(card, byClass("progress-label")); len(labels) != 1 || text(labels[0]) != tc.status {
					t.Errorf("status %v, want %q", labels, tc.status)
				}
			}
		})
	}
}
