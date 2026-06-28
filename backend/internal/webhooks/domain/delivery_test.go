package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewDelivery(t *testing.T) {
	userID := uuid.New()
	epID := uuid.New()
	payload := []byte(`{"event":"lead.created"}`)
	d, err := NewDelivery(userID, epID, EventLeadCreated, payload)
	if err != nil {
		t.Fatalf("NewDelivery: unexpected error %v", err)
	}
	if d.ID == uuid.Nil || d.EventID == uuid.Nil {
		t.Error("delivery must have generated ID and EventID for dedup")
	}
	if d.Status != DeliveryPending {
		t.Errorf("new delivery status = %q, want pending", d.Status)
	}
	if d.Attempts != 0 {
		t.Error("new delivery starts at 0 attempts")
	}
}

func TestNewDelivery_RejectsEmptyPayload(t *testing.T) {
	if _, err := NewDelivery(uuid.New(), uuid.New(), EventLeadCreated, nil); err == nil {
		t.Error("NewDelivery must reject empty payload")
	}
}

func TestDelivery_MarkDelivered(t *testing.T) {
	d, _ := NewDelivery(uuid.New(), uuid.New(), EventLeadCreated, []byte(`{}`))
	d.MarkDelivered(200, time.Now())
	if d.Status != DeliverySucceeded {
		t.Errorf("status = %q, want succeeded", d.Status)
	}
	if d.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", d.Attempts)
	}
	if d.HTTPStatus != 200 {
		t.Errorf("http status = %d, want 200", d.HTTPStatus)
	}
	if d.DeliveredAt == nil {
		t.Error("DeliveredAt must be set on success")
	}
}

// MarkFailed increments attempts and only becomes terminal once maxAttempts is
// reached; before that the row stays pending so the worker retries it.
func TestDelivery_MarkFailed_RetryThenExhaust(t *testing.T) {
	d, _ := NewDelivery(uuid.New(), uuid.New(), EventLeadCreated, []byte(`{}`))
	const maxAttempts = 3
	now := time.Unix(1_700_000_000, 0).UTC()

	d.MarkFailed("dial timeout", 0, maxAttempts, now)
	if d.Status != DeliveryPending {
		t.Errorf("after attempt 1 status = %q, want pending (retryable)", d.Status)
	}
	if d.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", d.Attempts)
	}
	if d.NextRetryAt == nil || !d.NextRetryAt.After(now) {
		t.Error("retryable failure must schedule NextRetryAt in the future")
	}

	d.MarkFailed("503", 503, maxAttempts, now)
	if d.Status != DeliveryPending {
		t.Errorf("after attempt 2 status = %q, want pending", d.Status)
	}

	d.MarkFailed("503", 503, maxAttempts, now)
	if d.Status != DeliveryFailed {
		t.Errorf("after attempt 3 status = %q, want failed (exhausted)", d.Status)
	}
	if d.Error == "" {
		t.Error("terminal failure must record the last error")
	}
	if d.NextRetryAt != nil {
		t.Error("terminal failure must clear NextRetryAt")
	}
}

func TestDelivery_NextRetryBackoff(t *testing.T) {
	d, _ := NewDelivery(uuid.New(), uuid.New(), EventLeadCreated, []byte(`{}`))
	base := time.Unix(1_700_000_000, 0).UTC()

	d.MarkFailed("err", 500, 5, base)
	first := d.NextRetryAfter(base)
	d.MarkFailed("err", 500, 5, base)
	second := d.NextRetryAfter(base)

	if !first.After(base) {
		t.Error("first retry must be scheduled after the base time")
	}
	if !second.After(first) {
		t.Error("backoff must grow with attempts (exponential)")
	}
}

// The terminal-status set is the single source the retention GC sweeps; pin it so
// a new terminal status can't be added to the enum without joining the set (#212).
func TestTerminalDeliveryStatuses_IsTheTerminalSet(t *testing.T) {
	got := TerminalDeliveryStatuses()
	want := map[DeliveryStatus]bool{DeliverySucceeded: true, DeliveryFailed: true}
	if len(got) != len(want) {
		t.Fatalf("terminal set size = %d, want %d", len(got), len(want))
	}
	for _, s := range got {
		if !want[s] {
			t.Errorf("unexpected terminal status %q in set", s)
		}
		if s == DeliveryPending {
			t.Error("pending is never terminal")
		}
	}
}
