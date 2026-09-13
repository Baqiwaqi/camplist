package web

import (
	"camplist/internal/packing"
	"errors"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

func storeError(w http.ResponseWriter, err error, fallback string) {
	status, message := storeErrorDetails(err, fallback)
	http.Error(w, message, status)
}

func storeErrorDetails(err error, fallback string) (int, string) {
	if errors.Is(err, packing.ErrAccessRemoved) {
		return http.StatusForbidden, "Your access was removed. Local changes will not be uploaded. You can export or remove your local copy."
	}
	if errors.Is(err, packing.ErrForbidden) {
		return http.StatusForbidden, "Only the owner can do that."
	}
	if errors.Is(err, packing.ErrInvalid) {
		return http.StatusBadRequest, "Please check the submitted values."
	}
	if errors.Is(err, packing.ErrConflict) {
		return http.StatusConflict, "The saved version changed. Review the current state and try again."
	}
	if errors.Is(err, packing.ErrNotFound) {
		return http.StatusNotFound, "That list, session, or item is no longer available."
	}
	var response *azcore.ResponseError
	if errors.As(err, &response) {
		switch response.StatusCode {
		case http.StatusNotFound:
			return http.StatusNotFound, "That list, session, or item is no longer available."
		case http.StatusPreconditionFailed, http.StatusConflict:
			return http.StatusConflict, "This list changed while you were working. Reload it before trying again."
		}
	}
	return http.StatusInternalServerError, fallback
}

func sessionExpired(w http.ResponseWriter) {
	http.Error(w, "Your session expired. Reload the page and sign in again.", http.StatusUnauthorized)
}

func malformedRequest(w http.ResponseWriter) {
	http.Error(w, "That change did not save. Reload the page and try again.", http.StatusBadRequest)
}
