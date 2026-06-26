package domain

import (
	"errors"
	"strings"
)

// ErrInvalidINN is returned by NewINN when the value is not a structurally
// valid Russian taxpayer number (wrong length, non-digits, or a failed control
// digit). The checksum is what distinguishes a real INN from any 10/12-digit
// string — a stray number scraped off a page rarely satisfies it.
var ErrInvalidINN = errors.New("enrichment: invalid INN")

// INN is a validated Russian taxpayer number (ИНН): 10 digits for a legal
// entity, 12 for a sole proprietor / individual. The zero value is invalid —
// construct via NewINN. Validating the control digit(s) here keeps the
// "what is a real INN" invariant in the domain.
type INN struct{ value string }

// innWeights10 are the control-digit weights for the 10-digit (legal entity) INN.
var innWeights10 = []int{2, 4, 10, 3, 5, 9, 4, 6, 8}

// innWeights12a / innWeights12b are the two control-digit weight sets for the
// 12-digit (individual) INN.
var (
	innWeights12a = []int{7, 2, 4, 10, 3, 5, 9, 4, 6, 8}
	innWeights12b = []int{3, 7, 2, 4, 10, 3, 5, 9, 4, 6, 8}
)

// NewINN parses and validates an INN, trimming surrounding whitespace. It
// rejects anything that is not 10 or 12 digits with correct control digit(s).
func NewINN(s string) (INN, error) {
	s = strings.TrimSpace(s)
	d, ok := digits(s)
	if !ok {
		return INN{}, ErrInvalidINN
	}
	switch len(d) {
	case 10:
		if checkDigit(d[:9], innWeights10) != d[9] {
			return INN{}, ErrInvalidINN
		}
	case 12:
		if checkDigit(d[:10], innWeights12a) != d[10] ||
			checkDigit(d[:11], innWeights12b) != d[11] {
			return INN{}, ErrInvalidINN
		}
	default:
		return INN{}, ErrInvalidINN
	}
	return INN{value: s}, nil
}

// String returns the normalized INN digits.
func (i INN) String() string { return i.value }

// digits converts a string to its per-digit values, reporting ok=false if any
// rune is not an ASCII digit (or the string is empty).
func digits(s string) ([]int, bool) {
	if s == "" {
		return nil, false
	}
	out := make([]int, len(s))
	for idx, r := range s {
		if r < '0' || r > '9' {
			return nil, false
		}
		out[idx] = int(r - '0')
	}
	return out, true
}

// checkDigit computes the Russian control digit: sum(digit*weight) mod 11 mod 10.
func checkDigit(ds, weights []int) int {
	sum := 0
	for i, w := range weights {
		sum += ds[i] * w
	}
	return sum % 11 % 10
}
