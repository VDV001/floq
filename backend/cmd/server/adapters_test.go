package main

import (
	"context"
	"errors"
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
	// The adapter delegates the transition-legality decision to the domain
	// (via the leadMover → usecase → Lead.TransitionTo): it ALWAYS attempts
	// the move for a matched lead and reacts to the result. A benign
	// disallowed-edge (ErrInvalidTransition) is swallowed; any other error
	// propagates. Transition legality itself is the domain's test, not this
	// one — hence the mover simulates the verdict via moverErr.
	otherErr := errors.New("db exploded")
	tests := []struct {
		name      string
		email     string
		lead      *leadsdomain.Lead
		moverErr  error
		wantCalls int
		wantErr   bool
	}{
		{"matched lead → attempts won", "a@b.ru", leadWith(leadsdomain.StatusQualified), nil, 1, false},
		{"illegal transition swallowed", "a@b.ru", leadWith(leadsdomain.StatusNew), leadsdomain.ErrInvalidTransition, 1, false},
		{"other mover error propagates", "a@b.ru", leadWith(leadsdomain.StatusQualified), otherErr, 1, true},
		{"no lead → skip", "a@b.ru", nil, nil, 0, false},
		{"empty email → skip", "", leadWith(leadsdomain.StatusQualified), nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mover := &fakeLeadMover{err: tt.moverErr}
			a := newOnecApplierAdapter(&fakeLeadLookup{lead: tt.lead}, mover, &fakeProspects{}, quietLogger())

			err := a.HandlePayment(context.Background(), uuid.New(), tt.email)
			if tt.wantErr && err == nil {
				t.Fatal("expected error to propagate, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if mover.calls != tt.wantCalls {
				t.Fatalf("UpdateStatus calls = %d, want %d", mover.calls, tt.wantCalls)
			}
			if tt.wantCalls > 0 && mover.status != "won" {
				t.Errorf("status = %q, want won", mover.status)
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
