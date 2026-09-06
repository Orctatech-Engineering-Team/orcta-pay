package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/apps"
	"github.com/Orctatech-Engineering-Team/orcta-pay/internal/platform"
)

func handleCreateApp(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name    string `json:"name"`
			Product string `json:"product"`
		}
		if err := decodeJSON(w, r, &body); err != nil {
			return
		}
		if body.Name == "" || body.Product == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "name and product are required")
			return
		}
		result, err := app.Apps.CreateApp(r.Context(), apps.CreateAppRequest{Name: body.Name, Product: body.Product})
		if err != nil {
			if errors.Is(err, apps.ErrInvalidRequest) {
				writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
				return
			}
			if errors.Is(err, apps.ErrConflict) {
				writeError(w, http.StatusConflict, "conflict", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
			return
		}
		writeJSON(w, http.StatusCreated, result)
	}
}

func handleListApps(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := app.Apps.ListApps(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
			return
		}
		if list == nil {
			list = []apps.App{}
		}
		// Encode as JSON array; avoid bare null.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(list)
	}
}

func handleGetApp(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		if idStr == "" {
			idStr = chi.URLParam(r, "appID")
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid app id")
			return
		}
		a, err := app.Apps.GetApp(r.Context(), id)
		if err != nil {
			if errors.Is(err, apps.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "app not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
			return
		}
		writeJSON(w, http.StatusOK, a)
	}
}

func handleRotateAppKey(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		if idStr == "" {
			idStr = chi.URLParam(r, "appID")
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid app id")
			return
		}
		result, err := app.Apps.RotateKey(r.Context(), id)
		if err != nil {
			if errors.Is(err, apps.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "app not found")
				return
			}
			if errors.Is(err, apps.ErrInvalidRequest) {
				writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleRevokeApp(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		if idStr == "" {
			idStr = chi.URLParam(r, "appID")
		}
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid app id")
			return
		}
		if err := app.Apps.RevokeApp(r.Context(), id); err != nil {
			if errors.Is(err, apps.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "app not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
