// Package phone provides validation and normalization helpers for
// Tunisian phone numbers used across the checkout flow.
//
// Accepted inputs (case/whitespace-insensitive):
//   - "+216XXXXXXXX"
//   - "00216XXXXXXXX"
//   - "216XXXXXXXX"
//   - "XXXXXXXX"
//
// XXXXXXXX must be exactly 8 digits and start with one of the valid
// Tunisian carrier prefixes: 2, 3, 4, 5, 7 or 9. (1, 6 and 8 are
// reserved for short codes or unused ranges.)
//
// Normalize() returns the canonical "+216XXXXXXXX" form so storage
// remains consistent regardless of how the user typed it.
package phone

import (
	"errors"
	"fmt"
	"strings"
)

// ErrEmpty is returned when an empty phone is provided where one is required.
var ErrEmpty = errors.New("phone number is required")

// ErrInvalid is returned for any input that does not match a valid Tunisian
// mobile/landline number. The message is user-friendly (French) since it
// surfaces in checkout error responses.
var ErrInvalid = errors.New("numéro de téléphone tunisien invalide (8 chiffres, préfixe +216 optionnel)")

// validTNPrefixes are the first-digit prefixes accepted for Tunisian numbers.
var validTNPrefixes = map[byte]struct{}{
	'2': {}, '3': {}, '4': {}, '5': {}, '7': {}, '9': {},
}

// Normalize validates the input and returns the canonical "+216XXXXXXXX"
// representation. It returns ErrEmpty if input is blank and ErrInvalid
// for any malformed value.
func Normalize(raw string) (string, error) {
	cleaned := strip(raw)
	if cleaned == "" {
		return "", ErrEmpty
	}

	digits, err := extractLocalDigits(cleaned)
	if err != nil {
		return "", err
	}

	return "+216" + digits, nil
}

// IsValid is a convenience wrapper for callers that only need a boolean.
func IsValid(raw string) bool {
	_, err := Normalize(raw)
	return err == nil
}

// extractLocalDigits returns the 8-digit local part if cleaned matches a
// supported format, otherwise ErrInvalid.
func extractLocalDigits(cleaned string) (string, error) {
	var local string
	switch {
	case strings.HasPrefix(cleaned, "+216"):
		local = cleaned[4:]
	case strings.HasPrefix(cleaned, "00216"):
		local = cleaned[5:]
	case strings.HasPrefix(cleaned, "216") && len(cleaned) == 11:
		local = cleaned[3:]
	default:
		local = cleaned
	}

	if len(local) != 8 {
		return "", ErrInvalid
	}
	for i := 0; i < len(local); i++ {
		if local[i] < '0' || local[i] > '9' {
			return "", ErrInvalid
		}
	}
	if _, ok := validTNPrefixes[local[0]]; !ok {
		return "", ErrInvalid
	}
	return local, nil
}

// strip removes characters that are typically used for readability
// (spaces, dashes, dots, parentheses) while keeping the leading "+".
func strip(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range strings.TrimSpace(raw) {
		switch r {
		case ' ', '\t', '-', '.', '(', ')':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// MustNormalize is intended for tests/fixtures. It panics on invalid input.
func MustNormalize(raw string) string {
	v, err := Normalize(raw)
	if err != nil {
		panic(fmt.Sprintf("phone.MustNormalize(%q): %v", raw, err))
	}
	return v
}
