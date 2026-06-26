package inbox

import "testing"

func TestEmailSubjectFor(t *testing.T) {
	cases := []struct {
		name string
		kind PendingReplyKind
		want string
	}{
		{"booking link", PendingReplyKindBookingLink, "Запись на встречу"},
		{"unknown kind falls back to generic", PendingReplyKind("mystery"), "Сообщение"},
		{"empty kind falls back to generic", PendingReplyKind(""), "Сообщение"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EmailSubjectFor(tc.kind); got != tc.want {
				t.Fatalf("EmailSubjectFor(%q) = %q, want %q", tc.kind, got, tc.want)
			}
		})
	}
}
