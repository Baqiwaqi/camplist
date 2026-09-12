package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Config struct {
	PackingStore *packing.Store
	Auth         *auth.Auth
}

func Routes(cfg Config) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	r.Get("/offline", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "static/offline/offline.html") })
	r.Get("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, "static/offline/sw.js")
	})
	h := handler{packingStore: cfg.PackingStore}

	// Auth
	r.Get("/auth/login", cfg.Auth.LoginHandler)
	r.Get("/auth/callback", cfg.Auth.CallbackHandler)
	r.Post("/auth/signout", cfg.Auth.DeleteHandler)

	r.Get("/login", h.LoginPage)

	// Pages
	r.Group(func(r chi.Router) {
		r.Use(cfg.Auth.RequireAuth)

		r.Get("/api/identity", h.IdentityAPI)
		r.Get("/api/sessions/{id}", h.SessionAPI)
		r.Post("/api/sessions/{id}/sync", h.SyncSessionAPI)
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
		r.Get("/packing-session/{id}/review", h.ReviewPage)
		r.Post("/packing-session/{id}/review", h.AddReviewHandler)
		r.Post("/packing-session/{id}/review/apply", h.ApplyReviewHandler)
		r.Post("/packing-session/{id}/review/recover", h.RecoverReviewHandler)
		r.Post("/packing-list/{id}/preparation", h.PreparationHandler)
		r.Delete("/packing-session/{id}", h.DeletePackingSession)
		r.Post("/packing-session/set-item", h.SetSessionItemHandler)
	})
	return r
}
