package auth

import (
	"github.com/gorilla/sessions"
	"net/http"
	"net/http/httptest"
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
