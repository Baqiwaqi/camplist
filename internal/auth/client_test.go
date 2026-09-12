package auth

import (
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestMissingSessionNavigatesToLogin(t *testing.T) {
	a := &Auth{cookieStore: sessions.NewCookieStore([]byte("01234567890123456789012345678901"))}
	for _, htmx := range []bool{false, true} {
		r := httptest.NewRequest("POST", "/packing-session/set-item", nil)
		if htmx {
			r.Header.Set("HX-Request", "true")
		}
		w := httptest.NewRecorder()
		a.RequireAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("unauthenticated request reached handler") })).ServeHTTP(w, r)
		if htmx {
			if w.Code != http.StatusOK || w.Header().Get("HX-Redirect") != "/login" {
				t.Error("HTMX must navigate the full page to login")
			}
		} else if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/login" {
			t.Error("missing normal login redirect")
		}
	}
}

func TestExpiredAPISessionReturnsJSONInsteadOfLoginHTML(t *testing.T) {
	a := &Auth{cookieStore: sessions.NewCookieStore([]byte("01234567890123456789012345678901"))}
	for _, path := range []string{"/api/identity", "/api/sessions/trip", "/api/sessions/trip/sync"} {
		w := httptest.NewRecorder()
		a.RequireAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unauthenticated API reached handler") })).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 401 || w.Header().Get("Location") != "" || w.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
}

func TestSignoutClearsAnInvalidSessionCookie(t *testing.T) {
	a := &Auth{cookieStore: sessions.NewCookieStore([]byte("01234567890123456789012345678901"))}
	r := httptest.NewRequest("POST", "/auth/signout", nil)
	r.AddCookie(&http.Cookie{Name: SESSION_COOKIE_KEY, Value: "expired-or-invalid"})
	w := httptest.NewRecorder()
	a.DeleteHandler(w, r)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/login" {
		t.Fatalf("signout failed: %d", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) == 0 || cookies[0].MaxAge != -1 {
		t.Fatal("signout did not remove stale cookie")
	}
}

func TestInvitationLoginPreservesOnlyAnInternalReturnAndOffersAccountChoice(t *testing.T) {
	a := &Auth{cookieStore: sessions.NewCookieStore([]byte("01234567890123456789012345678901")), oauthCfg: &oauth2.Config{ClientID: "client", Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/auth"}}}
	path := "/join/owner/packing-session/trip/AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	w := httptest.NewRecorder()
	a.LoginHandler(w, httptest.NewRequest("GET", "/auth/login?switch=1&returnTo="+url.QueryEscape(path), nil))
	redirect, _ := url.Parse(w.Header().Get("Location"))
	if redirect.Query().Get("prompt") != "select_account" {
		t.Fatal("missing account choice")
	}
	r := httptest.NewRequest("GET", "/", nil)
	for _, cookie := range w.Result().Cookies() {
		r.AddCookie(cookie)
	}
	ses, _ := a.cookieStore.Get(r, SESSION_COOKIE_KEY)
	if ses.Values["returnTo"] != path {
		t.Fatal("invitation lost")
	}
	w = httptest.NewRecorder()
	a.LoginHandler(w, httptest.NewRequest("GET", "/auth/login?returnTo=https://evil.example", nil))
	r = httptest.NewRequest("GET", "/", nil)
	for _, cookie := range w.Result().Cookies() {
		r.AddCookie(cookie)
	}
	ses, _ = a.cookieStore.Get(r, SESSION_COOKIE_KEY)
	if ses.Values["returnTo"] != "/" {
		t.Fatal("external return accepted")
	}
}
