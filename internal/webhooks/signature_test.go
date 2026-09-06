package webhooks

import (
	"testing"
	"time"
)

func TestStandardWebhookSignRoundTrip(t *testing.T) {
	secret := "whsec_" + "MfKQ9r8GKYqrqwRUPF1btjUhsYHguf.vSj9VfRvw0"
	id := "msg_2L8vGtJ9ZfT3iQ"
	ts := time.Unix(1725000000, 0)
	payload := []byte(`{"ok":true}`)

	sig := SignStandardWebhook(secret, id, ts, payload)
	if sig[:3] != "v1," {
		t.Fatalf("signature prefix = %q, want v1,", sig[:3])
	}
	if !VerifyStandardWebhook(secret, id, "1725000000", sig, payload, ts) {
		t.Fatal("valid signature rejected")
	}
}

func TestStandardWebhookRejectsTampered(t *testing.T) {
	secret := "whsec_MfKQ9r8GKYqrqwRUPF1btjUhsYHguf.vSj9VfRvw0"
	id, ts := "msg_1", "1725000000"
	payload := []byte(`{"ok":true}`)
	sig := SignStandardWebhook(secret, id, time.Unix(1725000000, 0), payload)

	if VerifyStandardWebhook(secret, id, ts, sig, []byte(`{"ok":false}`), time.Unix(1725000000, 0)) {
		t.Fatal("tampered payload accepted")
	}
	if VerifyStandardWebhook(secret+"x", id, ts, sig, payload, time.Unix(1725000000, 0)) {
		t.Fatal("wrong secret accepted")
	}
	if VerifyStandardWebhook("", id, ts, sig, payload, time.Unix(1725000000, 0)) {
		t.Fatal("empty secret accepted")
	}
}

func TestStandardWebhookTimestampTolerance(t *testing.T) {
	secret, id, payload := "secret", "msg_1", []byte(`{}`)
	sig := SignStandardWebhook(secret, id, time.Unix(1725000000, 0), payload)

	if VerifyStandardWebhook(secret, id, "1725000000", sig, payload, time.Unix(1725000000+int64(TimestampTolerance.Seconds())+1, 0)) {
		t.Fatal("stale timestamp accepted")
	}
	if VerifyStandardWebhook(secret, id, "1725000000", sig, payload, time.Unix(1725000000-7200, 0)) {
		t.Fatal("future timestamp beyond tolerance accepted")
	}
	if !VerifyStandardWebhook(secret, id, "1725000000", sig, payload, time.Unix(1725000000+60, 0)) {
		t.Fatal("timestamp within tolerance rejected")
	}
}

func TestStandardWebhookMultipleSignatures(t *testing.T) {
	secret, id := "whsec_MfKQ9r8GKYqrqwRUPF1btjUhsYHguf.vSj9VfRvw0", "msg_1"
	ts := time.Unix(1725000000, 0)
	payload := []byte(`{}`)
	good := SignStandardWebhook(secret, id, ts, payload)
	bad := SignStandardWebhook("whsec_other", id, ts, payload)

	combined := bad + " " + good
	if !VerifyStandardWebhook(secret, id, "1725000000", combined, payload, ts) {
		t.Fatal("second signature in list should verify")
	}
}
