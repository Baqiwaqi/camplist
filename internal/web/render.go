package web

import (
	"camplist/internal/views"
	"log"
	"net/http"

	"github.com/a-h/templ"
)

// render writes a component with the request path in context, so the header
// can mark the section the visitor is in.
func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	if err := c.Render(views.WithPath(r.Context(), r.URL.Path), w); err != nil {
		log.Printf("error rendering template: %v", err)
	}
}
