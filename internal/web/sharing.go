package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/views"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"net/http"
)

func (h *handler) SharingPage(w http.ResponseWriter, r *http.Request) { h.renderSharing(w, r, "") }
func (h *handler) renderSharing(w http.ResponseWriter, r *http.Request, link string) {
	view, err := h.packingStore.GetSharing(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "id"), auth.Subject(r.Context()))
	if err != nil {
		storeError(w, err, "Could not read sharing settings")
		return
	}
	render(w, r, views.SharingPage(view, link, csrf.Token(r)))
}
func (h *handler) CreateInvitation(w http.ResponseWriter, r *http.Request) {
	link, err := h.packingStore.CreateInvitation(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "id"), auth.Subject(r.Context()))
	if err != nil {
		storeError(w, err, "Could not create invitation")
		return
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	h.renderSharing(w, r, scheme+"://"+r.Host+link.Path())
}
func (h *handler) DecideInvitation(w http.ResponseWriter, r *http.Request) {
	if !parsePackingForm(w, r) {
		return
	}
	decision := r.PostForm.Get("decision")
	if decision != "approve" && decision != "revoke" {
		http.Error(w, "Invalid decision", 400)
		return
	}
	err := h.packingStore.DecideInvitation(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "id"), auth.Subject(r.Context()), chi.URLParam(r, "hash"), decision == "approve")
	if err != nil {
		storeError(w, err, "Could not change invitation")
		return
	}
	http.Redirect(w, r, "/sharing/"+chi.URLParam(r, "kind")+"/"+chi.URLParam(r, "id"), http.StatusSeeOther)
}
func (h *handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	actor := auth.Subject(r.Context())
	subject := chi.URLParam(r, "subject")
	err := h.packingStore.RemoveMember(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "id"), actor, subject)
	if err != nil {
		storeError(w, err, "Could not remove member")
		return
	}
	if actor == subject {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/sharing/"+chi.URLParam(r, "kind")+"/"+chi.URLParam(r, "id"), http.StatusSeeOther)
}
func invitationLink(r *http.Request) packing.InvitationLink {
	return packing.InvitationLink{Kind: chi.URLParam(r, "kind"), ID: chi.URLParam(r, "id"), Owner: chi.URLParam(r, "owner"), Token: chi.URLParam(r, "token")}
}
func (h *handler) JoinPage(w http.ResponseWriter, r *http.Request) {
	link := invitationLink(r)
	status, err := h.packingStore.InvitationStatus(r.Context(), link, auth.Subject(r.Context()))
	if err != nil {
		http.Error(w, "This invitation is unavailable or expired. Ask the owner for a new link.", 404)
		return
	}
	render(w, r, views.JoinPage(link, status, auth.UserProfile(r.Context()), csrf.Token(r)))
}
func (h *handler) RequestAccess(w http.ResponseWriter, r *http.Request) {
	profile := auth.UserProfile(r.Context())
	link := invitationLink(r)
	err := h.packingStore.RequestAccess(r.Context(), link, packing.Member{Subject: profile.Subject, Name: profile.Name, Email: profile.Email, EmailVerified: profile.EmailVerified})
	if err != nil {
		storeError(w, err, "Could not request access")
		return
	}
	http.Redirect(w, r, link.Path(), http.StatusSeeOther)
}
