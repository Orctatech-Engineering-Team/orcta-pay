// Package money provides the Money value type for monetary amounts.
// Integer minor units (pesewas), never float.
package money

import (
	"errors"
	"fmt"
	"strconv"
)

// Currency is an ISO 4217 currency code.
type Currency string

// GHS is the Ghanaian cedi.
const GHS Currency = "GHS"

const minorUnitsPerMajor = 100

// ErrCurrencyMismatch is returned when currencies differ in arithmetic.
var ErrCurrencyMismatch = errors.New("money: currency mismatch")

// Money is an exact amount in integer minor units.
type Money struct {
	minorUnits int64
	currency   Currency
}

// New returns a Money of the given minor units in currency.
func New(minorUnits int64, currency Currency) Money {
	return Money{minorUnits: minorUnits, currency: currency}
}

// Zero returns a zero amount in currency.
func Zero(currency Currency) Money {
	return Money{currency: currency}
}

// MinorUnits returns the raw amount for persistence and wire encoding.
func (m Money) MinorUnits() int64 { return m.minorUnits }

// Currency returns the currency.
func (m Money) Currency() Currency { return m.currency }

// Add returns the sum of m and other.
func (m Money) Add(other Money) (Money, error) {
	currency, err := m.combine(other)
	if err != nil {
		return Money{}, fmt.Errorf("money: add: %w", err)
	}
	return Money{minorUnits: m.minorUnits + other.minorUnits, currency: currency}, nil
}

// Sub returns m minus other.
func (m Money) Sub(other Money) (Money, error) {
	currency, err := m.combine(other)
	if err != nil {
		return Money{}, fmt.Errorf("money: sub: %w", err)
	}
	return Money{minorUnits: m.minorUnits - other.minorUnits, currency: currency}, nil
}

// Mul returns m scaled by an integer factor.
func (m Money) Mul(factor int64) Money {
	return Money{minorUnits: m.minorUnits * factor, currency: m.currency}
}

// IsZero reports whether the amount is zero.
func (m Money) IsZero() bool { return m.minorUnits == 0 }

// IsNegative reports whether the amount is below zero.
func (m Money) IsNegative() bool { return m.minorUnits < 0 }

// Compare returns -1, 0, or 1, erroring if currencies differ.
func (m Money) Compare(other Money) (int, error) {
	if _, err := m.combine(other); err != nil {
		return 0, fmt.Errorf("money: compare: %w", err)
	}
	switch {
	case m.minorUnits < other.minorUnits:
		return -1, nil
	case m.minorUnits > other.minorUnits:
		return 1, nil
	default:
		return 0, nil
	}
}

// String renders the amount for logs. Not a wire format.
func (m Money) String() string {
	sign := ""
	minorUnits := m.minorUnits
	if minorUnits < 0 {
		sign = "-"
		minorUnits = -minorUnits
	}
	major := strconv.FormatInt(minorUnits/minorUnitsPerMajor, 10)
	minor := strconv.FormatInt(minorUnits%minorUnitsPerMajor, 10)
	if len(minor) == 1 {
		minor = "0" + minor
	}
	return sign + major + "." + minor + " " + string(m.currency)
}

func (m Money) combine(other Money) (Currency, error) {
	switch {
	case m.currency == other.currency:
		return m.currency, nil
	case m.currency == "":
		return other.currency, nil
	case other.currency == "":
		return m.currency, nil
	default:
		return "", fmt.Errorf("%w: %s vs %s", ErrCurrencyMismatch, m.currency, other.currency)
	}
}
