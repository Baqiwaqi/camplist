package validation

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/web"
	"context"
	"fmt"
	"github.com/gorilla/sessions"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// This server exists only in an opt-in test binary and binds loopback only.
// Google sign-in is replaced by a signed fixture cookie; production routes,
// authentication middleware, CSRF, templates and Cosmos store are unchanged.
func TestBrowserSimulation(t *testing.T) {
	if os.Getenv("CAMPLIST_BROWSER") != "1" {
		t.Skip("set CAMPLIST_BROWSER=1 to run the interactive local fixture")
	}
	owner := fmt.Sprintf("camplist-browser-%d", time.Now().UnixNano())
	store := validationStore(t, owner)
	t.Chdir("../..")
	ctx := context.Background()
	list := packing.NewList(owner, "Validation camping trip", "")
	list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shelter"), packing.NewItem("Stove", "Kitchen")}
	must(t, store.SavePackingList(ctx, list))
	trip, err := store.CreatePackingSession(ctx, list.ID, owner)
	must(t, err)
	const cookieKey = "camplist-validation-cookie-key-32"
	login, err := auth.New(auth.Config{ClientID: "validation", ClientSecret: "validation", RedirectURL: "http://camplist-validation.localhost:3001/auth/callback", SessionKey: cookieKey})
	must(t, err)
	app := web.Routes(web.Config{PackingStore: store, Auth: login})
	cookieStore := sessions.NewCookieStore([]byte(cookieKey))
	var offline atomic.Bool
	var offlineHost atomic.Value
	offlineHost.Store("")
	var server *http.Server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/__validation/account":
			account := r.URL.Query().Get("name")
			if account != "" {
				account = owner + "-other"
			}
			if account == "" {
				account = owner
			}
			session, _ := cookieStore.Get(r, auth.SESSION_COOKIE_KEY)
			session.Options = &sessions.Options{Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode}
			session.Values[auth.USER_ID_KEY] = account
			session.Values[auth.USER_NAME_KEY] = "Validation " + account
			if err := session.Save(r, w); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			http.Redirect(w, r, "/packing-session/"+trip.ID, 303)
			return
		case "/__validation/network":
			offlineHost.Store(r.URL.Query().Get("host"))
			offline.Store(r.URL.Query().Get("offline") == "1")
			w.WriteHeader(204)
			return
		case "/__validation/stop":
			w.WriteHeader(204)
			go server.Shutdown(context.Background())
			return
		}
		if offline.Load() && (offlineHost.Load().(string) == "" || offlineHost.Load().(string) == r.Host) && !strings.HasPrefix(r.URL.Path, "/__validation/") {
			connection, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				connection.Close()
			}
			return
		}
		app.ServeHTTP(w, r)
	})
	ownerHandler := web.CSRFProtection([]byte("camplist-validation-csrf-key-32!!"), false, "camplist-validation.localhost:3001")(handler)
	memberHandler := web.CSRFProtection([]byte("camplist-validation-csrf-key-32!!"), false, "camplist-member.localhost:3001")(handler)
	server = &http.Server{Addr: "127.0.0.1:3001", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == "camplist-member.localhost:3001" {
			memberHandler.ServeHTTP(w, r)
		} else {
			ownerHandler.ServeHTTP(w, r)
		}
	})}
	t.Log("Browser fixture: http://camplist-validation.localhost:3001/__validation/account")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		t.Fatal(err)
	}
}
