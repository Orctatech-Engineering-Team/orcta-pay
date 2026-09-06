package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TimestampTolerance is the Standard Webhooks recommended skew window.
const TimestampTolerance = 5 * time.Minute

// VerifyStandardWebhook verifies a webhook per the Standard Webhooks spec
// (https://github.com/standard-webhooks/standard-webhooks).
//
//   - Headers: webhook-id, webhook-timestamp, webhook-signature.
//   - Signed content: "{id}.{timestamp}.{payload}".
//   - HMAC-SHA256, base64-encoded, versioned "v1," prefix. Multiple signatures
//     may be space-separated; any match verifies.
//   - Timestamps outside TimestampTolerance are rejected (replay protection).
//
// secret may carry the "whsec_" prefix; it is base64-decoded per spec, with a
// fallback to treating it as raw bytes.
func VerifyStandardWebhook(secret, id, timestamp, signatureHeader string, payload []byte, now time.Time) bool {
	if secret == "" || id == "" || timestamp == "" || signatureHeader == "" {
		return false
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if d := now.Sub(time.Unix(ts, 0)); d < 0 {
		if -d > TimestampTolerance {
			return false
		}
	} else if d > TimestampTolerance {
		return false
	}

	key := decodeSecret(secret)
	msg := id + "." + timestamp + "." + string(payload)

	for _, sig := range strings.Fields(signatureHeader) {
		sig = strings.TrimPrefix(sig, "v1,")
		got, err := base64.StdEncoding.DecodeString(sig)
		if err != nil {
			// Hex variant for interoperability with non-conforming senders.
			if hexSig, hexErr := hex.DecodeString(sig); hexErr == nil {
				got = hexSig
			} else {
				continue
			}
		}
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(msg))
		if hmac.Equal(got, mac.Sum(nil)) {
			return true
		}
	}
	return false
}

// decodeSecret strips the whsec_ prefix and base64-decodes per the spec,
// falling back to raw bytes for plain-text secrets.
func decodeSecret(secret string) []byte {
	s := strings.TrimPrefix(secret, "whsec_")
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b
	}
	return []byte(s)
}

// SignStandardWebhook returns the "v1,<base64>" signature for a payload —
// used by tests and local tooling to generate conforming webhooks.
func SignStandardWebhook(secret, id string, timestamp time.Time, payload []byte) string {
	key := decodeSecret(secret)
	msg := fmt.Sprintf("%s.%d.%s", id, timestamp.Unix(), string(payload))
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
