package prospects

import (
	"bytes"
	"testing"
)

// detectDelimiter and stripBOM are unexported CSV-preprocessing helpers.
// These direct unit tests pin boundary behaviours that the whole-file
// ImportCSV tests never exercised — each gap let a CONDITIONALS_BOUNDARY
// mutant survive.

func TestDetectDelimiter(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want rune
	}{
		// Equal delimiter counts (here both zero, a single-column header) must
		// default to comma. Pins the "Count(';') > Count(',')" comparison
		// against a ">" → ">=" mutation, which would flip the equal-count
		// default to ';'.
		{"single column defaults to comma", "name", ','},
		{"equal counts default to comma", "a;b,c", ','},
		{"more semicolons wins", "a;b;c", ';'},
		{"more commas wins", "a,b,c", ','},
		// A leading newline: the delimiter is read from the whole payload, not
		// the empty first line. Pins "idx > 0" against an "idx >= 0" mutation,
		// which would truncate firstLine to "" and misdetect the delimiter.
		{"leading newline counts whole payload", "\na;b;c", ';'},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectDelimiter([]byte(tc.in)); got != tc.want {
				t.Errorf("detectDelimiter(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestStripBOM(t *testing.T) {
	bom := []byte{0xEF, 0xBB, 0xBF}
	withBOM := append(append([]byte{}, bom...), []byte("a,b")...)
	cases := []struct {
		name string
		in   []byte
		want []byte
	}{
		// Exactly the 3-byte BOM and nothing else must strip to empty. Pins
		// "len(data) >= 3" against a "len(data) > 3" mutation, which would
		// leave a bare BOM unstripped (corrupting a BOM-only payload).
		{"bom only strips to empty", bom, []byte{}},
		{"bom prefix is stripped", withBOM, []byte("a,b")},
		{"no bom is untouched", []byte("a,b"), []byte("a,b")},
		{"too short to be bom is untouched", []byte{0xEF}, []byte{0xEF}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripBOM(tc.in); !bytes.Equal(got, tc.want) {
				t.Errorf("stripBOM(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
