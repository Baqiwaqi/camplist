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
