// Disposable Azure experiment. Never used by the production entrypoint.
package main

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/web"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
	"github.com/gorilla/sessions"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var requests, dbRequests, ruMilli, connections, opened, published atomic.Int64
var drop atomic.Bool
var mu sync.Mutex
var listeners = map[chan struct{}]bool{}

type meter struct{}

func (meter) Do(r *policy.Request) (*http.Response, error) {
	res, err := r.Next()
	if res != nil {
		dbRequests.Add(1)
		n, _ := strconv.ParseFloat(res.Header.Get("x-ms-request-charge"), 64)
		ruMilli.Add(int64(n * 1000))
	}
	return res, err
}
func broadcast() {
	published.Add(1)
	if drop.Load() {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	for ch := range listeners {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

type capture struct {
	http.ResponseWriter
	status int
}

func (w *capture) WriteHeader(s int) { w.status = s; w.ResponseWriter.WriteHeader(s) }
func (w *capture) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	return w.ResponseWriter.Write(b)
}
func (w *capture) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func main() {
	secret := os.Getenv("PROBE_SECRET")
	if len(secret) < 32 {
		log.Fatal("missing probe secret")
	}
	owner := os.Getenv("PROBE_OWNER")
	member := owner + "-member"
	stranger := owner + "-stranger"
	credential, err := azcosmos.NewKeyCredential(os.Getenv("DB_KEY"))
	must(err)
	client, err := azcosmos.NewClientWithKey(os.Getenv("DB_URL"), credential, &azcosmos.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{meter{}}}, EnableContentResponseOnWrite: true})
	must(err)
	container, err := client.NewContainer("dev", "packing_list")
	must(err)
	store := packing.NewStore(container)
	login, err := auth.New(auth.Config{ClientID: "probe", ClientSecret: "probe", RedirectURL: "https://example.invalid/auth/callback", SessionKey: secret, CookieSecure: true})
	must(err)
	cookies := sessions.NewCookieStore([]byte(secret))
	cookies.Options = &sessions.Options{Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: 3600}
	// Deterministic fixture IDs permit restarts without re-seeding resources.
	tripID := owner + "-trip"
	listID := owner + "-list"
	app := web.Routes(web.Config{PackingStore: store, Auth: login})
	admin := func(r *http.Request) bool {
		return subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Probe-Secret")), []byte(secret)) == 1
	}
	stream := login.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && origin != "https://"+r.Host {
			http.Error(w, "origin denied", 403)
			return
		}
		actor := auth.Subject(r.Context())
		if _, err := store.GetPackingSession(r.Context(), tripID, actor); err != nil {
			http.Error(w, "access denied", 403)
			return
		}
		ses, _ := cookies.Get(r, auth.SESSION_COOKIE_KEY)
		expires, _ := ses.Values["probeExpires"].(int64)
		rc := http.NewResponseController(w)
		if err := rc.SetWriteDeadline(time.Time{}); err != nil {
			http.Error(w, "stream unsupported", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Accel-Buffering", "no")
		send := func(event string) bool {
			rc.SetWriteDeadline(time.Now().Add(5 * time.Second))
			_, err := fmt.Fprintf(w, "event: %s\ndata: {}\n\n", event)
			if err == nil {
				err = rc.Flush()
			}
			rc.SetWriteDeadline(time.Time{})
			return err == nil
		}
		ch := make(chan struct{}, 1)
		mu.Lock()
		listeners[ch] = true
		mu.Unlock()
		connections.Add(1)
		opened.Add(1)
		defer func() { mu.Lock(); delete(listeners, ch); mu.Unlock(); connections.Add(-1) }()
		if !send("ready") {
			return
		}
		recheckSeconds, _ := strconv.Atoi(os.Getenv("PROBE_RECHECK_SECONDS"))
		if recheckSeconds < 15 {
			recheckSeconds = 15
		}
		nextCheck := time.Now().Add(time.Duration(recheckSeconds) * time.Second)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		deadline := time.NewTimer(12 * time.Minute)
		defer deadline.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-deadline.C:
				return
			case <-ticker.C:
				if time.Now().Unix() >= expires {
					send("signin")
					return
				}
				if !time.Now().Before(nextCheck) {
					if _, err := store.GetPackingSession(r.Context(), tripID, actor); err != nil {
						send("access_removed")
						return
					}
					nextCheck = time.Now().Add(time.Duration(recheckSeconds) * time.Second)
				}
				if r.URL.Query().Get("silent") != "1" {
					rc.SetWriteDeadline(time.Now().Add(5 * time.Second))
					fmt.Fprint(w, ": heartbeat\n\n")
					if rc.Flush() != nil {
						return
					}
					rc.SetWriteDeadline(time.Time{})
				}
			case <-ch:
				if time.Now().Unix() >= expires {
					send("signin")
					return
				}
				if _, err := store.GetPackingSession(r.Context(), tripID, actor); err != nil {
					send("access_removed")
					return
				}
				if !send("changed") {
					return
				}
			}
		}
	}))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			requests.Add(1)
		}
		if strings.HasPrefix(r.URL.Path, "/__probe/") {
			if !admin(r) {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			switch r.URL.Path {
			case "/__probe/login":
				actor := owner
				switch r.URL.Query().Get("account") {
				case "member":
					actor = member
				case "stranger":
					actor = stranger
				}
				ttl := 3600
				if n, e := strconv.Atoi(r.URL.Query().Get("ttl")); e == nil && n > 0 {
					ttl = n
				}
				ses, _ := cookies.Get(r, auth.SESSION_COOKIE_KEY)
				ses.Options = &sessions.Options{Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: ttl}
				ses.Values[auth.USER_ID_KEY] = actor
				ses.Values[auth.USER_NAME_KEY] = "Probe " + r.URL.Query().Get("account")
				ses.Values["probeExpires"] = time.Now().Unix() + int64(ttl)
				must(ses.Save(r, w))
				json.NewEncoder(w).Encode(map[string]string{"trip": tripID})
				return
			case "/__probe/seed":
				list := packing.NewList(owner, "SSE investigation", "")
				list.ID = listID
				list.Items = []packing.PackingItem{packing.NewItem("Tent", "Shared"), packing.NewItem("Stove", "Kitchen")}
				must(store.SavePackingList(r.Context(), list))
				trip := packing.NewPackingSession(list)
				trip.ID = tripID
				trip.Sharing.Members = map[string]packing.Member{member: {Subject: member, Name: "Probe member"}}
				data, _ := json.Marshal(trip)
				_, err := container.CreateItem(r.Context(), azcosmos.NewPartitionKeyString(owner), data, nil)
				must(err)
				ref := map[string]string{"id": "shared:packing-session:" + tripID, "userId": member, "type": "shared-reference", "kind": "packing-session", "resourceId": tripID, "owner": owner}
				data, _ = json.Marshal(ref)
				_, err = container.CreateItem(r.Context(), azcosmos.NewPartitionKeyString(member), data, nil)
				must(err)
				json.NewEncoder(w).Encode(map[string]string{"trip": tripID})
				return
			case "/__probe/drop":
				drop.Store(r.URL.Query().Get("value") == "1")
				w.WriteHeader(204)
				return
			case "/__probe/revoke":
				err := store.RemoveMember(r.Context(), "packing-session", tripID, owner, member)
				if err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
				broadcast()
				w.WriteHeader(204)
				return
			case "/__probe/stats":
				json.NewEncoder(w).Encode(map[string]any{"requests": requests.Load(), "dbRequests": dbRequests.Load(), "ru": float64(ruMilli.Load()) / 1000, "connections": connections.Load(), "opened": opened.Load(), "published": published.Load(), "replica": os.Getenv("CONTAINER_APP_REPLICA_NAME")})
				return
			case "/__probe/cleanup":
				count := 0
				for _, actor := range []string{owner, member, stranger} {
					pager := container.NewQueryItemsPager("SELECT c.id FROM c WHERE c.userId = @user", azcosmos.NewPartitionKeyString(actor), &azcosmos.QueryOptions{QueryParameters: []azcosmos.QueryParameter{{Name: "@user", Value: actor}}})
					for pager.More() {
						page, err := pager.NextPage(r.Context())
						must(err)
						for _, raw := range page.Items {
							var doc struct {
								ID string `json:"id"`
							}
							must(json.Unmarshal(raw, &doc))
							_, err := container.DeleteItem(r.Context(), azcosmos.NewPartitionKeyString(actor), doc.ID, nil)
							must(err)
							count++
						}
					}
				}
				json.NewEncoder(w).Encode(map[string]int{"deleted": count})
				return
			}
			http.NotFound(w, r)
			return
		}
		// Enforce a short signed expiry for controlled session-expiration tests.
		ses, _ := cookies.Get(r, auth.SESSION_COOKIE_KEY)
		expires, _ := ses.Values["probeExpires"].(int64)
		if expires > 0 && time.Now().Unix() >= expires && strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(401)
			fmt.Fprint(w, `{"code":"signin"}`)
			return
		}
		if r.URL.Path == "/api/probe/events" {
			stream.ServeHTTP(w, r)
			return
		}
		cw := &capture{ResponseWriter: w}
		app.ServeHTTP(cw, r)
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/sync") && cw.status >= 200 && cw.status < 300 {
			broadcast()
		}
	})
	// Same CSRF wrapper as production. Own hostname supplied by browser requests.
	root := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/__probe/") {
			handler.ServeHTTP(w, r)
			return
		}
		web.CSRFProtection([]byte(secret), true, r.Host)(handler).ServeHTTP(w, r)
	})
	server := &http.Server{Addr: ":3000", Handler: root, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		server.Shutdown(c)
	}()
	log.Print("disposable stream probe started")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
