package api

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"testing"
)

func TestVerifyPaystackHMAC(t *testing.T) {
	secret := "sk_test_example"
	body := []byte(`{"event":"charge.success"}`)
	mac := hmac.New(sha512.New, []byte(secret))
	_, _ = mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	if !verifyPaystackHMAC(secret, body, signature) {
		t.Fatal("valid Paystack signature was rejected")
	}
	if verifyPaystackHMAC(secret, body, "invalid") {
		t.Fatal("invalid Paystack signature was accepted")
	}
}
