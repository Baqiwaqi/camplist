package web

import (
	"github.com/gorilla/csrf"
	"net/http"
)

// CSRFProtection is shared by the production server and browser validation.
// A new name avoids older, path-scoped cookies shadowing the root cookie.
func CSRFProtection(key []byte, secure bool, trustedHost string) func(http.Handler) http.Handler {
	return csrf.Protect(key, csrf.Path("/"), csrf.CookieName("camplist-csrf"), csrf.Secure(secure), csrf.FieldName("_csrf"), csrf.TrustedOrigins([]string{trustedHost}))
}
