package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestParseEventKind(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    EventKind
		wantErr error
	}{
		{"payment", "payment", EventKindPayment, nil},
		{"counterparty", "counterparty_created", EventKindCounterpartyCreated, nil},
		{"order status", "order_status", EventKindOrderStatus, nil},
		{"shipment", "shipment", EventKindShipment, nil},
		{"unknown", "delivery", "", ErrInvalidEventKind},
		{"empty", "", "", ErrInvalidEventKind},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEventKind(tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEventKind_IsValid(t *testing.T) {
	valid := []EventKind{EventKindPayment, EventKindCounterpartyCreated, EventKindOrderStatus, EventKindShipment}
	for _, k := range valid {
		if !k.IsValid() {
			t.Errorf("%q should be valid", k)
		}
	}
	if EventKind("nope").IsValid() {
		t.Error("unknown kind should be invalid")
	}
}

func TestNewExternalEvent(t *testing.T) {
	tests := []struct {
		name         string
		externalID   string
		externalType string
		kind         EventKind
		payload      []byte
		wantErr      error
	}{
		{"valid", "ОП-0001", "Документ.ОплатаПокупателя", EventKindPayment, []byte(`{"sum":1000}`), nil},
		{"empty external id", "", "Документ.ОплатаПокупателя", EventKindPayment, []byte(`{}`), ErrEmptyExternalID},
		{"empty external type", "ОП-0001", "", EventKindPayment, []byte(`{}`), ErrEmptyExternalType},
		{"invalid kind", "ОП-0001", "Документ", EventKind("bad"), []byte(`{}`), ErrInvalidEventKind},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev, err := NewExternalEvent(tt.externalID, tt.externalType, tt.kind, tt.payload)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				if ev == nil {
					t.Fatal("expected non-nil event")
				}
				if ev.ExternalID != tt.externalID || ev.Kind != tt.kind {
					t.Fatalf("event fields mismatch: %+v", ev)
				}
			}
		})
	}
}

func TestNewSyncRecord(t *testing.T) {
	user := uuid.New()
	ev, err := NewExternalEvent("ОП-0001", "Документ.ОплатаПокупателя", EventKindPayment, []byte(`{"sum":1000}`))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	t.Run("valid inbound", func(t *testing.T) {
		rec, err := NewSyncRecord(user, ev, DirectionInbound)
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if rec.UserID != user || rec.ExternalID != ev.ExternalID || rec.ExternalType != ev.ExternalType {
			t.Fatalf("record fields mismatch: %+v", rec)
		}
		if rec.Direction != DirectionInbound {
			t.Fatalf("direction = %q", rec.Direction)
		}
		if rec.Status != SyncStatusReceived {
			t.Fatalf("status = %q, want received", rec.Status)
		}
		if rec.PayloadHash == "" {
			t.Fatal("expected payload hash to be computed")
		}
		if rec.ID == uuid.Nil {
			t.Fatal("expected generated ID")
		}
	})

	t.Run("nil user", func(t *testing.T) {
		if _, err := NewSyncRecord(uuid.Nil, ev, DirectionInbound); !errors.Is(err, ErrNilUser) {
			t.Fatalf("err = %v, want ErrNilUser", err)
		}
	})

	t.Run("nil event", func(t *testing.T) {
		if _, err := NewSyncRecord(user, nil, DirectionInbound); !errors.Is(err, ErrNilEvent) {
			t.Fatalf("err = %v, want ErrNilEvent", err)
		}
	})

	t.Run("invalid direction", func(t *testing.T) {
		if _, err := NewSyncRecord(user, ev, SyncDirection("sideways")); !errors.Is(err, ErrInvalidDirection) {
			t.Fatalf("err = %v, want ErrInvalidDirection", err)
		}
	})
}

func TestPayloadHash_StableAndDistinct(t *testing.T) {
	user := uuid.New()
	mk := func(payload string) *SyncRecord {
		ev, _ := NewExternalEvent("ID-1", "Документ", EventKindPayment, []byte(payload))
		rec, _ := NewSyncRecord(user, ev, DirectionInbound)
		return rec
	}
	a, b := mk(`{"sum":1000}`), mk(`{"sum":1000}`)
	if a.PayloadHash != b.PayloadHash {
		t.Error("same payload must hash equally")
	}
	c := mk(`{"sum":2000}`)
	if a.PayloadHash == c.PayloadHash {
		t.Error("different payload must hash differently")
	}
}
