package web

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/testsupport"

	"github.com/gorilla/sessions"
)

// The inline edit flow through the real router, auth middleware and CSRF protection.
func TestEditItemThroughRouter(t *testing.T) {
	key := "01234567890123456789012345678901"
	a, err := auth.New(auth.Config{ClientID: "id", ClientSecret: "secret", RedirectURL: "http://localhost/auth/callback", SessionKey: key})
	if err != nil {
		t.Skipf("auth setup needs network for OIDC discovery: %v", err)
	}
	store := packing.NewStore(testsupport.NewDocuments())
	list := packing.NewList("user", "Camping", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter")}
	if err := store.SavePackingList(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateInvitation(context.Background(), "packing-list", list.ID, "user"); err != nil {
		t.Fatal(err)
	}
	item := list.Items[0]

	// Same CSRF setup as cmd/web: the app trusts its own host as the request origin.
	routes := Routes(Config{PackingStore: store, Auth: a})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		CSRFProtection([]byte(key), false, r.Host)(routes).ServeHTTP(w, r)
	}))
	defer srv.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	base, _ := url.Parse(srv.URL)

	// Sign in the way the OAuth callback does: a session cookie holding the user id.
	cookies := sessions.NewCookieStore([]byte(key))
	cookies.Options = &sessions.Options{Path: "/", MaxAge: 600}
	seed := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	ses, _ := cookies.Get(seed, auth.SESSION_COOKIE_KEY)
	ses.Values[auth.USER_ID_KEY] = "user"
	ses.Values[auth.USER_NAME_KEY] = "Sam"
	if err := ses.Save(seed, rec); err != nil {
		t.Fatal(err)
	}
	jar.SetCookies(base, rec.Result().Cookies())

	do := func(method, path string, body url.Values, htmx bool) (*http.Response, string) {
		var reader io.Reader
		if body != nil {
			reader = strings.NewReader(body.Encode())
		}
		r, _ := http.NewRequest(method, srv.URL+path, reader)
		if body != nil {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			r.Header.Set("Origin", srv.URL)
			r.Header.Set("Referer", srv.URL+"/")
		}
		if htmx {
			r.Header.Set("HX-Request", "true")
		}
		res, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(res.Body)
		res.Body.Close()
		return res, string(b)
	}

	res, page := do("GET", "/packing-list/"+list.ID, nil, false)
	if res.StatusCode != http.StatusOK || !strings.Contains(page, "/items/"+item.ID+"/edit") {
		t.Fatalf("details page: %d, edit link present: %v", res.StatusCode, strings.Contains(page, "/items/"))
	}
	token := regexp.MustCompile(`name="_csrf" value="([^"]+)"`).FindStringSubmatch(page)
	if token == nil {
		t.Fatal("no CSRF token on the details page")
	}

	res, fragment := do("GET", "/packing-list/"+list.ID+"/items/"+item.ID+"/edit", nil, true)
	if res.StatusCode != http.StatusOK || res.Header.Get("HX-Redirect") != "" || strings.Contains(fragment, "<html") || !strings.Contains(fragment, `value="Tent"`) {
		t.Fatalf("edit fragment: %d redirect=%q body=%.160s", res.StatusCode, res.Header.Get("HX-Redirect"), fragment)
	}

	revision := regexp.MustCompile(`name="revision" value="([^"]+)"`).FindStringSubmatch(fragment)
	if revision == nil {
		t.Fatal("missing displayed revision")
	}
	res, row := do("POST", "/packing-list/"+list.ID+"/edit-item/"+item.ID, url.Values{"_csrf": {token[1]}, "name": {"Big tent"}, "category": {"Shelter"}, "revision": {html.UnescapeString(revision[1])}}, true)
	if res.StatusCode != http.StatusOK || !strings.Contains(row, "Big tent") || strings.Contains(row, "<form") {
		t.Fatalf("save: %d body=%.160s", res.StatusCode, row)
	}

	_, page = do("GET", "/packing-list/"+list.ID, nil, false)
	if !strings.Contains(page, "Big tent") {
		t.Fatal("edited name not shown on the details page")
	}

	res, _ = do("GET", "/packing-list/"+list.ID+"/items/"+item.ID+"/edit", nil, false)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("fallback edit page: %d", res.StatusCode)
	}
	headersMatch := regexp.MustCompile(`hx-headers="([^"]+)"`).FindStringSubmatch(fragment)
	if headersMatch == nil {
		t.Fatal("missing delete headers in the edit row")
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(html.UnescapeString(headersMatch[1])), &headers); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("DELETE", srv.URL+"/packing-list/"+list.ID+"/remove-item/"+item.ID, nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	// The CSRF header comes from Layout's body, and static/revision.js sends
	// the list revision the save swapped into the page.
	req.Header.Set("X-CSRF-Token", html.UnescapeString(token[1]))
	req.Header.Set("X-Camplist-Revision", listRevision(t, row))
	req.Header.Set("Origin", srv.URL)
	req.Header.Set("Referer", srv.URL+"/")
	req.Header.Set("HX-Request", "true")
	deleted, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer deleted.Body.Close()
	if deleted.StatusCode >= 400 {
		t.Fatalf("delete immediately after edit: %d", deleted.StatusCode)
	}
	current, err := store.GetPackingList(context.Background(), list.ID, "user")
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Items) != 0 {
		t.Fatal("edited item was not deleted")
	}

}
