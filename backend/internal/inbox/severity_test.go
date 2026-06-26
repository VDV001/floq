package inbox

import "testing"

func TestSeverityIsValid(t *testing.T) {
	cases := []struct {
		name string
		sev  Severity
		want bool
	}{
		{"info", SeverityInfo, true},
		{"warn", SeverityWarn, true},
		{"block", SeverityBlock, true},
		{"empty", Severity(""), false},
		{"bogus", Severity("critical"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.sev.IsValid(); got != tc.want {
				t.Fatalf("Severity(%q).IsValid() = %v, want %v", tc.sev, got, tc.want)
			}
		})
	}
}

func TestSeverityString(t *testing.T) {
	cases := []struct {
		sev  Severity
		want string
	}{
		{SeverityInfo, "info"},
		{SeverityWarn, "warn"},
		{SeverityBlock, "block"},
	}
	for _, tc := range cases {
		if got := tc.sev.String(); got != tc.want {
			t.Fatalf("Severity(%q).String() = %q, want %q", tc.sev, got, tc.want)
		}
	}
}
