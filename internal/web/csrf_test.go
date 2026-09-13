package web

import (
	"camplist/internal/packing"
	"github.com/gorilla/csrf"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestIdentityTokenCanBeUsedAtSignoutPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/identity", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, csrf.Token(r)) })
	mux.HandleFunc("POST /auth/signout", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	server := httptest.NewUnstartedServer(nil)
	host := server.Listener.Addr().String()
	server.Config.Handler = CSRFProtection([]byte("01234567890123456789012345678901"), false, host)(mux)
	server.Start()
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	legacyURL, _ := url.Parse(server.URL)
	jar.SetCookies(legacyURL, []*http.Cookie{{Name: "_gorilla_csrf", Value: "legacy-scoped-cookie", Path: "/auth"}})
	client := &http.Client{Jar: jar}
	response, err := client.Get(server.URL + "/api/identity")
	if err != nil {
		t.Fatal(err)
	}
	token, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest("POST", server.URL+"/auth/signout", nil)
	request.Header.Set("X-CSRF-Token", string(token))
	request.Header.Set("Origin", server.URL)
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 204 {
		t.Fatalf("identity token rejected on signout path: %d", response.StatusCode)
	}
	target, _ := url.Parse(server.URL + "/auth/signout")
	found := false
	for _, cookie := range jar.Cookies(target) {
		if cookie.Name == "camplist-csrf" {
			found = true
		}
	}
	if !found {
		t.Fatal("CSRF cookie missing at signout path")
	}
}

func TestNoReferrerFormsRequireSameOriginMetadataAndValidToken(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			io.WriteString(w, csrf.Token(r))
			return
		}
		w.WriteHeader(204)
	})
	protected := CSRFProtection([]byte("01234567890123456789012345678901"), false, "camplist.test")(handler)
	get := httptest.NewRecorder()
	protected.ServeHTTP(get, httptest.NewRequest("GET", "https://camplist.test/join/example", nil))
	token := get.Body.String()
	for _, tc := range []struct {
		site, token string
		want        int
	}{{"same-origin", token, 204}, {"cross-site", token, 403}, {"", token, 403}, {"same-origin", "", 403}} {
		request := httptest.NewRequest("POST", "https://camplist.test/join/example", nil)
		request.Header.Set("Origin", "null")
		request.Header.Set("Sec-Fetch-Site", tc.site)
		request.Header.Set("X-CSRF-Token", tc.token)
		for _, cookie := range get.Result().Cookies() {
			request.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		protected.ServeHTTP(response, request)
		if response.Code != tc.want {
			t.Fatalf("site %q token-present %v: got %d want %d", tc.site, tc.token != "", response.Code, tc.want)
		}
	}
}

func TestCSRFFailureIsMarkedForTheErrorToast(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	protected := CSRFProtection([]byte("01234567890123456789012345678901"), false, "camplist.test")(handler)
	for _, htmx := range []bool{true, false} {
		request := httptest.NewRequest("POST", "https://camplist.test/packing-lists", nil)
		request.Header.Set("Origin", "https://camplist.test")
		if htmx {
			request.Header.Set("HX-Request", "true")
		}
		response := httptest.NewRecorder()
		protected.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("htmx %v: got %d want 403", htmx, response.Code)
		}
		if got := response.Header().Get("X-Camplist-Error"); got != "csrf" {
			t.Errorf("htmx %v: X-Camplist-Error = %q, want csrf", htmx, got)
		}
		if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
			t.Errorf("htmx %v: Content-Type = %q, want text/plain", htmx, got)
		}
		if body := response.Body.String(); !strings.Contains(body, "Your session expired") {
			t.Errorf("htmx %v: body %q does not explain the expired session", htmx, body)
		}
	}
}

func TestStoreErrorsStayPlainTextWithoutTheCSRFMarker(t *testing.T) {
	response := httptest.NewRecorder()
	storeError(response, packing.ErrForbidden, "Could not save.")
	if response.Code != http.StatusForbidden || response.Header().Get("X-Camplist-Error") != "" {
		t.Fatalf("got %d with marker %q", response.Code, response.Header().Get("X-Camplist-Error"))
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", got)
	}
	if body := strings.TrimSpace(response.Body.String()); body != "Only the owner can do that." {
		t.Errorf("body = %q", body)
	}
}
