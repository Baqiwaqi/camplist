package web

import (
	"github.com/go-chi/chi/v5/middleware"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

func requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasPrefix(path, "/join/") {
			path = "/join/[redacted]"
		}
		response := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		defer func() { log.Printf("%s %s %d", r.Method, path, response.Status()) }()
		next.ServeHTTP(response, r)
	})
}
func sharingProtection() func(http.Handler) http.Handler {
	type window struct {
		until time.Time
		count int
	}
	var mu sync.Mutex
	attempts := map[string]window{}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			join := strings.HasPrefix(r.URL.Path, "/join/")
			if join || strings.HasPrefix(r.URL.Path, "/sharing/") || strings.HasPrefix(r.URL.Path, "/auth/") {
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Referrer-Policy", "no-referrer")
			}
			if join {
				host, _, _ := net.SplitHostPort(r.RemoteAddr)
				if host == "" {
					host = r.RemoteAddr
				}
				now := time.Now()
				mu.Lock()
				for key, value := range attempts {
					if !value.until.After(now) {
						delete(attempts, key)
					}
				}
				value := attempts[host]
				if value.until.IsZero() {
					value.until = now.Add(time.Minute)
				}
				value.count++
				allowed := value.count <= 30 && len(attempts) < 4096
				if allowed {
					attempts[host] = value
				}
				mu.Unlock()
				if !allowed {
					w.Header().Set("Retry-After", "60")
					http.Error(w, "Please wait a minute before trying again.", 429)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
