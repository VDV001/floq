package domain

import (
	"errors"
	"testing"
)

func TestParseEventType(t *testing.T) {
	cases := []struct {
		in      string
		want    EventType
		wantErr bool
	}{
		{"lead.created", EventLeadCreated, false},
		{"lead.qualified", EventLeadQualified, false},
		{"lead.archived", EventLeadArchived, false},
		{"pending_reply.approved", EventPendingReplyApproved, false},
		{"sequence.completed", EventSequenceCompleted, false},
		{"  lead.created  ", EventLeadCreated, false}, // trimmed
		{"LEAD.CREATED", EventLeadCreated, false},     // case-insensitive
		{"", "", true},
		{"lead.deleted", "", true},     // unknown
		{"random", "", true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseEventType(c.in)
			if c.wantErr {
				if !errors.Is(err, ErrUnknownEventType) {
					t.Fatalf("ParseEventType(%q): want ErrUnknownEventType, got %v", c.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseEventType(%q): unexpected error %v", c.in, err)
			}
			if got != c.want {
				t.Fatalf("ParseEventType(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestKnownEventTypes_AllParse(t *testing.T) {
	known := KnownEventTypes()
	if len(known) == 0 {
		t.Fatal("KnownEventTypes() returned empty set")
	}
	for _, et := range known {
		if !et.IsKnown() {
			t.Errorf("KnownEventTypes() contains %q but IsKnown()=false", et)
		}
		parsed, err := ParseEventType(string(et))
		if err != nil || parsed != et {
			t.Errorf("round-trip failed for %q: parsed=%q err=%v", et, parsed, err)
		}
	}
}

func TestEventType_IsKnown_RejectsUnknown(t *testing.T) {
	if EventType("nope").IsKnown() {
		t.Error("IsKnown() must be false for an unregistered event type")
	}
}
