package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/testsupport"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func sharingRequest(method, path, actor string, params map[string]string, form url.Values) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	route := chi.NewRouteContext()
	for key, value := range params {
		route.URLParams.Add(key, value)
	}
	return r.WithContext(context.WithValue(context.WithValue(r.Context(), chi.RouteCtxKey, route), auth.USER_ID_KEY, actor))
}

func TestListApprovalAsksToShareTripsWithTheSameAccount(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Weekend", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "")}
	store.SavePackingList(ctx, list)
	trip, err := store.CreatePackingSession(ctx, list.ID, "owner")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := store.CreatePackingSession(ctx, list.ID, "owner")
	link, _ := store.CreateInvitation(ctx, "packing-list", list.ID, "owner")
	store.RequestAccess(ctx, link, packing.Member{Subject: "guest", Name: "Robin"})
	h := handler{packingStore: store}
	params := map[string]string{"kind": "packing-list", "id": list.ID, "hash": link.Hash()}
	base := "/sharing/packing-list/" + list.ID

	w := httptest.NewRecorder()
	h.SharingPage(w, sharingRequest("GET", base, "owner", params, nil))
	if !strings.Contains(w.Body.String(), `hx-get="`+base+"/invitations/"+link.Hash()+`/approve"`) {
		t.Fatalf("approval does not open the trip prompt:\n%s", w.Body.String())
	}

	r := sharingRequest("GET", base+"/invitations/"+link.Hash()+"/approve", "owner", params, nil)
	r.Header.Set("HX-Request", "true")
	w = httptest.NewRecorder()
	h.ApproveInvitationPage(w, r)
	body := w.Body.String()
	for _, want := range []string{"Also share trips with Robin?", `name="trip" value="` + trip.ID + `"`, `value="` + other.ID + `"`, `value="approve"`, "Cancel"} {
		if !strings.Contains(body, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(body, "<html") || strings.Contains(body, "checked") {
		t.Fatalf("prompt must be an unticked fragment:\n%s", body)
	}

	// Without htmx the prompt opens on the full page.
	w = httptest.NewRecorder()
	h.ApproveInvitationPage(w, sharingRequest("GET", base+"/invitations/"+link.Hash()+"/approve", "owner", params, nil))
	if !strings.Contains(w.Body.String(), "<html") || !strings.Contains(w.Body.String(), "Also share trips with Robin?") {
		t.Fatal("no-script fallback lost the prompt")
	}

	// The requester cannot open the owner's prompt.
	r = sharingRequest("GET", base+"/invitations/"+link.Hash()+"/approve", "guest", params, nil)
	r.Header.Set("HX-Request", "true")
	w = httptest.NewRecorder()
	h.ApproveInvitationPage(w, r)
	if w.Code == http.StatusOK || strings.Contains(w.Body.String(), trip.ID) {
		t.Fatalf("requester saw the trip prompt: %d", w.Code)
	}

	w = httptest.NewRecorder()
	h.DecideInvitation(w, sharingRequest("POST", base+"/invitations/"+link.Hash(), "owner", params, url.Values{"decision": {"approve"}, "trip": {trip.ID}}))
	if w.Code != http.StatusSeeOther {
		t.Fatalf("approve: %d %s", w.Code, w.Body.String())
	}
	if _, err := store.GetPackingSession(ctx, trip.ID, "guest"); err != nil {
		t.Fatalf("chosen trip not shared: %v", err)
	}
	if _, err := store.GetPackingSession(ctx, other.ID, "guest"); err == nil {
		t.Fatal("unchosen trip shared")
	}
	w = httptest.NewRecorder()
	h.SessionDetailsPage(w, sharingRequest("GET", "/trips/"+trip.ID, "guest", map[string]string{"id": trip.ID}, nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Tent") {
		t.Fatalf("recipient cannot open shared trip: %d", w.Code)
	}
}

func TestListApprovalWithoutTripsApprovesDirectly(t *testing.T) {
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Weekend", "")
	store.SavePackingList(ctx, list)
	link, _ := store.CreateInvitation(ctx, "packing-list", list.ID, "owner")
	store.RequestAccess(ctx, link, packing.Member{Subject: "guest", Name: "Robin"})
	h := handler{packingStore: store}
	params := map[string]string{"kind": "packing-list", "id": list.ID, "hash": link.Hash()}
	base := "/sharing/packing-list/" + list.ID

	w := httptest.NewRecorder()
	h.SharingPage(w, sharingRequest("GET", base, "owner", params, nil))
	body := w.Body.String()
	if strings.Contains(body, "/approve") || !strings.Contains(body, `name="decision" value="approve"`) {
		t.Fatalf("a list without trips should approve without asking:\n%s", body)
	}
	w = httptest.NewRecorder()
	h.DecideInvitation(w, sharingRequest("POST", base+"/invitations/"+link.Hash(), "owner", params, url.Values{"decision": {"approve"}}))
	if w.Code != http.StatusSeeOther {
		t.Fatalf("approve: %d %s", w.Code, w.Body.String())
	}
	if _, err := store.GetPackingList(ctx, list.ID, "guest"); err != nil {
		t.Fatalf("list not shared: %v", err)
	}

	// Trip choices only belong to list requests.
	trip, _ := store.CreatePackingSession(ctx, list.ID, "owner")
	tripLink, _ := store.CreateInvitation(ctx, "packing-session", trip.ID, "owner")
	w = httptest.NewRecorder()
	h.DecideInvitation(w, sharingRequest("POST", "/sharing/packing-session/"+trip.ID+"/invitations/"+tripLink.Hash(), "owner", map[string]string{"kind": "packing-session", "id": trip.ID, "hash": tripLink.Hash()}, url.Values{"decision": {"revoke"}, "trip": {trip.ID}}))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("trip choice on a trip invitation: %d", w.Code)
	}
}

// sharedList returns a store with a list, a pending request from Robin and an
// approved member Sam, plus the pending link.
func sharedList(t *testing.T) (*packing.Store, packing.PackingList, packing.InvitationLink) {
	t.Helper()
	ctx := context.Background()
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("owner", "Weekend", "")
	store.SavePackingList(ctx, list)
	member, _ := store.CreateInvitation(ctx, "packing-list", list.ID, "owner")
	store.RequestAccess(ctx, member, packing.Member{Subject: "sam", Name: "Sam", Email: "sam@example.com"})
	if err := store.DecideInvitation(ctx, "packing-list", list.ID, "owner", member.Hash(), true); err != nil {
		t.Fatal(err)
	}
	link, _ := store.CreateInvitation(ctx, "packing-list", list.ID, "owner")
	store.RequestAccess(ctx, link, packing.Member{Subject: "guest", Name: "Robin", Email: "robin@example.com"})
	return store, list, link
}

func TestCreateInvitationSwapsInviteCardAndSupportsNormalForms(t *testing.T) {
	for _, htmx := range []bool{true, false} {
		t.Run(fmt.Sprint(htmx), func(t *testing.T) {
			store, list, _ := sharedList(t)
			h := handler{packingStore: store}
			params := map[string]string{"kind": "packing-list", "id": list.ID}
			r := sharingRequest("POST", "/sharing/packing-list/"+list.ID+"/invitations", "owner", params, url.Values{})
			if htmx {
				r.Header.Set("HX-Request", "true")
			}
			w := httptest.NewRecorder()
			h.CreateInvitation(w, r)
			body := w.Body.String()
			view, _ := store.GetSharing(context.Background(), "packing-list", list.ID, "owner")
			if w.Code != http.StatusOK || len(view.Sharing.Invitations) != 3 {
				t.Fatalf("create: %d, %d invitations", w.Code, len(view.Sharing.Invitations))
			}
			created := view.Sharing.Invitations[2]
			for _, want := range []string{`id="invite-card"`, `id="invitation-link"`, "/join/owner/packing-list/" + list.ID + "/", `id="invitation-` + created.Hash + `"`} {
				if !strings.Contains(body, want) {
					t.Errorf("missing %q", want)
				}
			}
			if htmx {
				if strings.Contains(body, "<html") || strings.Contains(body, "Robin") || !strings.Contains(body, `hx-swap-oob="beforeend:#invitation-rows"`) {
					t.Fatalf("create should return the card and the new row only:\n%s", body)
				}
			} else if !strings.Contains(body, "<html") || !strings.Contains(body, "Robin") {
				t.Fatalf("normal form lost the sharing page:\n%s", body)
			}
		})
	}
}

func TestDecideInvitationSwapsRowAndSupportsNormalForms(t *testing.T) {
	for _, decision := range []string{"approve", "revoke"} {
		for _, htmx := range []bool{true, false} {
			t.Run(decision+fmt.Sprint(htmx), func(t *testing.T) {
				store, list, link := sharedList(t)
				h := handler{packingStore: store}
				base := "/sharing/packing-list/" + list.ID
				params := map[string]string{"kind": "packing-list", "id": list.ID, "hash": link.Hash()}
				r := sharingRequest("POST", base+"/invitations/"+link.Hash(), "owner", params, url.Values{"decision": {decision}})
				if htmx {
					r.Header.Set("HX-Request", "true")
				}
				w := httptest.NewRecorder()
				h.DecideInvitation(w, r)
				_, err := store.GetPackingList(context.Background(), list.ID, "guest")
				if (decision == "approve") != (err == nil) {
					t.Fatalf("%s: guest access err %v", decision, err)
				}
				if !htmx {
					if w.Code != http.StatusSeeOther || w.Header().Get("Location") != base {
						t.Fatalf("normal form missing redirect: %d", w.Code)
					}
					return
				}
				body := w.Body.String()
				status := map[string]string{"approve": "approved", "revoke": "revoked"}[decision]
				for _, want := range []string{`id="invitation-` + link.Hash() + `"`, `tabindex="-1" autofocus`, status + " · Expires"} {
					if !strings.Contains(body, want) {
						t.Errorf("missing %q", want)
					}
				}
				if strings.Contains(body, "<html") || strings.Contains(body, `name="decision"`) {
					t.Fatalf("decision should return the decided row:\n%s", body)
				}
				members := strings.Contains(body, `id="members" class="card mt-8" hx-swap-oob="true"`)
				if members != (decision == "approve") || members && (!strings.Contains(body, "Robin · robin@example.com") || !strings.Contains(body, "Sam")) {
					t.Fatalf("member card out of band only after approval:\n%s", body)
				}
			})
		}
	}
}

func TestRemoveMemberConfirmsAndSupportsNormalForms(t *testing.T) {
	store, list, _ := sharedList(t)
	h := handler{packingStore: store}
	base := "/sharing/packing-list/" + list.ID
	params := map[string]string{"kind": "packing-list", "id": list.ID}
	w := httptest.NewRecorder()
	h.SharingPage(w, sharingRequest("GET", base, "owner", params, nil))
	for _, want := range []string{`hx-post="` + base + `/members/sam/remove"`, `hx-confirm="Remove access for Sam?"`, `data-confirm-action="Remove access"`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("remove missing %q", want)
		}
	}
	w = httptest.NewRecorder()
	h.SharingPage(w, sharingRequest("GET", base, "sam", params, nil))
	for _, want := range []string{`hx-post="` + base + `/members/sam/remove"`, `hx-confirm="Leave this shared resource?"`, `data-confirm-action="Leave"`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Errorf("leave missing %q", want)
		}
	}

	for _, htmx := range []bool{true, false} {
		for _, actor := range []string{"owner", "sam"} {
			t.Run(actor+fmt.Sprint(htmx), func(t *testing.T) {
				store, list, _ := sharedList(t)
				h := handler{packingStore: store}
				base := "/sharing/packing-list/" + list.ID
				params := map[string]string{"kind": "packing-list", "id": list.ID, "subject": "sam"}
				r := sharingRequest("POST", base+"/members/sam/remove", actor, params, url.Values{})
				if htmx {
					r.Header.Set("HX-Request", "true")
				}
				w := httptest.NewRecorder()
				h.RemoveMember(w, r)
				if _, err := store.GetPackingList(context.Background(), list.ID, "sam"); err == nil {
					t.Fatal("member kept access")
				}
				body := w.Body.String()
				switch {
				case !htmx:
					want := base
					if actor == "sam" {
						want = "/"
					}
					if w.Code != http.StatusSeeOther || w.Header().Get("Location") != want {
						t.Fatalf("normal form redirect: %d %q", w.Code, w.Header().Get("Location"))
					}
				case actor == "sam":
					if w.Header().Get("HX-Redirect") != "/" || body != "" {
						t.Fatalf("leaving should navigate home: %d %q", w.Code, body)
					}
				default:
					if !strings.Contains(body, `id="members"`) || !strings.Contains(body, `id="members-heading" tabindex="-1" autofocus`) || strings.Contains(body, "Sam") || strings.Contains(body, "<html") || strings.Contains(body, "hx-swap-oob") {
						t.Fatalf("removal should return the member card with focus:\n%s", body)
					}
				}
			})
		}
	}
}
