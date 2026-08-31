package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/orctatech/orcta-pay/internal/charges"
	"github.com/orctatech/orcta-pay/internal/money"
	"github.com/orctatech/orcta-pay/internal/payouts"
	"github.com/orctatech/orcta-pay/internal/platform"
)

const maxBodyBytes = 1 << 20 // 1 MiB

func handleCreateCharge(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Product        string         `json:"product"`
			AmountPesewas  int64          `json:"amount_pesewas"`
			Currency       string         `json:"currency"`
			Wallet         string         `json:"wallet"`
			Phone          string         `json:"phone"`
			IdempotencyKey string         `json:"idempotency_key"`
			Metadata       map[string]any `json:"metadata"`
		}
		if err := decodeJSON(w, r, &body); err != nil {
			return
		}
		req := charges.ChargeRequest{
			Product:        body.Product,
			Amount:         money.New(body.AmountPesewas, money.Currency(body.Currency)),
			Wallet:         body.Wallet,
			Phone:          body.Phone,
			IdempotencyKey: body.IdempotencyKey,
			Metadata:       body.Metadata,
		}
		result, err := app.Charges.Initiate(r.Context(), req)
		if err != nil {
			if errors.Is(err, charges.ErrInvalidRequest) {
				writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
			return
		}
		switch v := result.(type) {
		case charges.ChargePending:
			writeJSON(w, http.StatusCreated, map[string]any{
				"ref": v.Ref, "gateway": v.Gateway, "status": "pending", "external_ref": v.ExternalRef,
			})
		case charges.ChargeFailed:
			writeError(w, http.StatusBadGateway, "unavailable", v.Reason)
		case charges.ChargeSucceeded:
			writeJSON(w, http.StatusCreated, map[string]any{
				"ref": v.Ref, "gateway": v.Gateway, "status": "succeeded",
			})
		default:
			writeJSON(w, http.StatusCreated, result)
		}
	}
}

func handleChargeStatus(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ref := chi.URLParam(r, "ref")
		if ref == "" {
			writeError(w, http.StatusBadRequest, "invalid_request", "ref is required")
			return
		}
		result, err := app.Charges.Status(r.Context(), ref)
		if err != nil {
			if errors.Is(err, charges.ErrInvalidRequest) {
				writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ref": result.Reference, "status": result.Status, "verified_at": result.VerifiedAt,
		})
	}
}

func handleCreatePayout(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Product        string `json:"product"`
			IdempotencyKey string `json:"idempotency_key"`
			Entries        []struct {
				Recipient     string `json:"recipient"`
				AmountPesewas int64  `json:"amount_pesewas"`
				Currency      string `json:"currency"`
			} `json:"entries"`
		}
		if err := decodeJSON(w, r, &body); err != nil {
			return
		}
		var entries []payouts.Entry
		for _, e := range body.Entries {
			entries = append(entries, payouts.Entry{
				Recipient: e.Recipient,
				Amount:    money.New(e.AmountPesewas, money.Currency(e.Currency)),
			})
		}
		batch, err := app.Payouts.Create(r.Context(), payouts.CreateRequest{
			Product:        body.Product,
			Entries:        entries,
			IdempotencyKey: body.IdempotencyKey,
		})
		if err != nil {
			if errors.Is(err, payouts.ErrInvalidRequest) {
				writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
				return
			}
			if errors.Is(err, payouts.ErrInsufficientFunds) {
				writeError(w, http.StatusConflict, "conflict", err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"batch_id": batch.ID, "product": batch.Product, "status": batch.Status, "total_pesewas": batch.Total.MinorUnits(),
		})
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer func() { _, _ = io.Copy(io.Discard, r.Body); _ = r.Body.Close() }()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON: "+err.Error())
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}
