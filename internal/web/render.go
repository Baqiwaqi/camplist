package web

import (
	"camplist/internal/views"
	"log"
	"net/http"

	"github.com/a-h/templ"
)

// render writes a component with the request path in context, so the header
// can mark the section the visitor is in.
//
// A boosted form that renders instead of redirecting is showing its own
// errors. htmx would push the POST URL, which has no GET route, so a reload or
// Back after it fails; HX-Push-Url: false keeps the address bar on the form page.
func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	if r.Header.Get("HX-Boosted") == "true" && r.Method != http.MethodGet {
		w.Header().Set("HX-Push-Url", "false")
	}
	if err := c.Render(views.WithPath(r.Context(), r.URL.Path), w); err != nil {
		log.Printf("error rendering template: %v", err)
	}
}
