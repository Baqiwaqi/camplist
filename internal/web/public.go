package web

import (
	"camplist/internal/auth"
	"camplist/internal/views"
	"net/http"
)

// HomePage is the landing page for visitors. Members see their lists instead,
// so the brand link and bookmarks keep working after signing in.
func (h *handler) HomePage(w http.ResponseWriter, r *http.Request) {
	if _, err := auth.UserID(r.Context()); err == nil {
		h.MainPage(w, r)
		return
	}
	render(w, r, views.Landing())
}

// DemoPage is a packing session that lives only in the browser. Nothing is
// stored, so it needs neither an account nor the database.
func (h *handler) DemoPage(w http.ResponseWriter, r *http.Request) {
	render(w, r, views.DemoPage())
}
