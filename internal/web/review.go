package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/views"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"net/http"
	"strconv"
)

func (h *handler) ReviewPage(w http.ResponseWriter, r *http.Request) {
	h.renderReview(w, r, packing.ReviewEntry{ID: uuid.NewString(), Action: "none"}, "", http.StatusOK)
}
func (h *handler) renderReview(w http.ResponseWriter, r *http.Request, entry packing.ReviewEntry, message string, status int) {
	user, err := auth.UserID(r.Context())
	if err != nil {
		http.Error(w, "Sign in required", 401)
		return
	}
	session, err := h.packingStore.GetPackingSession(r.Context(), chi.URLParam(r, "id"), user)
	if err != nil {
		storeError(w, err, "Could not read trip")
		return
	}
	list, err := h.packingStore.GetPackingList(r.Context(), session.TemplateID(), user)
	available := err == nil
	recoverable := false
	if err != nil {
		code, _ := storeErrorDetails(err, "")
		recoverable = code == 404
		if code != 404 && code != 403 {
			storeError(w, err, "Could not read template")
			return
		}
	}
	if session.UserID != user && !available {
		storeError(w, packing.ErrForbidden, "")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	render(w, r, views.ReviewPage(session, list, available, recoverable, entry, message, csrf.Token(r)))
}
func parsePackingForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return false
	}
	return true
}
func (h *handler) AddReviewHandler(w http.ResponseWriter, r *http.Request) {
	if !parsePackingForm(w, r) {
		return
	}
	user, err := auth.UserID(r.Context())
	if err != nil {
		http.Error(w, "Sign in required", 401)
		return
	}
	entry := packing.ReviewEntry{ID: r.PostForm.Get("entryId"), ItemID: r.PostForm.Get("itemId"), Name: r.PostForm.Get("name"), Category: r.PostForm.Get("category"), Note: r.PostForm.Get("note"), Forgotten: r.PostForm.Get("forgotten") == "true", Unused: r.PostForm.Get("unused") == "true", NeedsAttention: r.PostForm.Get("needsAttention") == "true", Action: packing.ReviewAction(r.PostForm.Get("action")), Task: r.PostForm.Get("task")}
	if entry.ItemID != "" {
		session, e := h.packingStore.GetPackingSession(r.Context(), chi.URLParam(r, "id"), user)
		if e != nil {
			storeError(w, e, "Could not read trip")
			return
		}
		for _, item := range session.List.Items {
			if item.ID == entry.ItemID {
				entry.Name = item.Name
				entry.Category = item.Category
			}
		}
	}
	_, err = h.packingStore.AddReviewEntry(r.Context(), chi.URLParam(r, "id"), user, entry)
	if err != nil {
		status, message := storeErrorDetails(err, "Could not save review")
		h.renderReview(w, r, entry, message, status)
		return
	}
	http.Redirect(w, r, "/trips/"+chi.URLParam(r, "id")+"/review", http.StatusSeeOther)
}
func (h *handler) ApplyReviewHandler(w http.ResponseWriter, r *http.Request) {
	if !parsePackingForm(w, r) {
		return
	}
	user, err := auth.UserID(r.Context())
	if err != nil {
		http.Error(w, "Sign in required", 401)
		return
	}
	list, err := h.packingStore.ApplyReview(r.Context(), chi.URLParam(r, "id"), user, r.PostForm.Get("revision"), r.PostForm["selected"])
	if err != nil {
		status, message := storeErrorDetails(err, "Could not apply review")
		h.renderReview(w, r, packing.ReviewEntry{ID: uuid.NewString(), Action: "none"}, message, status)
		return
	}
	http.Redirect(w, r, "/packing-lists/"+list.ID, http.StatusSeeOther)
}
func (h *handler) RecoverReviewHandler(w http.ResponseWriter, r *http.Request) {
	user, err := auth.UserID(r.Context())
	if err != nil {
		http.Error(w, "Sign in required", 401)
		return
	}
	_, err = h.packingStore.RecoverReviewTemplate(r.Context(), chi.URLParam(r, "id"), user)
	if err != nil {
		storeError(w, err, "Could not recover template")
		return
	}
	http.Redirect(w, r, "/trips/"+chi.URLParam(r, "id")+"/review", http.StatusSeeOther)
}
func (h *handler) PreparationHandler(w http.ResponseWriter, r *http.Request) {
	if !parsePackingForm(w, r) {
		return
	}
	user, err := auth.UserID(r.Context())
	if err != nil {
		http.Error(w, "Sign in required", 401)
		return
	}
	done, err := strconv.ParseBool(r.PostForm.Get("done"))
	if err != nil {
		http.Error(w, "Invalid task state", 400)
		return
	}
	_, err = h.packingStore.SetPreparationTask(r.Context(), chi.URLParam(r, "id"), user, r.PostForm.Get("taskId"), done, r.PostForm.Get("revision"))
	if err != nil {
		storeError(w, err, "Could not save preparation")
		return
	}
	http.Redirect(w, r, "/packing-lists/"+chi.URLParam(r, "id"), http.StatusSeeOther)
}
