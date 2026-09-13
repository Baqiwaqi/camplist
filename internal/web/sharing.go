package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/views"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"net/http"
)

func (h *handler) SharingPage(w http.ResponseWriter, r *http.Request) { h.renderSharing(w, r, "", "") }
func (h *handler) renderSharing(w http.ResponseWriter, r *http.Request, link, approving string) {
	view, trips, ok := h.loadSharing(w, r)
	if !ok {
		return
	}
	render(w, r, views.SharingPage(view, link, csrf.Token(r), trips, approving))
}

// loadSharing reads sharing settings and, when an owner has a list request to
// approve, the current trips that can be shared with the same account.
func (h *handler) loadSharing(w http.ResponseWriter, r *http.Request) (packing.SharingView, []packing.PackingSession, bool) {
	view, err := h.packingStore.GetSharing(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "id"), auth.Subject(r.Context()))
	if err != nil {
		storeError(w, err, "Could not read sharing settings")
		return view, nil, false
	}
	pending := false
	for _, invitation := range view.Sharing.Invitations {
		pending = pending || invitation.Status == "pending"
	}
	if view.Kind != "packing-list" || view.Actor != view.Owner || !pending {
		return view, nil, true
	}
	trips, err := h.packingStore.ListTrips(r.Context(), view.ID, view.Actor)
	if err != nil {
		storeError(w, err, "Could not read trips for this list")
		return view, nil, false
	}
	return view, trips, true
}

// InvitationRow returns one invitation row; with approve it opens the trip
// prompt. Without htmx both fall back to the whole sharing page.
func (h *handler) InvitationRow(w http.ResponseWriter, r *http.Request) { h.invitationRow(w, r, false) }
func (h *handler) ApproveInvitationPage(w http.ResponseWriter, r *http.Request) {
	h.invitationRow(w, r, true)
}
func (h *handler) invitationRow(w http.ResponseWriter, r *http.Request, approving bool) {
	hash := chi.URLParam(r, "hash")
	if !isHTMX(r) {
		if !approving {
			hash = ""
		}
		h.renderSharing(w, r, "", hash)
		return
	}
	view, trips, ok := h.loadSharing(w, r)
	if !ok {
		return
	}
	for _, invitation := range view.Sharing.Invitations {
		if invitation.Hash == hash {
			render(w, r, views.InvitationRow(view, invitation, trips, approving && invitation.Status == "pending" && len(trips) > 0, false, csrf.Token(r)))
			return
		}
	}
	http.Error(w, "That invitation is no longer available.", http.StatusNotFound)
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
	joinURL := scheme + "://" + r.Host + link.Path()
	if !isHTMX(r) {
		// The one-time link cannot survive a redirect, so the plain form renders
		// the page in the POST response.
		h.renderSharing(w, r, joinURL, "")
		return
	}
	actor := auth.Subject(r.Context())
	view, err := h.packingStore.GetSharing(r.Context(), link.Kind, link.ID, actor)
	if err != nil {
		// The link exists and cannot be shown again, so show it without the new row.
		view = packing.SharingView{Kind: link.Kind, ID: link.ID, Owner: actor, Actor: actor}
	}
	render(w, r, views.CreatedInvitation(view, joinURL, link.Hash(), csrf.Token(r)))
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
	kind, id, actor, hash := chi.URLParam(r, "kind"), chi.URLParam(r, "id"), auth.Subject(r.Context()), chi.URLParam(r, "hash")
	var err error
	if trips := r.PostForm["trip"]; decision == "approve" && kind == "packing-list" {
		err = h.packingStore.ApproveInvitationWithTrips(r.Context(), id, actor, hash, trips)
	} else if len(trips) > 0 {
		http.Error(w, "Only list requests can share trips", 400)
		return
	} else {
		err = h.packingStore.DecideInvitation(r.Context(), kind, id, actor, hash, decision == "approve")
	}
	if err != nil {
		storeError(w, err, "Could not change invitation")
		return
	}
	if isHTMX(r) {
		h.decidedInvitation(w, r, hash, decision == "approve")
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
		if isHTMX(r) {
			w.Header().Set("HX-Redirect", "/")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if isHTMX(r) {
		view, err := h.packingStore.GetSharing(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "id"), actor)
		if err != nil {
			storeError(w, err, "Could not read sharing settings")
			return
		}
		render(w, r, views.MemberList(view, false, true, csrf.Token(r)))
		return
	}
	http.Redirect(w, r, "/sharing/"+chi.URLParam(r, "kind")+"/"+chi.URLParam(r, "id"), http.StatusSeeOther)
}

// decidedInvitation answers an htmx approve or revoke with the decided row and,
// after an approval, the member card out of band. An expired invitation is no
// longer listed, so its row is replaced with nothing.
func (h *handler) decidedInvitation(w http.ResponseWriter, r *http.Request, hash string, approved bool) {
	view, err := h.packingStore.GetSharing(r.Context(), chi.URLParam(r, "kind"), chi.URLParam(r, "id"), auth.Subject(r.Context()))
	if err != nil {
		storeError(w, err, "Could not read sharing settings")
		return
	}
	for _, invitation := range view.Sharing.Invitations {
		if invitation.Hash == hash {
			render(w, r, views.DecidedInvitation(view, invitation, approved, csrf.Token(r)))
			return
		}
	}
	if approved {
		render(w, r, views.MemberList(view, true, false, csrf.Token(r)))
	}
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
