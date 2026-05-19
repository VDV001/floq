package inbox

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// --- in-memory fakes ---

type fakePendingReplyRepo struct {
	mu       sync.Mutex
	rows     map[uuid.UUID]*PendingReply
	saveErr  error
	getErr   error
	updErr   error
	listErr  error
	findErr  error
	// dedupOnSave mirrors the partial unique index that production
	// installs in migration 031: when true, Save returns
	// ErrPendingReplyDuplicatePending if a pending row already exists
	// for the same (user_id, lead_id, kind, body) tuple. Off by default
	// so existing tests are unaffected.
	dedupOnSave bool
}

func newFakeRepo() *fakePendingReplyRepo {
	return &fakePendingReplyRepo{rows: map[uuid.UUID]*PendingReply{}}
}

func (f *fakePendingReplyRepo) Save(_ context.Context, pr *PendingReply) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	if f.dedupOnSave {
		for _, row := range f.rows {
			if row.UserID == pr.UserID &&
				row.LeadID == pr.LeadID &&
				row.Kind == pr.Kind &&
				row.Body == pr.Body &&
				row.Status == PendingReplyStatusPending {
				return ErrPendingReplyDuplicatePending
			}
		}
	}
	copy := *pr
	f.rows[pr.ID] = &copy
	return nil
}

func (f *fakePendingReplyRepo) CountPendingByUser(_ context.Context, userID uuid.UUID) (map[uuid.UUID]int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[uuid.UUID]int)
	for _, row := range f.rows {
		if row.UserID == userID && row.Status == PendingReplyStatusPending {
			out[row.LeadID]++
		}
	}
	return out, nil
}

func (f *fakePendingReplyRepo) FindPendingByContent(_ context.Context, userID, leadID uuid.UUID, kind PendingReplyKind, body string) (*PendingReply, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.findErr != nil {
		return nil, f.findErr
	}
	trimmed := strings.TrimSpace(body)
	for _, row := range f.rows {
		if row.UserID == userID &&
			row.LeadID == leadID &&
			row.Kind == kind &&
			row.Body == trimmed &&
			row.Status == PendingReplyStatusPending {
			copy := *row
			return &copy, nil
		}
	}
	return nil, nil
}

func (f *fakePendingReplyRepo) GetByID(_ context.Context, userID, id uuid.UUID) (*PendingReply, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	row, ok := f.rows[id]
	if !ok || row.UserID != userID {
		return nil, nil
	}
	copy := *row
	return &copy, nil
}

func (f *fakePendingReplyRepo) ListByLead(_ context.Context, userID, leadID uuid.UUID) ([]*PendingReply, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := []*PendingReply{}
	for _, row := range f.rows {
		if row.UserID == userID && row.LeadID == leadID {
			copy := *row
			out = append(out, &copy)
		}
	}
	return out, nil
}

func (f *fakePendingReplyRepo) UpdateBody(_ context.Context, pr *PendingReply, expectedStatus PendingReplyStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.updErr != nil {
		return f.updErr
	}
	existing, ok := f.rows[pr.ID]
	if !ok || existing.Status != expectedStatus {
		return ErrPendingReplyNotFound
	}
	// Mirror real repo: body-only column write; do not stamp decided_*.
	existing.Body = pr.Body
	return nil
}

func (f *fakePendingReplyRepo) Update(_ context.Context, pr *PendingReply, expectedStatus PendingReplyStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.updErr != nil {
		return f.updErr
	}
	existing, ok := f.rows[pr.ID]
	if !ok || existing.Status != expectedStatus {
		// Either missing row or the optimistic lock failed — both
		// surface as ErrPendingReplyNotFound to mirror the real
		// repository's uniform-404 contract.
		return ErrPendingReplyNotFound
	}
	copy := *pr
	f.rows[pr.ID] = &copy
	return nil
}

type spyDispatcher struct {
	mu      sync.Mutex
	calls   []*PendingReply
	failErr error
}

func (s *spyDispatcher) Dispatch(_ context.Context, pr *PendingReply) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Snapshot at dispatch time — the usecase mutates the entity to
	// Sent immediately after a successful dispatch returns, so storing
	// a pointer would lose the "what did the dispatcher actually see"
	// signal that the test wants to assert.
	snapshot := *pr
	s.calls = append(s.calls, &snapshot)
	return s.failErr
}

func (s *spyDispatcher) Calls() []*PendingReply {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*PendingReply{}, s.calls...)
}

// --- Propose ---

func TestPendingReplyUseCase_Propose_PersistsAndReturnsEntity(t *testing.T) {
	repo := newFakeRepo()
	disp := &spyDispatcher{}
	uc := NewPendingReplyUseCase(repo, disp)
	ctx := context.Background()
	userID := uuid.New()
	leadID := uuid.New()

	pr, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "hello")
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if pr == nil {
		t.Fatal("Propose returned nil entity")
	}
	if pr.Status != PendingReplyStatusPending {
		t.Errorf("returned status = %v, want pending", pr.Status)
	}

	stored, _ := repo.GetByID(ctx, userID, pr.ID)
	if stored == nil {
		t.Fatal("Propose did not persist the entity")
	}
	if stored.Body != "hello" {
		t.Errorf("stored body = %q, want hello", stored.Body)
	}
	if len(disp.Calls()) != 0 {
		t.Errorf("Propose must NOT dispatch — that's the whole point of the HITL gate")
	}
}

func TestPendingReplyUseCase_Propose_RejectsInvalidInput(t *testing.T) {
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})

	_, err := uc.Propose(context.Background(), uuid.New(), uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "  ")
	if !errors.Is(err, ErrPendingReplyEmptyBody) {
		t.Fatalf("want ErrPendingReplyEmptyBody, got %v", err)
	}
}

func TestPendingReplyUseCase_Propose_PropagatesRepoError(t *testing.T) {
	repo := newFakeRepo()
	repo.saveErr = errors.New("db down")
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})

	_, err := uc.Propose(context.Background(), uuid.New(), uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok")
	if err == nil || !errors.Is(err, repo.saveErr) {
		t.Fatalf("want save error wrapped, got %v", err)
	}
}

func TestPendingReplyUseCase_Propose_DuplicateReturnsExistingEntity(t *testing.T) {
	repo := newFakeRepo()
	repo.dedupOnSave = true
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	ctx := context.Background()
	userID := uuid.New()
	leadID := uuid.New()

	first, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "book me")
	if err != nil {
		t.Fatalf("first Propose error: %v", err)
	}

	second, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "book me")
	if err != nil {
		t.Fatalf("second Propose must be silently idempotent, got error: %v", err)
	}
	if second == nil {
		t.Fatal("second Propose returned nil entity — caller expects the already-enqueued row")
	}
	if second.ID != first.ID {
		t.Errorf("second Propose returned a different entity ID (%v vs %v) — the dedup contract is that the SAME row surfaces both times", second.ID, first.ID)
	}

	// Repo invariant: exactly one row stored.
	listed, err := uc.ListByLead(ctx, userID, leadID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Errorf("rows persisted = %d, want 1 (dedup must collapse the second insert)", len(listed))
	}
}

func TestPendingReplyUseCase_Propose_DuplicateWhitespaceVariantStillDedups(t *testing.T) {
	// Body is trimmed by the domain factory, so whitespace-only
	// variations on the same content must still hit the dedup index.
	repo := newFakeRepo()
	repo.dedupOnSave = true
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	ctx := context.Background()
	userID := uuid.New()
	leadID := uuid.New()

	first, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "book me")
	if err != nil {
		t.Fatal(err)
	}
	second, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "  book me  ")
	if err != nil {
		t.Fatalf("trimmed-equivalent Propose must dedup, got error: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("trimmed-equivalent Propose returned different ID — factory trim + dedup must agree")
	}
}

func TestPendingReplyUseCase_Propose_DifferentBodyAllowedAlongside(t *testing.T) {
	repo := newFakeRepo()
	repo.dedupOnSave = true
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	ctx := context.Background()
	userID := uuid.New()
	leadID := uuid.New()

	first, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "book me")
	if err != nil {
		t.Fatal(err)
	}
	second, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "book me, please")
	if err != nil {
		t.Fatalf("Propose with different body must NOT be deduped, got error: %v", err)
	}
	if second.ID == first.ID {
		t.Error("different bodies must produce different entities")
	}

	listed, err := uc.ListByLead(ctx, userID, leadID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 {
		t.Errorf("rows persisted = %d, want 2 (different bodies are not duplicates)", len(listed))
	}
}

func TestPendingReplyUseCase_Propose_DuplicateButRowDisappearedSurfacesError(t *testing.T) {
	// Save returns ErrPendingReplyDuplicatePending but FindPendingByContent
	// finds nothing — race anomaly: the dedup-causing row was removed
	// between Save and Find. The usecase must NOT silently swallow this;
	// caller deserves an explicit error so the bot logs and humans can
	// investigate.
	repo := newFakeRepo()
	repo.saveErr = ErrPendingReplyDuplicatePending // dedup hit, but no row in store
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})

	_, err := uc.Propose(context.Background(), uuid.New(), uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok")
	if err == nil {
		t.Fatal("dup-without-find-match must surface an error")
	}
	if !errors.Is(err, ErrPendingReplyDuplicatePending) {
		t.Errorf("anomaly error must wrap ErrPendingReplyDuplicatePending so caller can branch, got %v", err)
	}
}

// --- ListByLead ---

func TestPendingReplyUseCase_ListByLead_ScopedByUser(t *testing.T) {
	repo := newFakeRepo()
	disp := &spyDispatcher{}
	uc := NewPendingReplyUseCase(repo, disp)
	ctx := context.Background()
	userA := uuid.New()
	userB := uuid.New()
	leadA := uuid.New()

	_, err := uc.Propose(ctx, userA, leadA, ChannelTelegram, PendingReplyKindBookingLink, "first")
	if err != nil {
		t.Fatal(err)
	}
	_, err = uc.Propose(ctx, userA, leadA, ChannelTelegram, PendingReplyKindBookingLink, "second")
	if err != nil {
		t.Fatal(err)
	}

	got, err := uc.ListByLead(ctx, userA, leadA)
	if err != nil {
		t.Fatalf("ListByLead error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}

	other, err := uc.ListByLead(ctx, userB, leadA)
	if err != nil {
		t.Fatalf("cross-tenant ListByLead error: %v", err)
	}
	if len(other) != 0 {
		t.Errorf("cross-tenant ListByLead returned %d rows, want 0", len(other))
	}
}

// --- Approve ---

func TestPendingReplyUseCase_Approve_TransitionsThenDispatchesThenMarksSent(t *testing.T) {
	repo := newFakeRepo()
	disp := &spyDispatcher{}
	uc := NewPendingReplyUseCase(repo, disp)
	ctx := context.Background()
	userID := uuid.New()
	leadID := uuid.New()

	pr, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "ok")
	if err != nil {
		t.Fatal(err)
	}

	if err := uc.Approve(ctx, userID, pr.ID); err != nil {
		t.Fatalf("Approve error: %v", err)
	}

	calls := disp.Calls()
	if len(calls) != 1 {
		t.Fatalf("dispatcher calls = %d, want 1", len(calls))
	}
	if calls[0].ID != pr.ID {
		t.Errorf("dispatched ID = %v, want %v", calls[0].ID, pr.ID)
	}
	if calls[0].Status != PendingReplyStatusApproved {
		t.Errorf("dispatcher saw status = %v, want approved (dispatcher acts on the approved snapshot)", calls[0].Status)
	}

	stored, _ := repo.GetByID(ctx, userID, pr.ID)
	if stored.Status != PendingReplyStatusSent {
		t.Errorf("stored status = %v, want sent after successful dispatch", stored.Status)
	}
	if stored.DecidedAt == nil {
		t.Error("DecidedAt should be set after Approve")
	}
	if stored.SentAt == nil {
		t.Error("SentAt should be set after successful dispatch")
	}
	if stored.DecidedBy == nil || *stored.DecidedBy != userID {
		t.Errorf("DecidedBy = %v, want userID %v — usecase must pass operator id into domain stamp", stored.DecidedBy, userID)
	}
}

func TestPendingReplyUseCase_Approve_NotFoundReturnsSentinel(t *testing.T) {
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})

	err := uc.Approve(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrPendingReplyNotFound) {
		t.Fatalf("want ErrPendingReplyNotFound, got %v", err)
	}
}

func TestPendingReplyUseCase_Approve_CrossTenantReturnsNotFound(t *testing.T) {
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	ctx := context.Background()
	userA := uuid.New()
	userB := uuid.New()

	pr, err := uc.Propose(ctx, userA, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "owned by A")
	if err != nil {
		t.Fatal(err)
	}

	err = uc.Approve(ctx, userB, pr.ID)
	if !errors.Is(err, ErrPendingReplyNotFound) {
		t.Fatalf("cross-tenant approve must surface ErrPendingReplyNotFound (uniform 404), got %v", err)
	}
}

func TestPendingReplyUseCase_Approve_DispatcherFailureKeepsApprovedAndPropagates(t *testing.T) {
	repo := newFakeRepo()
	disp := &spyDispatcher{failErr: errors.New("telegram api 500")}
	uc := NewPendingReplyUseCase(repo, disp)
	ctx := context.Background()
	userID := uuid.New()

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok")
	if err != nil {
		t.Fatal(err)
	}

	err = uc.Approve(ctx, userID, pr.ID)
	if err == nil {
		t.Fatal("Approve must propagate dispatcher failure")
	}

	stored, _ := repo.GetByID(ctx, userID, pr.ID)
	if stored.Status != PendingReplyStatusApproved {
		t.Errorf("after dispatch failure, status must remain approved (retry-friendly), got %v", stored.Status)
	}
	if stored.DecidedAt == nil {
		t.Error("DecidedAt must be persisted even when dispatch fails — operator did decide")
	}
	if stored.SentAt != nil {
		t.Error("SentAt must remain nil when dispatch failed")
	}
}

func TestPendingReplyUseCase_Approve_LosesRaceToAnotherOperator_MapsToAlreadyDecided(t *testing.T) {
	repo := newFakeRepo()
	disp := &spyDispatcher{}
	uc := NewPendingReplyUseCase(repo, disp)
	ctx := context.Background()
	userID := uuid.New()

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "racey")
	if err != nil {
		t.Fatal(err)
	}

	// Pre-flip the persisted row to status=approved as if another
	// operator beat us to it. fakeRepo.Update with expected=pending
	// will then fail the optimistic check, and the usecase must
	// translate that into ErrPendingReplyAlreadyDecided so the
	// handler answers 409 instead of 500.
	repo.mu.Lock()
	repo.rows[pr.ID].Status = PendingReplyStatusApproved
	repo.mu.Unlock()

	err = uc.Approve(ctx, userID, pr.ID)
	if !errors.Is(err, ErrPendingReplyAlreadyDecided) {
		t.Fatalf("want ErrPendingReplyAlreadyDecided after lost race, got %v", err)
	}
	if len(disp.Calls()) != 0 {
		t.Error("dispatcher MUST NOT fire when the optimistic lock was lost")
	}
}

func TestPendingReplyUseCase_Approve_RejectsAlreadyDecided(t *testing.T) {
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	ctx := context.Background()
	userID := uuid.New()

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok")
	if err != nil {
		t.Fatal(err)
	}
	if err := uc.Approve(ctx, userID, pr.ID); err != nil {
		t.Fatal(err)
	}

	err = uc.Approve(ctx, userID, pr.ID)
	if !errors.Is(err, ErrPendingReplyAlreadyDecided) {
		t.Fatalf("second Approve must return ErrPendingReplyAlreadyDecided, got %v", err)
	}
}

// --- Reject ---

func TestPendingReplyUseCase_Reject_HappyPath(t *testing.T) {
	repo := newFakeRepo()
	disp := &spyDispatcher{}
	uc := NewPendingReplyUseCase(repo, disp)
	ctx := context.Background()
	userID := uuid.New()

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok")
	if err != nil {
		t.Fatal(err)
	}

	before := time.Now().UTC()
	if err := uc.Reject(ctx, userID, pr.ID); err != nil {
		t.Fatalf("Reject error: %v", err)
	}
	after := time.Now().UTC()

	stored, _ := repo.GetByID(ctx, userID, pr.ID)
	if stored.Status != PendingReplyStatusRejected {
		t.Errorf("status = %v, want rejected", stored.Status)
	}
	if stored.DecidedAt == nil || stored.DecidedAt.Before(before) || stored.DecidedAt.After(after) {
		t.Errorf("DecidedAt = %v, want within [%v, %v]", stored.DecidedAt, before, after)
	}
	if stored.DecidedBy == nil || *stored.DecidedBy != userID {
		t.Errorf("DecidedBy = %v, want userID %v — usecase must pass operator id into domain stamp on Reject too", stored.DecidedBy, userID)
	}
	if len(disp.Calls()) != 0 {
		t.Error("Reject must NOT dispatch")
	}
}

func TestPendingReplyUseCase_Reject_NotFoundReturnsSentinel(t *testing.T) {
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})

	err := uc.Reject(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrPendingReplyNotFound) {
		t.Fatalf("want ErrPendingReplyNotFound, got %v", err)
	}
}

func TestPendingReplyUseCase_Approve_WithoutDispatcherReturnsError(t *testing.T) {
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, nil) // nil dispatcher allowed at construction time
	ctx := context.Background()
	userID := uuid.New()

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok")
	if err != nil {
		t.Fatal(err)
	}

	err = uc.Approve(ctx, userID, pr.ID)
	if !errors.Is(err, ErrPendingReplyDispatcherNotConfigured) {
		t.Fatalf("want ErrPendingReplyDispatcherNotConfigured, got %v", err)
	}
	stored, _ := repo.GetByID(ctx, userID, pr.ID)
	if stored.Status == PendingReplyStatusSent {
		t.Error("entity must NOT reach sent state without a dispatcher")
	}
}

func TestPendingReplyUseCase_SetDispatcher_InjectsAtRuntime(t *testing.T) {
	repo := newFakeRepo()
	disp := &spyDispatcher{}
	uc := NewPendingReplyUseCase(repo, nil)
	uc.SetDispatcher(disp)
	ctx := context.Background()
	userID := uuid.New()

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok")
	if err != nil {
		t.Fatal(err)
	}
	if err := uc.Approve(ctx, userID, pr.ID); err != nil {
		t.Fatalf("Approve after SetDispatcher must succeed, got %v", err)
	}
	if len(disp.Calls()) != 1 {
		t.Errorf("dispatcher injected via setter should receive the call, got %d calls", len(disp.Calls()))
	}
}

func TestPendingReplyUseCase_Reject_AlreadyDecided(t *testing.T) {
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	ctx := context.Background()
	userID := uuid.New()

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok")
	if err != nil {
		t.Fatal(err)
	}
	if err := uc.Reject(ctx, userID, pr.ID); err != nil {
		t.Fatal(err)
	}

	err = uc.Reject(ctx, userID, pr.ID)
	if !errors.Is(err, ErrPendingReplyAlreadyDecided) {
		t.Fatalf("second Reject must return ErrPendingReplyAlreadyDecided, got %v", err)
	}
}

// --- UpdateBody (#48) ---

func TestPendingReplyUseCase_UpdateBody_HappyPath(t *testing.T) {
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	ctx := context.Background()
	userID := uuid.New()

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "original")
	if err != nil {
		t.Fatal(err)
	}

	updated, err := uc.UpdateBody(ctx, userID, pr.ID, "  edited  ")
	if err != nil {
		t.Fatalf("UpdateBody returned error: %v", err)
	}
	if updated.Body != "edited" {
		t.Fatalf("returned body = %q, want trimmed 'edited'", updated.Body)
	}
	// Persistence side-effect: refetch shows new body and unchanged status.
	stored, err := repo.GetByID(ctx, userID, pr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Body != "edited" {
		t.Fatalf("stored body = %q, want 'edited'", stored.Body)
	}
	if stored.Status != PendingReplyStatusPending {
		t.Fatalf("status changed during edit: got %v", stored.Status)
	}
}

func TestPendingReplyUseCase_UpdateBody_NotFound(t *testing.T) {
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	ctx := context.Background()

	_, err := uc.UpdateBody(ctx, uuid.New(), uuid.New(), "anything")
	if !errors.Is(err, ErrPendingReplyNotFound) {
		t.Fatalf("want ErrPendingReplyNotFound, got %v", err)
	}
}

func TestPendingReplyUseCase_UpdateBody_CrossTenantIsNotFound(t *testing.T) {
	// Cross-tenant access must collapse to NotFound — never leak existence.
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	ctx := context.Background()
	owner := uuid.New()
	attacker := uuid.New()

	pr, err := uc.Propose(ctx, owner, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "victim body")
	if err != nil {
		t.Fatal(err)
	}

	_, err = uc.UpdateBody(ctx, attacker, pr.ID, "tampered")
	if !errors.Is(err, ErrPendingReplyNotFound) {
		t.Fatalf("cross-tenant must return ErrPendingReplyNotFound, got %v", err)
	}
	// Body must remain untouched.
	stored, _ := repo.GetByID(ctx, owner, pr.ID)
	if stored.Body != "victim body" {
		t.Fatalf("body tampered to %q", stored.Body)
	}
}

func TestPendingReplyUseCase_UpdateBody_AlreadyDecidedMapsTo409(t *testing.T) {
	// Domain returns ErrPendingReplyNotEditable on non-Pending; usecase
	// must surface that as ErrPendingReplyAlreadyDecided so the handler
	// answers a single 409 for both "decided too late" cases (transition
	// race and edit race).
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	ctx := context.Background()
	userID := uuid.New()

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "original")
	if err != nil {
		t.Fatal(err)
	}
	if err := uc.Reject(ctx, userID, pr.ID); err != nil {
		t.Fatal(err)
	}

	_, err = uc.UpdateBody(ctx, userID, pr.ID, "too late")
	if !errors.Is(err, ErrPendingReplyAlreadyDecided) {
		t.Fatalf("want ErrPendingReplyAlreadyDecided, got %v", err)
	}
}

func TestPendingReplyUseCase_UpdateBody_EmptyBodyBubbles(t *testing.T) {
	// Domain factory invariant must bubble unchanged so handler can map
	// to 400 (not 409). The handler distinguishes empty-body input from
	// already-decided.
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	ctx := context.Background()
	userID := uuid.New()

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "original")
	if err != nil {
		t.Fatal(err)
	}

	_, err = uc.UpdateBody(ctx, userID, pr.ID, "   ")
	if !errors.Is(err, ErrPendingReplyEmptyBody) {
		t.Fatalf("want ErrPendingReplyEmptyBody, got %v", err)
	}
}
