package web

import (
	"camplist/internal/packing"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func Routes(store packing.Store) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	h := handler{packingStore: &store}

	// Pages
	r.Get("/", h.MainPage)
	r.Get("/sessions", h.SessionsPage)
	r.Get("/packing-list/new", h.NewListPage)
	r.Post("/packing-list/new", h.NewListHandler)

	r.Get("/packing-list/{id}/edit", h.EditListPage)
	r.Post("/packing-list/{id}/edit", h.EditListHandler)

	r.Get("/packing-list/{id}", h.ListDetailsPage)

	r.Post("/packing-list/{id}/add-item", h.AddItemHandler)
	r.Delete("/packing-list/{id}/remove-item/{itemId}", h.RemoveItemHandler)

	r.Delete("/packing-list/{id}", h.DeleteListHandler)

	r.Post("/packing-list/start-session", h.CreateSessionHandler)
	r.Get("/packing-session/{id}", h.SessionDetailsPage)
	r.Delete("/packing-session/{id}", h.DeletePackingSession)
	r.Post("/packing-session/toggle-item", h.ToggleSessionItemHandler)
	return r
}
