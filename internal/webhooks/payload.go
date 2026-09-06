package webhooks

import (
	"encoding/json"
	"fmt"
	"strings"
)

// event is the minimal extracted information needed to process a webhook.
type event struct {
	EventID   string // aggregator_event_id for webhook_inbox dedup
	Reference string // our optd-... reference (ClientReference / reference / externalref)
}

// parsePayload extracts the event ID and charge reference from a gateway
// webhook payload. Gateways differ in shape, so lookups are case-insensitive
// and descend one level into a nested object (Hubtel Data, Paystack data).
func parsePayload(gatewayName string, raw []byte) (event, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return event{}, fmt.Errorf("json: %w", err)
	}

	var ev event
	switch gatewayName {
	case "paystack":
		ev = event{
			EventID:   lookupString(m, "id"),
			Reference: lookupString(m, "reference"),
		}
		// Paystack event IDs are per-event; combine event type + data.id.
		if eventType, _ := m["event"].(string); eventType != "" && ev.EventID != "" {
			ev.EventID = eventType + "-" + ev.EventID
		}
	case "hubtel":
		ev = event{
			EventID:   lookupString(m, "TransactionId", "transactionid", "transaction_id"),
			Reference: lookupString(m, "ClientReference", "client_reference"),
		}
	case "moolre":
		ev = event{
			EventID:   lookupString(m, "transactionid", "transaction_id", "TransactionId"),
			Reference: lookupString(m, "externalref", "external_ref", "reference", "ClientReference"),
		}
	default:
		return event{}, fmt.Errorf("unknown gateway %q", gatewayName)
	}
	if ev.EventID == "" || ev.Reference == "" {
		return event{}, fmt.Errorf("missing event id or reference")
	}
	return ev, nil
}

// lookupString searches m (case-insensitive), then one nested object level.
func lookupString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v := directString(m, k); v != "" {
			return v
		}
	}
	// One nested level (Hubtel "Data", Paystack "data").
	for _, nested := range m {
		if nm, ok := nested.(map[string]any); ok {
			for _, k := range keys {
				if v := directString(nm, k); v != "" {
					return v
				}
			}
		}
	}
	return ""
}

func directString(m map[string]any, key string) string {
	for k, v := range m {
		if !strings.EqualFold(k, key) {
			continue
		}
		switch t := v.(type) {
		case string:
			return t
		case float64:
			// Numeric event IDs (Paystack data.id) format without exponent.
			return strings.TrimSuffix(fmt.Sprintf("%.0f", t), ".")
		case json.Number:
			return t.String()
		}
	}
	return ""
}
