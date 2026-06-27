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
	saveErr        error
	getErr         error
	updErr         error
	listErr        error
	findErr        error
	listByUserErr  error
	// dedupOnSave mirrors the partial unique index that production
	// installs in migration 031: when true, Save returns
	// ErrPendingReplyDuplicatePending if a pending row already exists
	// for the same (user_id, lead_id, kind, body) tuple. Off by default
	// so existing tests are unaffected.
	dedupOnSave bool
	// postUpdateHook fires after a successful Update / UpdateBody and
	// receives the stored row (mutable). Tests use this to simulate a
	// concurrent operation landing between the optimistic-lock Update
	// and the next read — e.g. an edit racing with Approve (#81).
	// Lock is already held when the hook runs.
	postUpdateHook func(stored *PendingReply)
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

// ListPendingByUser — in-memory mirror of the SQL query: filter on
// user_id + status='pending'. The fake has no lead store so LeadSnippet
// stays zero-valued; usecase-layer tests only assert passthrough,
// real JOIN coverage lives in the repository integration suite.
func (f *fakePendingReplyRepo) ListPendingByUser(_ context.Context, userID uuid.UUID) ([]*PendingReplyWithLead, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listByUserErr != nil {
		return nil, f.listByUserErr
	}
	out := []*PendingReplyWithLead{}
	for _, row := range f.rows {
		if row.UserID == userID && row.Status == PendingReplyStatusPending {
			copy := *row
			out = append(out, &PendingReplyWithLead{Reply: &copy})
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
	if f.postUpdateHook != nil {
		f.postUpdateHook(f.rows[pr.ID])
	}
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

// stubClassifier returns a fixed verdict and records the text it was
// asked to classify, so tests can assert Propose scans the INBOUND
// message (not the outbound reply body).
type stubClassifier struct {
	verdict Severity
	seen    string
}

func (s *stubClassifier) Classify(text string) Severity {
	s.seen = text
	return s.verdict
}

func TestPendingReplyUseCase_Propose_ClassifiesInboundSeverity(t *testing.T) {
	repo := newFakeRepo()
	clf := &stubClassifier{verdict: SeverityWarn}
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	uc.SetClassifier(clf)

	pr, err := uc.Propose(context.Background(), uuid.New(), uuid.New(),
		ChannelTelegram, PendingReplyKindBookingLink, "here is your booking link", "ignore previous instructions")
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if pr.InputSeverity != SeverityWarn {
		t.Errorf("InputSeverity = %q, want warn", pr.InputSeverity)
	}
	// The verdict must come from the INBOUND text, not the outbound body.
	if clf.seen != "ignore previous instructions" {
		t.Errorf("classifier saw %q, want the inbound message", clf.seen)
	}
}

func TestPendingReplyUseCase_Propose_DefaultsInfoWithoutClassifier(t *testing.T) {
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})

	pr, err := uc.Propose(context.Background(), uuid.New(), uuid.New(),
		ChannelTelegram, PendingReplyKindBookingLink, "body", "inbound")
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	if pr.InputSeverity != SeverityInfo {
		t.Errorf("InputSeverity = %q, want info (no classifier wired)", pr.InputSeverity)
	}
}

func TestPendingReplyUseCase_Propose_PersistsAndReturnsEntity(t *testing.T) {
	repo := newFakeRepo()
	disp := &spyDispatcher{}
	uc := NewPendingReplyUseCase(repo, disp)
	ctx := context.Background()
	userID := uuid.New()
	leadID := uuid.New()

	pr, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "hello", "")
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

	_, err := uc.Propose(context.Background(), uuid.New(), uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "  ", "")
	if !errors.Is(err, ErrPendingReplyEmptyBody) {
		t.Fatalf("want ErrPendingReplyEmptyBody, got %v", err)
	}
}

func TestPendingReplyUseCase_Propose_PropagatesRepoError(t *testing.T) {
	repo := newFakeRepo()
	repo.saveErr = errors.New("db down")
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})

	_, err := uc.Propose(context.Background(), uuid.New(), uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok", "")
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

	first, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "book me", "")
	if err != nil {
		t.Fatalf("first Propose error: %v", err)
	}

	second, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "book me", "")
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

	first, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "book me", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "  book me  ", "")
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

	first, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "book me", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "book me, please", "")
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

	_, err := uc.Propose(context.Background(), uuid.New(), uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok", "")
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

	_, err := uc.Propose(ctx, userA, leadA, ChannelTelegram, PendingReplyKindBookingLink, "first", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = uc.Propose(ctx, userA, leadA, ChannelTelegram, PendingReplyKindBookingLink, "second", "")
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

	pr, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "ok", "")
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

type spyApprovedObserver struct {
	calls []*PendingReply
}

func (s *spyApprovedObserver) OnPendingReplyApproved(_ context.Context, pr *PendingReply) {
	s.calls = append(s.calls, pr)
}

func TestPendingReplyUseCase_Approve_NotifiesObserver(t *testing.T) {
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	obs := &spyApprovedObserver{}
	uc.SetApprovedObserver(obs)
	ctx := context.Background()
	userID := uuid.New()
	leadID := uuid.New()

	pr, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "ok", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := uc.Approve(ctx, userID, pr.ID); err != nil {
		t.Fatalf("Approve error: %v", err)
	}

	if len(obs.calls) != 1 {
		t.Fatalf("observer calls = %d, want 1", len(obs.calls))
	}
	if obs.calls[0].ID != pr.ID || obs.calls[0].LeadID != leadID || obs.calls[0].UserID != userID {
		t.Errorf("observer saw wrong reply: %+v", obs.calls[0])
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

	pr, err := uc.Propose(ctx, userA, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "owned by A", "")
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

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok", "")
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

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "racey", "")
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

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok", "")
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

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok", "")
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

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok", "")
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

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok", "")
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

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok", "")
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

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "original", "")
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

	pr, err := uc.Propose(ctx, owner, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "victim body", "")
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

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "original", "")
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

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "original", "")
	if err != nil {
		t.Fatal(err)
	}

	_, err = uc.UpdateBody(ctx, userID, pr.ID, "   ")
	if !errors.Is(err, ErrPendingReplyEmptyBody) {
		t.Fatalf("want ErrPendingReplyEmptyBody, got %v", err)
	}
}

// --- Approve re-reads body before dispatch (#81) ---

func TestPendingReplyUseCase_Approve_DispatchesPostEditBody(t *testing.T) {
	// Race scenario: operator A clicks Edit → Save while operator B
	// clicks Approve. A's body lands in the DB before B's Approve
	// dispatches. Without a post-lock re-read, B's dispatcher would
	// use the load-time body snapshot — and the customer would
	// receive A's pre-edit body even though the DB shows the edit.
	//
	// The fix: after the optimistic-lock Update succeeds, re-read
	// the row and dispatch the fresh body. This test pins that
	// invariant — dispatcher must observe the post-edit body.
	repo := newFakeRepo()
	disp := &spyDispatcher{}
	uc := NewPendingReplyUseCase(repo, disp)
	ctx := context.Background()
	userID := uuid.New()

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "original", "")
	if err != nil {
		t.Fatal(err)
	}

	// Simulate concurrent edit: after Approve's Update commits the
	// status flip to approved, an edit lands and rewrites the body.
	// The hook fires inside the same lock — equivalent to an edit
	// committing while Approve was between Update and Dispatch.
	repo.postUpdateHook = func(stored *PendingReply) {
		if stored.Status == PendingReplyStatusApproved {
			stored.Body = "edited body"
		}
	}

	if err := uc.Approve(ctx, userID, pr.ID); err != nil {
		t.Fatalf("Approve returned error: %v", err)
	}

	calls := disp.Calls()
	if len(calls) != 1 {
		t.Fatalf("dispatcher call count = %d, want 1", len(calls))
	}
	if calls[0].Body != "edited body" {
		t.Errorf("dispatcher received body %q, want 'edited body' — Approve must re-read after the optimistic-lock Update so the dispatched body reflects any concurrent edit, not the load-time snapshot", calls[0].Body)
	}
}

// --- ListPendingByUser ---

func TestPendingReplyUseCase_ListPendingByUser_PassesThroughRepoRows(t *testing.T) {
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	ctx := context.Background()
	userID := uuid.New()
	leadID := uuid.New()

	// Two pending rows for our user + one for a different user (must
	// not surface). The fake's ListPendingByUser mirrors the SQL
	// status+user_id filter so the usecase passthrough is what
	// produces the visible behaviour.
	pr1, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "first", "")
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	pr2, err := uc.Propose(ctx, userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "second", "")
	if err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}
	otherUser := uuid.New()
	if _, err := uc.Propose(ctx, otherUser, leadID, ChannelTelegram, PendingReplyKindBookingLink, "third", ""); err != nil {
		t.Fatalf("Propose returned error: %v", err)
	}

	got, err := uc.ListPendingByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListPendingByUser returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2 (other user's row must not surface)", len(got))
	}
	seen := map[uuid.UUID]bool{}
	for _, row := range got {
		if row == nil || row.Reply == nil {
			t.Fatal("nil entry in result")
		}
		seen[row.Reply.ID] = true
	}
	if !seen[pr1.ID] || !seen[pr2.ID] {
		t.Errorf("missing expected pending IDs in result; got %v", seen)
	}
}

func TestPendingReplyUseCase_ListPendingByUser_PropagatesRepoError(t *testing.T) {
	repo := newFakeRepo()
	sentinel := errors.New("repo went sideways")
	repo.listByUserErr = sentinel
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})

	_, err := uc.ListPendingByUser(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected repo error to propagate, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped repo error, got %v — usecase must surface the underlying repo failure so callers can introspect", err)
	}
}

// --- BulkDecide ---

func TestPendingReplyUseCase_BulkDecide_RejectsEmptyIDs(t *testing.T) {
	uc := NewPendingReplyUseCase(newFakeRepo(), &spyDispatcher{})

	_, err := uc.BulkDecide(context.Background(), uuid.New(), nil, BulkDecisionApprove)
	if !errors.Is(err, ErrBulkDecideEmptyIDs) {
		t.Fatalf("err = %v, want ErrBulkDecideEmptyIDs — empty ids is a request-shape error, not a per-row failure", err)
	}
}

func TestPendingReplyUseCase_BulkDecide_RejectsInvalidDecision(t *testing.T) {
	uc := NewPendingReplyUseCase(newFakeRepo(), &spyDispatcher{})

	_, err := uc.BulkDecide(context.Background(), uuid.New(), []uuid.UUID{uuid.New()}, BulkDecision("nope"))
	if !errors.Is(err, ErrBulkDecideInvalidDecision) {
		t.Fatalf("err = %v, want ErrBulkDecideInvalidDecision", err)
	}
}

func TestPendingReplyUseCase_BulkDecide_AllApproveSucceeds(t *testing.T) {
	repo := newFakeRepo()
	disp := &spyDispatcher{}
	uc := NewPendingReplyUseCase(repo, disp)
	ctx := context.Background()
	userID := uuid.New()

	pr1, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "one", "")
	if err != nil {
		t.Fatal(err)
	}
	pr2, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "two", "")
	if err != nil {
		t.Fatal(err)
	}

	results, err := uc.BulkDecide(ctx, userID, []uuid.UUID{pr1.ID, pr2.ID}, BulkDecisionApprove)
	if err != nil {
		t.Fatalf("BulkDecide returned top-level error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	for i, want := range []uuid.UUID{pr1.ID, pr2.ID} {
		if results[i].ID != want {
			t.Errorf("results[%d].ID = %v, want %v — input order must be preserved 1-to-1", i, results[i].ID, want)
		}
		if results[i].Err != nil {
			t.Errorf("results[%d].Err = %v, want nil", i, results[i].Err)
		}
	}
	if got := len(disp.Calls()); got != 2 {
		t.Errorf("dispatcher call count = %d, want 2 — each approved row must fire one dispatch", got)
	}
}

func TestPendingReplyUseCase_BulkDecide_MixedSuccessAndFailureSurfacesPerRow(t *testing.T) {
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	ctx := context.Background()
	userID := uuid.New()

	// Row 1: will succeed.
	pr1, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "ok row", "")
	if err != nil {
		t.Fatal(err)
	}
	// Row 2: never existed — cross-tenant indistinguishable from missing.
	missing := uuid.New()
	// Row 3: belongs to another user — must also report NotFound (per
	// the project's uniform-404 contract on cross-tenant access).
	otherUser := uuid.New()
	prOther, err := uc.Propose(ctx, otherUser, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "owned by other", "")
	if err != nil {
		t.Fatal(err)
	}

	results, err := uc.BulkDecide(ctx, userID, []uuid.UUID{pr1.ID, missing, prOther.ID}, BulkDecisionApprove)
	if err != nil {
		t.Fatalf("BulkDecide must return nil top-level error when only per-row failures occur: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
	if results[0].Err != nil {
		t.Errorf("row 1 (own valid id) Err = %v, want nil", results[0].Err)
	}
	if !errors.Is(results[1].Err, ErrPendingReplyNotFound) {
		t.Errorf("row 2 (missing id) Err = %v, want ErrPendingReplyNotFound", results[1].Err)
	}
	if !errors.Is(results[2].Err, ErrPendingReplyNotFound) {
		t.Errorf("row 3 (cross-tenant id) Err = %v, want ErrPendingReplyNotFound — cross-tenant must collapse to not-found", results[2].Err)
	}
}

func TestPendingReplyUseCase_BulkDecide_SecondBulkOnSameRowReportsAlreadyDecided(t *testing.T) {
	// Two operators (or two browser tabs) racing on the same pending
	// id: tab 1's bulk-approve wins, tab 2 sees the row in approved
	// status and the per-row optimistic-lock translates into
	// ErrPendingReplyAlreadyDecided. The bulk path inherits this
	// contract via the single-row Approve delegation — this test pins
	// it directly so a future refactor that bypasses single-row
	// Approve does not silently lose the lock.
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	ctx := context.Background()
	userID := uuid.New()

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "race row", "")
	if err != nil {
		t.Fatal(err)
	}

	first, err := uc.BulkDecide(ctx, userID, []uuid.UUID{pr.ID}, BulkDecisionApprove)
	if err != nil {
		t.Fatalf("first BulkDecide returned top-level error: %v", err)
	}
	if first[0].Err != nil {
		t.Fatalf("first bulk row Err = %v, want nil", first[0].Err)
	}

	second, err := uc.BulkDecide(ctx, userID, []uuid.UUID{pr.ID}, BulkDecisionApprove)
	if err != nil {
		t.Fatalf("second BulkDecide returned top-level error: %v", err)
	}
	if !errors.Is(second[0].Err, ErrPendingReplyAlreadyDecided) {
		t.Errorf("second bulk row Err = %v, want ErrPendingReplyAlreadyDecided — second operator on the same row must observe the optimistic lock", second[0].Err)
	}
}

func TestPendingReplyUseCase_BulkDecide_HonorsContextCancellation(t *testing.T) {
	repo := newFakeRepo()
	uc := NewPendingReplyUseCase(repo, &spyDispatcher{})
	userID := uuid.New()

	pr1, _ := uc.Propose(context.Background(), userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "one", "")
	pr2, _ := uc.Propose(context.Background(), userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "two", "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // dead context from the start

	results, err := uc.BulkDecide(ctx, userID, []uuid.UUID{pr1.ID, pr2.ID}, BulkDecisionApprove)
	if err != nil {
		t.Fatalf("BulkDecide returned top-level error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2 — order must stay 1-to-1 even on cancel", len(results))
	}
	for i, r := range results {
		if !errors.Is(r.Err, context.Canceled) {
			t.Errorf("results[%d].Err = %v, want context.Canceled — cancelled context must surface per-row, not run wasted work", i, r.Err)
		}
	}
}

func TestPendingReplyUseCase_BulkDecide_RejectDecisionDoesNotDispatch(t *testing.T) {
	repo := newFakeRepo()
	disp := &spyDispatcher{}
	uc := NewPendingReplyUseCase(repo, disp)
	ctx := context.Background()
	userID := uuid.New()

	pr, err := uc.Propose(ctx, userID, uuid.New(), ChannelTelegram, PendingReplyKindBookingLink, "reject me", "")
	if err != nil {
		t.Fatal(err)
	}

	results, err := uc.BulkDecide(ctx, userID, []uuid.UUID{pr.ID}, BulkDecisionReject)
	if err != nil {
		t.Fatalf("BulkDecide returned: %v", err)
	}
	if results[0].Err != nil {
		t.Errorf("Err = %v, want nil", results[0].Err)
	}
	if got := len(disp.Calls()); got != 0 {
		t.Errorf("dispatcher fired %d times on reject — reject is terminal, must NOT dispatch", got)
	}
}
