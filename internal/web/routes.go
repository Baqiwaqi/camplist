package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Config struct {
	PackingStore *packing.Store
	Auth         *auth.Auth
}

func Routes(cfg Config) *chi.Mux {
	r := chi.NewRouter()
	r.Use(requestLog)
	r.Use(sharingProtection())
	// Process health only: probes must not consume Cosmos request units.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok\n"))
	})

	// Browsers revalidate static files on every load (304 when unchanged), so a
	// restyle never renders against a cached stylesheet or script.
	fileServer := http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
	r.Handle("/static/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		fileServer.ServeHTTP(w, r)
	}))

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

	// Public pages: visitors get the landing page and a demo, members their lists.
	r.With(cfg.Auth.OptionalAuth).Get("/", h.HomePage)
	r.Get("/demo", h.DemoPage)

	// Pages
	r.Group(func(r chi.Router) {
		r.Use(cfg.Auth.RequireAuth)

		r.Get("/join/{owner}/{kind}/{id}/{token}", h.JoinPage)
		r.Post("/join/{owner}/{kind}/{id}/{token}", h.RequestAccess)
		r.Get("/sharing/{kind}/{id}", h.SharingPage)
		r.Post("/sharing/{kind}/{id}/invitations", h.CreateInvitation)
		r.Get("/sharing/{kind}/{id}/invitations/{hash}", h.InvitationRow)
		r.Get("/sharing/{kind}/{id}/invitations/{hash}/approve", h.ApproveInvitationPage)
		r.Post("/sharing/{kind}/{id}/invitations/{hash}", h.DecideInvitation)
		r.Post("/sharing/{kind}/{id}/members/{subject}/remove", h.RemoveMember)
		r.Get("/packing-lists", h.MainPage)
		r.Post("/trips/{id}/name", h.RenameTrip)
		r.Post("/trips/{id}/entries", h.AddTripEntry)
		r.Get("/api/identity", h.IdentityAPI)
		r.Get("/api/sessions/{id}", h.SessionAPI)
		r.Post("/api/sessions/{id}/sync", h.SyncSessionAPI)
		r.Get("/sessions", h.SessionsPage)
		r.Get("/trips", h.SessionsPage)
		r.Get("/trips/archive", h.ArchivedSessionsPage)
		r.Get("/packing-list/new", h.NewListPage)
		r.Get("/packing-lists/new", h.NewListPage)
		r.Post("/packing-list/new", h.NewListHandler)
		r.Post("/packing-lists/new", h.NewListHandler)

		r.Get("/packing-list/{id}/edit", h.EditListPage)
		r.Get("/packing-lists/{id}/edit", h.EditListPage)
		r.Post("/packing-list/{id}/edit", h.EditListHandler)
		r.Post("/packing-lists/{id}/edit", h.EditListHandler)

		r.Get("/packing-list/{id}", h.ListDetailsPage)
		r.Get("/packing-lists/{id}", h.ListDetailsPage)

		r.Post("/packing-list/{id}/add-item", h.AddItemHandler)
		r.Post("/packing-lists/{id}/add-item", h.AddItemHandler)
		r.Delete("/packing-list/{id}/remove-item/{itemId}", h.RemoveItemHandler)
		r.Delete("/packing-lists/{id}/remove-item/{itemId}", h.RemoveItemHandler)
		r.Get("/packing-list/{id}/items/{itemId}", h.ItemRowHandler)
		r.Get("/packing-lists/{id}/items/{itemId}", h.ItemRowHandler)
		r.Get("/packing-list/{id}/items/{itemId}/edit", h.EditItemPage)
		r.Get("/packing-lists/{id}/items/{itemId}/edit", h.EditItemPage)
		r.Post("/packing-list/{id}/edit-item/{itemId}", h.EditItemHandler)
		r.Post("/packing-lists/{id}/edit-item/{itemId}", h.EditItemHandler)

		r.Delete("/packing-list/{id}", h.DeleteListHandler)
		r.Delete("/packing-lists/{id}", h.DeleteListHandler)

		r.Post("/packing-list/start-session", h.CreateSessionHandler)
		r.Post("/packing-lists/start-session", h.CreateSessionHandler)
		r.Get("/packing-session/{id}", h.SessionDetailsPage)
		r.Get("/trips/{id}", h.SessionDetailsPage)
		r.Get("/packing-session/{id}/review", h.ReviewPage)
		r.Get("/trips/{id}/review", h.ReviewPage)
		r.Post("/packing-session/{id}/review", h.AddReviewHandler)
		r.Post("/trips/{id}/review", h.AddReviewHandler)
		r.Post("/packing-session/{id}/review/apply", h.ApplyReviewHandler)
		r.Post("/trips/{id}/review/apply", h.ApplyReviewHandler)
		r.Post("/packing-session/{id}/review/recover", h.RecoverReviewHandler)
		r.Post("/trips/{id}/review/recover", h.RecoverReviewHandler)
		r.Post("/packing-list/{id}/preparation", h.PreparationHandler)
		r.Post("/packing-lists/{id}/preparation", h.PreparationHandler)
		r.Post("/packing-list/{id}/preparation/edit", h.EditPreparationTask)
		r.Post("/packing-lists/{id}/preparation/edit", h.EditPreparationTask)
		r.Post("/packing-session/{id}/preparation", h.SetSessionPreparationTask)
		r.Post("/trips/{id}/preparation", h.SetSessionPreparationTask)
		r.Delete("/packing-session/{id}", h.DeletePackingSession)
		r.Delete("/trips/{id}", h.DeletePackingSession)
		r.Post("/packing-session/set-item", h.SetSessionItemHandler)
		r.Post("/trips/set-item", h.SetSessionItemHandler)
	})
	return r
}
