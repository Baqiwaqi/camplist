package web

import (
	"github.com/gorilla/csrf"
	"net/http"
)

// CSRFProtection is shared by the production server and browser validation.
// A new name avoids older, path-scoped cookies shadowing the root cookie.
func CSRFProtection(key []byte, secure bool, trustedHost string) func(http.Handler) http.Handler {
	protect := csrf.Protect(key, csrf.Path("/"), csrf.CookieName("camplist-csrf"), csrf.Secure(secure), csrf.FieldName("_csrf"), csrf.TrustedOrigins([]string{trustedHost}), csrf.ErrorHandler(http.HandlerFunc(csrfFailure)))
	return func(next http.Handler) http.Handler {
		guarded := protect(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// no-referrer navigation forms may have an opaque Origin. Fetch Metadata is
			// browser-controlled: only a same-origin request can supply this signal.
			// Keep Gorilla's cookie/token validation and reject null cross-site origins.
			if r.Header.Get("Origin") == "null" && r.Header.Get("Sec-Fetch-Site") == "same-origin" {
				r = r.Clone(r.Context())
				r.Header.Set("Origin", "https://"+r.Host)
			}
			guarded.ServeHTTP(w, r)
		})
	}
}

// csrfFailure marks the rejection so the error toast can tell an expired
// session apart from other 403s, whose plain-text reason it shows as sent.
func csrfFailure(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Camplist-Error", "csrf")
	http.Error(w, "Your session expired. Reload the page and try again.", http.StatusForbidden)
}
