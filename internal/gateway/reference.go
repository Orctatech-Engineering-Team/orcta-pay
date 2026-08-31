package gateway

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// ReferencePrefix is the product attribution prefix per ADR-034.
const ReferencePrefix = "optd"

// BuildReference formats optd-{product}-{gateway}-{ulid}.
func BuildReference(product string, gateway Gateway, ulid string) string {
	return fmt.Sprintf("%s-%s-%s-%s", ReferencePrefix, product, string(gateway), ulid)
}

// ParseReference splits an optd reference into product, gateway, and ulid.
func ParseReference(ref string) (product string, gateway Gateway, ulid string, ok bool) {
	parts := strings.Split(ref, "-")
	if len(parts) < 4 || parts[0] != ReferencePrefix {
		return "", "", "", false
	}
	// product may contain dashes, so gateway is always second-last, ulid last.
	ulid = parts[len(parts)-1]
	gateway = Gateway(parts[len(parts)-2])
	if !gateway.Valid() {
		return "", "", "", false
	}
	product = strings.Join(parts[1:len(parts)-2], "-")
	if product == "" {
		return "", "", "", false
	}
	return product, gateway, ulid, true
}

// NewULID generates a ULID-like 26-char Crockford Base32 string.
func NewULID() string {
	var entropy [10]byte
	_, _ = rand.Read(entropy[:])
	ms := uint64(time.Now().UTC().UnixMilli()) & 0xFFFFFFFFFF // 48 bits
	var id [16]byte
	binary.BigEndian.PutUint16(id[0:2], uint16(ms>>32))
	binary.BigEndian.PutUint32(id[2:6], uint32(ms))
	copy(id[6:], entropy[:])
	return encodeCrockford(id[:])
}

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func encodeCrockford(b []byte) string {
	// 128 bits -> 26 chars (5 bits each, last char 2 bits padded).
	out := make([]byte, 26)
	var buffer uint16
	var bitsLeft uint
	var idx int
	for _, by := range b {
		buffer = (buffer << 8) | uint16(by)
		bitsLeft += 8
		for bitsLeft >= 5 && idx < 26 {
			bitsLeft -= 5
			out[idx] = crockford[(buffer>>bitsLeft)&0x1F]
			idx++
		}
	}
	if idx < 26 {
		out[idx] = crockford[(buffer<<uint(5-bitsLeft))&0x1F]
	}
	return string(out)
}
