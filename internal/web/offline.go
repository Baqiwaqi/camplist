package web

import (
	"camplist/internal/auth"
	"camplist/internal/packing"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"io"
	"net/http"
)

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func apiUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	user, err := auth.UserID(r.Context())
	if err != nil {
		jsonResponse(w, 401, map[string]string{"code": "signin"})
		return "", false
	}
	if expected := r.Header.Get("X-Camplist-Account"); expected != "" && expected != user {
		jsonResponse(w, 409, map[string]string{"code": "account"})
		return "", false
	}
	return user, true
}
func (h *handler) IdentityAPI(w http.ResponseWriter, r *http.Request) {
	user, ok := apiUser(w, r)
	if !ok {
		return
	}
	jsonResponse(w, 200, map[string]string{"userId": user, "csrfToken": csrf.Token(r)})
}
func (h *handler) SessionAPI(w http.ResponseWriter, r *http.Request) {
	user, ok := apiUser(w, r)
	if !ok {
		return
	}
	session, err := h.packingStore.GetPackingSession(r.Context(), chi.URLParam(r, "id"), user)
	if err != nil {
		status, message := storeErrorDetails(err, "Could not read session")
		jsonResponse(w, status, map[string]string{"code": accessErrorCode(err), "message": message})
		return
	}
	session.Operations = nil
	jsonResponse(w, 200, map[string]any{"session": session.Snapshot(user)})
}
func (h *handler) SyncSessionAPI(w http.ResponseWriter, r *http.Request) {
	user, ok := apiUser(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input struct {
		ID               string `json:"id"`
		Action           string `json:"action,omitempty"`
		Kind             string `json:"kind,omitempty"`
		Name             string `json:"name,omitempty"`
		Category         string `json:"category,omitempty"`
		Scope            string `json:"scope,omitempty"`
		SaveForFuture    bool   `json:"saveForFuture,omitempty"`
		ItemID           string `json:"itemId"`
		Checked          *bool  `json:"checked"`
		ExpectedRevision *int64 `json:"expectedRevision"`
	}
	if err := decoder.Decode(&input); err != nil || input.Action != "add" && (input.Checked == nil || input.ExpectedRevision == nil) {
		jsonResponse(w, 400, map[string]string{"code": "invalid"})
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		jsonResponse(w, 400, map[string]string{"code": "invalid"})
		return
	}
	if input.Checked == nil {
		input.Checked = new(bool)
	}
	if input.ExpectedRevision == nil {
		input.ExpectedRevision = new(int64)
	}
	op := packing.PackingOperation{Action: input.Action, Kind: input.Kind, Name: input.Name, Category: input.Category, Scope: input.Scope, SaveForFuture: input.SaveForFuture, ID: input.ID, ItemID: input.ItemID, Checked: *input.Checked, ExpectedRevision: *input.ExpectedRevision}
	session, err := h.packingStore.SyncSessionItem(r.Context(), chi.URLParam(r, "id"), user, op)
	if op.Action == "add" && op.Kind != "task" && (err == nil || errors.Is(err, packing.ErrFutureSave)) {
		h.rememberCategory(r.Context(), user, op.Category)
	}
	session.Operations = nil
	if errors.Is(err, packing.ErrFutureSave) {
		jsonResponse(w, 200, map[string]any{"operationId": op.ID, "session": session.Snapshot(user), "futureSaveError": true})
		return
	}
	if errors.Is(err, packing.ErrConflict) && session.ID != "" {
		jsonResponse(w, 409, map[string]any{"code": "conflict", "session": session.Snapshot(user)})
		return
	}
	if err != nil {
		status, message := storeErrorDetails(err, "Could not synchronize")
		jsonResponse(w, status, map[string]string{"code": accessErrorCode(err), "message": message})
		return
	}
	jsonResponse(w, 200, map[string]any{"operationId": op.ID, "session": session.Snapshot(user)})
}

func accessErrorCode(err error) string {
	if errors.Is(err, packing.ErrAccessRemoved) {
		return "access_removed"
	}
	return "unavailable"
}
