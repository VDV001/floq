package domain

import (
	"errors"
	"strings"
)

// ErrInvalidOGRN is returned by NewOGRN when the value is not a structurally
// valid primary state registration number (wrong length, non-digits, or a
// failed control digit).
var ErrInvalidOGRN = errors.New("enrichment: invalid OGRN")

// OGRN is a validated Russian primary state registration number (ОГРН): 13
// digits for an organization, 15 for a sole proprietor (ОГРНИП). The zero
// value is invalid — construct via NewOGRN.
type OGRN struct{ value string }

// NewOGRN parses and validates an OGRN, trimming surrounding whitespace.
// Control digit: 13-digit uses mod 11 over the first 12 digits; 15-digit uses
// mod 13 over the first 14; the remainder's last digit must match the final
// digit.
func NewOGRN(s string) (OGRN, error) {
	s = strings.TrimSpace(s)
	d, ok := digits(s)
	if !ok {
		return OGRN{}, ErrInvalidOGRN
	}
	switch len(d) {
	case 13:
		if modN(d[:12], 11)%10 != d[12] {
			return OGRN{}, ErrInvalidOGRN
		}
	case 15:
		if modN(d[:14], 13)%10 != d[14] {
			return OGRN{}, ErrInvalidOGRN
		}
	default:
		return OGRN{}, ErrInvalidOGRN
	}
	return OGRN{value: s}, nil
}

// String returns the normalized OGRN digits.
func (o OGRN) String() string { return o.value }

// modN returns the number formed by ds modulo n, computed iteratively so no
// intermediate value overflows regardless of digit count.
func modN(ds []int, n int) int {
	acc := 0
	for _, d := range ds {
		acc = (acc*10 + d) % n
	}
	return acc
}
