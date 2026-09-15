package views

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"camplist/internal/auth"
	"camplist/internal/packing"
)

func renderWithPath(t *testing.T, path string) string {
	t.Helper()
	ctx := context.WithValue(context.Background(), auth.USER_ID_KEY, "user")
	ctx = context.WithValue(ctx, auth.USER_NAME_KEY, "Sam Camper")
	ctx = WithPath(ctx, path)
	var out bytes.Buffer
	if err := PackingSessionsOverviewPage("Sessions", []packing.PackingSession{}, 0, "token").Render(ctx, &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestHeaderMarksTheCurrentSection(t *testing.T) {
	body := renderWithPath(t, "/trips")
	if !strings.Contains(body, `href="/trips" aria-current="page"`) {
		t.Error("trips link is not marked current on the trips page")
	}
	if strings.Contains(body, `href="/packing-lists" aria-current="page"`) {
		t.Error("lists link is marked current on the sessions page")
	}
	body = renderWithPath(t, "/packing-lists/abc")
	if !strings.Contains(body, `href="/packing-lists" aria-current="page"`) {
		t.Error("lists link is not marked current on a list page")
	}
}

func TestAccountItemsSitBehindTheMemberName(t *testing.T) {
	body := renderWithPath(t, "/")
	for _, want := range []string{
		`aria-haspopup="menu"`,
		`>Sam Camper</button>`,
		`role="menuitem" href="/offline">Saved on this device</a>`,
		`class="option" role="menuitem">Sign out</button>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("header missing %q", want)
		}
	}
	if strings.Count(body, `data-signout`) != 2 {
		t.Errorf("expected the sign-out form in the menu and in the phone panel, found %d", strings.Count(body, "data-signout"))
	}
	if strings.Contains(body, `>On this device</a>`) {
		t.Error("the device page is still a primary link")
	}
}

func TestLayoutWiresTheErrorToastBeforeAlpineStarts(t *testing.T) {
	body := renderWithPath(t, "/trips")
	for _, want := range []string{
		`<script src="/static/request-error.js" defer></script>`,
		`x-data="requestError"`,
		`x-on:htmx:response-error.window="failed($event.detail.xhr)"`,
		`x-show="reload"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("layout is missing %s", want)
		}
	}
	if strings.Index(body, "/static/request-error.js") > strings.Index(body, "/static/alpine.min.js") {
		t.Error("request-error.js must load before Alpine starts")
	}
}

// A boosted form that re-renders with an error swaps in a new #nav-panel.
// Alpine hides it with an inline display, then htmx settles the same-id element
// back to the style attribute in the markup, so the markup itself must hide it.
func TestPhoneNavPanelStaysClosedAfterASwap(t *testing.T) {
	body := renderWithPath(t, "/packing-lists/new")
	start := strings.Index(body, `id="nav-panel"`)
	if start == -1 {
		t.Fatal("phone nav panel is missing")
	}
	tag := body[strings.LastIndex(body[:start], "<"):]
	tag = tag[:strings.Index(tag, ">")]
	if !strings.Contains(tag, `x-show="open"`) || !strings.Contains(tag, `style="display: none"`) {
		t.Errorf("phone nav panel must start hidden inline so an htmx settle keeps it closed: %s", tag)
	}
}
