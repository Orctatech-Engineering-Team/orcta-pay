package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/orctatech/orcta-pay/internal/platform"
)

func handleListCharges(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.Pool == nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}
		product := r.URL.Query().Get("product")
		gateway := r.URL.Query().Get("gateway")
		status := r.URL.Query().Get("status")

		query := `SELECT ref, product, gateway, amount_pesewas, currency, status, created_at::text FROM payment_intents WHERE 1=1`
		args := []any{}
		idx := 1
		if product != "" && product != "all" {
			query += ` AND product = $` + strconv.Itoa(idx)
			args = append(args, product)
			idx++
		}
		if gateway != "" && gateway != "all" {
			query += ` AND gateway = $` + strconv.Itoa(idx)
			args = append(args, gateway)
			idx++
		}
		if status != "" && status != "all" {
			query += ` AND status = $` + strconv.Itoa(idx)
			args = append(args, status)
			idx++
		}
		query += ` ORDER BY created_at DESC LIMIT $` + strconv.Itoa(idx)
		args = append(args, limit)

		rows, err := app.Pool.Query(r.Context(), query, args...)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
			return
		}
		defer rows.Close()

		type row struct {
			Ref           string `json:"ref"`
			Product       string `json:"product"`
			Gateway       string `json:"gateway"`
			AmountPesewas int64  `json:"amount_pesewas"`
			Currency      string `json:"currency"`
			Status        string `json:"status"`
			CreatedAt     string `json:"created_at"`
		}
		out := []row{}
		for rows.Next() {
			var ref, productV, gatewayV, currencyV, statusV, createdAt string
			var amount int64
			if err := rows.Scan(&ref, &productV, &gatewayV, &amount, &currencyV, &statusV, &createdAt); err != nil {
				continue
			}
			out = append(out, row{
				Ref: ref, Product: productV, Gateway: gatewayV,
				AmountPesewas: amount, Currency: currencyV, Status: statusV,
				CreatedAt: createdAt,
			})
		}
		if out == nil {
			out = []row{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}

func handleListPayouts(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.Pool == nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		rows, err := app.Pool.Query(r.Context(),
			`SELECT id, product, total_pesewas, currency, status, batch_date::text, created_at::text FROM payout_batches ORDER BY created_at DESC LIMIT 100`)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
			return
		}
		defer rows.Close()

		type batchRow struct {
			ID                string `json:"id"`
			BatchDate         string `json:"batch_date"`
			Status            string `json:"status"`
			VendorCount       int    `json:"vendor_count"`
			GrossPesewas      int64  `json:"gross_pesewas"`
			CommissionPesewas int64  `json:"commission_pesewas"`
			NetPesewas        int64  `json:"net_pesewas"`
			CreatedAt         string `json:"created_at"`
			Lines             []any  `json:"lines"`
		}
		batches := []batchRow{}
		for rows.Next() {
			var id, product, currency, status, batchDate, createdAt string
			var total int64
			if err := rows.Scan(&id, &product, &total, &currency, &status, &batchDate, &createdAt); err != nil {
				continue
			}
			// Count reservations for vendor_count.
			var vendorCount int
			_ = app.Pool.QueryRow(r.Context(), `SELECT COUNT(*) FROM payout_reservations WHERE batch_id = $1`, id).Scan(&vendorCount)
			batches = append(batches, batchRow{
				ID: id, BatchDate: batchDate, Status: status,
				VendorCount: vendorCount, GrossPesewas: total, CommissionPesewas: 0,
				NetPesewas: total, CreatedAt: createdAt, Lines: []any{},
			})
			_ = product
			_ = currency
		}
		if batches == nil {
			batches = []batchRow{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(batches)
	}
}

func handleListLedger(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.Pool == nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		limit := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}
		ref := r.URL.Query().Get("ref")
		query := `SELECT id, kind, ref, amount_pesewas, currency, value_time::text, booking_time::text, settlement_time::text, product, created_at::text FROM ledger_entries`
		args := []any{}
		if ref != "" {
			query += ` WHERE ref = $1 ORDER BY booking_time LIMIT $2`
			args = append(args, ref, limit)
		} else {
			query += ` ORDER BY created_at DESC LIMIT $1`
			args = append(args, limit)
		}
		rows, err := app.Pool.Query(r.Context(), query, args...)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
			return
		}
		defer rows.Close()

		type entry struct {
			ID             string  `json:"id"`
			Product        string  `json:"product"`
			Wallet         string  `json:"wallet"`
			VendorID       string  `json:"vendor_id"`
			EntryType      string  `json:"entry_type"`
			AmountPesewas  int64   `json:"amount_pesewas"`
			Reason         string  `json:"reason"`
			Ref            string  `json:"ref"`
			ValueTime      string  `json:"value_time"`
			BookingTime    string  `json:"booking_time"`
			SettlementTime *string `json:"settlement_time"`
			Kind           string  `json:"kind"`
		}
		out := []entry{}
		for rows.Next() {
			var id, kind, refV, currency, product string
			var amount int64
			var valueTime, bookingTime, createdAt string
			var settlementTime *string
			if err := rows.Scan(&id, &kind, &refV, &amount, &currency, &valueTime, &bookingTime, &settlementTime, &product, &createdAt); err != nil {
				continue
			}
			entryType := "credit"
			reason := "order_payment"
			if kind == "commission" {
				entryType = "debit"
				reason = "commission_deduction"
			}
			_ = createdAt
			_ = currency
			out = append(out, entry{
				ID: id, Product: product, Wallet: product, VendorID: product,
				EntryType: entryType, AmountPesewas: amount, Reason: reason,
				Ref: refV, ValueTime: valueTime, BookingTime: bookingTime,
				SettlementTime: settlementTime, Kind: kind,
			})
		}
		if out == nil {
			out = []entry{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}

func handleListWebhooks(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.Pool == nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		gateway := r.URL.Query().Get("gateway")
		query := `SELECT aggregator_event_id, gateway, payload, received_at::text, processed_at::text FROM webhook_inbox`
		args := []any{}
		if gateway != "" && gateway != "all" {
			query += ` WHERE gateway = $1 ORDER BY received_at DESC LIMIT 100`
			args = append(args, gateway)
		} else {
			query += ` ORDER BY received_at DESC LIMIT 100`
		}
		rows, err := app.Pool.Query(r.Context(), query, args...)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "query failed")
			return
		}
		defer rows.Close()

		type wh struct {
			ID                string  `json:"id"`
			AggregatorEventID string  `json:"aggregator_event_id"`
			Gateway           string  `json:"gateway"`
			Payload           any     `json:"payload"`
			ReceivedAt        string  `json:"received_at"`
			ProcessedAt       *string `json:"processed_at"`
		}
		out := []wh{}
		for rows.Next() {
			var aggID, gw string
			var payload []byte
			var receivedAt string
			var processedAt *string
			if err := rows.Scan(&aggID, &gw, &payload, &receivedAt, &processedAt); err != nil {
				continue
			}
			var p any
			if len(payload) > 0 {
				_ = json.Unmarshal(payload, &p)
			}
			out = append(out, wh{
				ID: aggID, AggregatorEventID: aggID, Gateway: gw,
				Payload: p, ReceivedAt: receivedAt, ProcessedAt: processedAt,
			})
		}
		if out == nil {
			out = []wh{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}
}

func handleGatewayHealth(app *platform.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.Pool == nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		_ = r
		_ = app
		writeJSON(w, http.StatusOK, []any{})
	}
}
