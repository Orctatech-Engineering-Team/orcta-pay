package webhooks

import "testing"

func TestParsePayloadPaystack(t *testing.T) {
	raw := []byte(`{"event":"charge.success","data":{"id":12345,"reference":"optd-orctago-paystack-01ARZ3NDEKTSV4RRFFQ69G5FAV","status":"success"}}`)
	ev, err := parsePayload("paystack", raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev.EventID != "charge.success-12345" {
		t.Fatalf("EventID = %q", ev.EventID)
	}
	if ev.Reference != "optd-orctago-paystack-01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Fatalf("Reference = %q", ev.Reference)
	}
}

func TestParsePayloadHubtelNested(t *testing.T) {
	raw := []byte(`{"Data":{"TransactionId":"TX-99","ClientReference":"optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV","Status":"Success"}}`)
	ev, err := parsePayload("hubtel", raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev.EventID != "TX-99" || ev.Reference != "optd-orctago-hubtel-01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Fatalf("ev = %+v", ev)
	}
}

func TestParsePayloadMoolre(t *testing.T) {
	raw := []byte(`{"transactionid":"MRL-77","externalref":"optd-orctago-moolre-01ARZ3NDEKTSV4RRFFQ69G5FAV","status":"1"}`)
	ev, err := parsePayload("moolre", raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if ev.EventID != "MRL-77" || ev.Reference != "optd-orctago-moolre-01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Fatalf("ev = %+v", ev)
	}
}

func TestParsePayloadRejectsMissingFields(t *testing.T) {
	if _, err := parsePayload("paystack", []byte(`{"event":"charge.success"}`)); err == nil {
		t.Fatal("want error for missing reference")
	}
	if _, err := parsePayload("paystack", []byte(`not-json`)); err == nil {
		t.Fatal("want error for bad json")
	}
}
