package domain

import (
	"errors"
	"testing"
)

func TestLead_Archive_SetsArchivedAtPreservesStatus(t *testing.T) {
	lead := &Lead{Status: StatusQualified}
	if err := lead.Archive(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lead.ArchivedAt == nil {
		t.Fatal("expected ArchivedAt to be set")
	}
	if !lead.IsArchived() {
		t.Error("expected IsArchived() to be true")
	}
	// Archive is ORTHOGONAL to the pipeline status — it must not touch it.
	if lead.Status != StatusQualified {
		t.Errorf("status changed by Archive: got %s", lead.Status)
	}
	if !lead.UpdatedAt.Equal(*lead.ArchivedAt) {
		t.Error("expected UpdatedAt to be bumped to ArchivedAt")
	}
}

func TestLead_Archive_WorksOnTerminalStatus(t *testing.T) {
	// 'closed' is terminal for transitions, but archive is orthogonal and must
	// still succeed on it — the whole point of the flag (vs status='closed').
	lead := &Lead{Status: StatusClosed}
	if err := lead.Archive(); err != nil {
		t.Fatalf("archive on closed lead failed: %v", err)
	}
	if lead.Status != StatusClosed {
		t.Errorf("status changed: got %s", lead.Status)
	}
}

func TestLead_Archive_Idempotent_ReturnsErrAlreadyArchived(t *testing.T) {
	lead := &Lead{Status: StatusNew}
	if err := lead.Archive(); err != nil {
		t.Fatalf("first archive failed: %v", err)
	}
	first := *lead.ArchivedAt

	err := lead.Archive()
	if !errors.Is(err, ErrAlreadyArchived) {
		t.Fatalf("expected ErrAlreadyArchived, got %v", err)
	}
	if !lead.ArchivedAt.Equal(first) {
		t.Error("ArchivedAt must not change on a rejected re-archive")
	}
}

func TestLead_Unarchive_ClearsArchivedAt(t *testing.T) {
	lead := &Lead{Status: StatusNew}
	if err := lead.Archive(); err != nil {
		t.Fatalf("archive failed: %v", err)
	}
	if err := lead.Unarchive(); err != nil {
		t.Fatalf("unarchive failed: %v", err)
	}
	if lead.ArchivedAt != nil {
		t.Error("expected ArchivedAt to be nil after Unarchive")
	}
	if lead.IsArchived() {
		t.Error("expected IsArchived() to be false")
	}
}

func TestLead_Unarchive_NotArchived_ReturnsErrNotArchived(t *testing.T) {
	lead := &Lead{Status: StatusNew}
	err := lead.Unarchive()
	if !errors.Is(err, ErrNotArchived) {
		t.Fatalf("expected ErrNotArchived, got %v", err)
	}
}

func TestLead_IsArchived(t *testing.T) {
	tests := []struct {
		name     string
		archived bool
		want     bool
	}{
		{"fresh lead", false, false},
		{"archived lead", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lead := &Lead{Status: StatusNew}
			if tt.archived {
				_ = lead.Archive()
			}
			if got := lead.IsArchived(); got != tt.want {
				t.Errorf("IsArchived() = %v, want %v", got, tt.want)
			}
		})
	}
}
