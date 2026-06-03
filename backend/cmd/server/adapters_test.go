package main

import (
	"context"
	"io"
	"log/slog"
	"testing"

	leadsdomain "github.com/daniil/floq/internal/leads/domain"
	"github.com/daniil/floq/internal/prospects"
	prospectsdomain "github.com/daniil/floq/internal/prospects/domain"
	"github.com/google/uuid"
)

type fakeLeadLookup struct {
	lead *leadsdomain.Lead
	err  error
}

func (f *fakeLeadLookup) GetLeadByEmailAddress(_ context.Context, _ uuid.UUID, _ string) (*leadsdomain.Lead, error) {
	return f.lead, f.err
}

type fakeLeadMover struct {
	calls  int
	id     uuid.UUID
	status string
	err    error
}

func (f *fakeLeadMover) UpdateStatus(_ context.Context, id uuid.UUID, status string) error {
	f.calls++
	f.id, f.status = id, status
	return f.err
}

type fakeProspects struct {
	existing   *prospectsdomain.Prospect
	created    *prospects.CreateProspectInput
	createErr  error
	createCall int
}

func (f *fakeProspects) FindByEmail(_ context.Context, _ uuid.UUID, _ string) (*prospectsdomain.Prospect, error) {
	return f.existing, nil
}

func (f *fakeProspects) CreateProspect(_ context.Context, input prospects.CreateProspectInput) (*prospectsdomain.Prospect, error) {
	f.createCall++
	f.created = &input
	return &prospectsdomain.Prospect{}, f.createErr
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func leadWith(status leadsdomain.LeadStatus) *leadsdomain.Lead {
	return &leadsdomain.Lead{ID: uuid.New(), Status: status}
}

func TestOnecApplier_HandlePayment(t *testing.T) {
	tests := []struct {
		name       string
		email      string
		lead       *leadsdomain.Lead
		wantMoved  bool
		wantStatus string
	}{
		{"qualified lead → won", "a@b.ru", leadWith(leadsdomain.StatusQualified), true, "won"},
		{"new lead can't go to won → skip", "a@b.ru", leadWith(leadsdomain.StatusNew), false, ""},
		{"no lead → skip", "a@b.ru", nil, false, ""},
		{"empty email → skip", "", leadWith(leadsdomain.StatusQualified), false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mover := &fakeLeadMover{}
			a := newOnecApplierAdapter(&fakeLeadLookup{lead: tt.lead}, mover, &fakeProspects{}, quietLogger())

			if err := a.HandlePayment(context.Background(), uuid.New(), tt.email); err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if tt.wantMoved && mover.calls != 1 {
				t.Fatalf("expected UpdateStatus call, got %d", mover.calls)
			}
			if !tt.wantMoved && mover.calls != 0 {
				t.Fatalf("expected no transition, got %d calls", mover.calls)
			}
			if tt.wantMoved && mover.status != tt.wantStatus {
				t.Errorf("status = %q, want %q", mover.status, tt.wantStatus)
			}
		})
	}
}

func TestOnecApplier_TransitionTargets(t *testing.T) {
	// From in_conversation, both followup (shipment) and won (payment) are legal;
	// order_status keeps in_conversation (a self-transition is not legal, so it
	// skips). Verifies each handler picks the right target.
	tests := []struct {
		name   string
		invoke func(a *onecApplierAdapter) error
		from   leadsdomain.LeadStatus
		want   string
		moved  bool
	}{
		{"shipment→followup", func(a *onecApplierAdapter) error {
			return a.HandleShipment(context.Background(), uuid.New(), "a@b.ru")
		}, leadsdomain.StatusInConversation, "followup", true},
		{"order→in_conversation from qualified", func(a *onecApplierAdapter) error {
			return a.HandleOrderStatus(context.Background(), uuid.New(), "a@b.ru")
		}, leadsdomain.StatusQualified, "in_conversation", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mover := &fakeLeadMover{}
			a := newOnecApplierAdapter(&fakeLeadLookup{lead: leadWith(tt.from)}, mover, &fakeProspects{}, quietLogger())
			if err := tt.invoke(a); err != nil {
				t.Fatalf("err: %v", err)
			}
			if tt.moved && (mover.calls != 1 || mover.status != tt.want) {
				t.Fatalf("calls=%d status=%q, want 1/%q", mover.calls, mover.status, tt.want)
			}
		})
	}
}

func TestOnecApplier_HandleCounterpartyCreated(t *testing.T) {
	t.Run("new → creates prospect", func(t *testing.T) {
		ps := &fakeProspects{existing: nil}
		a := newOnecApplierAdapter(&fakeLeadLookup{}, &fakeLeadMover{}, ps, quietLogger())

		err := a.HandleCounterpartyCreated(context.Background(), uuid.New(), "a@b.ru", "ООО Тест", "Тест")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if ps.createCall != 1 {
			t.Fatalf("expected CreateProspect, got %d", ps.createCall)
		}
		if ps.created.Name != "ООО Тест" || ps.created.Email != "a@b.ru" {
			t.Errorf("created with wrong fields: %+v", ps.created)
		}
	})

	t.Run("missing name falls back to email", func(t *testing.T) {
		ps := &fakeProspects{existing: nil}
		a := newOnecApplierAdapter(&fakeLeadLookup{}, &fakeLeadMover{}, ps, quietLogger())

		_ = a.HandleCounterpartyCreated(context.Background(), uuid.New(), "a@b.ru", "", "")
		if ps.created.Name != "a@b.ru" {
			t.Errorf("name = %q, want email fallback", ps.created.Name)
		}
	})

	t.Run("existing prospect → no create", func(t *testing.T) {
		ps := &fakeProspects{existing: &prospectsdomain.Prospect{ID: uuid.New()}}
		a := newOnecApplierAdapter(&fakeLeadLookup{}, &fakeLeadMover{}, ps, quietLogger())

		_ = a.HandleCounterpartyCreated(context.Background(), uuid.New(), "a@b.ru", "N", "C")
		if ps.createCall != 0 {
			t.Error("must not create when prospect already exists")
		}
	})

	t.Run("empty email → skip", func(t *testing.T) {
		ps := &fakeProspects{}
		a := newOnecApplierAdapter(&fakeLeadLookup{}, &fakeLeadMover{}, ps, quietLogger())

		_ = a.HandleCounterpartyCreated(context.Background(), uuid.New(), "", "N", "C")
		if ps.createCall != 0 {
			t.Error("empty email must skip")
		}
	})
}
