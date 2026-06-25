package sequences

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the broader sequence/step/outbound IDOR fix (#162): every
// id-addressed operation must be scoped to the authenticated caller. A foreign
// (or missing) resource is rejected with a sentinel the handler maps to 404 —
// the same answer for "doesn't exist" and "exists but isn't yours", so the
// endpoint can't be used to enumerate another tenant's resources.

// --- Use-case level: ownership sentinels ---

func TestGetSequence_ForeignCaller_NotOwned(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: seqID, UserID: uuid.New(), Name: "Stranger's"}}}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	_, err := uc.GetSequence(context.Background(), uuid.New(), seqID)
	require.ErrorIs(t, err, domain.ErrSequenceNotOwned)
}

func TestUpdateSequence_ForeignCaller_NotOwned(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: seqID, UserID: uuid.New(), Name: "Old"}}}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.UpdateSequence(context.Background(), uuid.New(), seqID, "Hacked")
	require.ErrorIs(t, err, domain.ErrSequenceNotOwned)
	assert.Equal(t, "Old", repo.sequences[0].Name, "a foreign sequence must not be renamed")
}

func TestDeleteSequence_ForeignCaller_NotOwned(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: seqID, UserID: uuid.New(), Name: "Stranger's"}}}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.DeleteSequence(context.Background(), uuid.New(), seqID)
	require.ErrorIs(t, err, domain.ErrSequenceNotOwned)
}

func TestToggleActive_ForeignCaller_NotOwned(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: seqID, UserID: uuid.New(), Name: "x"}}}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.ToggleActive(context.Background(), uuid.New(), seqID, true)
	require.ErrorIs(t, err, domain.ErrSequenceNotOwned)
	assert.False(t, repo.sequences[0].IsActive, "a foreign sequence must not be toggled")
}

func TestCreateStep_ForeignSequence_NotOwned(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: seqID, UserID: uuid.New(), Name: "Stranger's"}}}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	step := domain.NewSequenceStep(seqID, 1, 0, domain.StepChannelEmail, "intro", "")
	err := uc.CreateStep(context.Background(), uuid.New(), step)
	require.ErrorIs(t, err, domain.ErrSequenceNotOwned)
}

func TestDeleteStep_ForeignSequence_NotOwned(t *testing.T) {
	seqID := uuid.New()
	stepID := uuid.New()
	repo := &mockRepo{
		sequences: []domain.Sequence{{ID: seqID, UserID: uuid.New(), Name: "Stranger's"}},
		steps:     []domain.SequenceStep{{ID: stepID, SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelEmail}},
	}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.DeleteStep(context.Background(), uuid.New(), stepID)
	require.ErrorIs(t, err, domain.ErrSequenceNotOwned)
}

// Launching a sequence the caller doesn't own leaks the foreign sequence's
// step content (via generated messages) — rejected even when the targeted
// prospects belong to the caller.
func TestLaunch_ForeignSequence_NotOwned(t *testing.T) {
	seqID := uuid.New()
	caller := uuid.New()
	pid := uuid.New()
	repo := &mockRepo{
		sequences: []domain.Sequence{{ID: seqID, UserID: uuid.New(), Name: "Stranger's"}},
		steps:     []domain.SequenceStep{{ID: uuid.New(), SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelEmail}},
	}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{ID: pid, UserID: caller, Status: "new", VerifyStatus: "valid", IsEligibleForSequence: true}
	uc := NewUseCase(repo, &mockAI{coldBody: "hi"}, pr, &mockLeadCreator{})

	err := uc.Launch(context.Background(), caller, seqID, []uuid.UUID{pid}, true)
	require.ErrorIs(t, err, domain.ErrSequenceNotOwned)
	assert.Empty(t, repo.messages, "no message queued for a foreign sequence")
}

func TestApproveMessage_ForeignCaller_NotOwned(t *testing.T) {
	pid := uuid.New()
	msg := domain.NewOutboundMessage(pid, uuid.New(), 1, domain.StepChannelEmail, "hi", time.Now())
	repo := &mockRepo{messages: []*domain.OutboundMessage{msg}}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{ID: pid, UserID: uuid.New()} // owned by a stranger
	uc := NewUseCase(repo, &mockAI{}, pr, &mockLeadCreator{})

	err := uc.ApproveMessage(context.Background(), uuid.New(), msg.ID)
	require.ErrorIs(t, err, domain.ErrMessageNotOwned)
	assert.Equal(t, domain.OutboundStatusDraft, msg.Status, "a foreign message must not be approved")
}

func TestRejectMessage_ForeignCaller_NotOwned(t *testing.T) {
	pid := uuid.New()
	msg := domain.NewOutboundMessage(pid, uuid.New(), 1, domain.StepChannelEmail, "hi", time.Now())
	repo := &mockRepo{messages: []*domain.OutboundMessage{msg}}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{ID: pid, UserID: uuid.New()}
	uc := NewUseCase(repo, &mockAI{}, pr, &mockLeadCreator{})

	err := uc.RejectMessage(context.Background(), uuid.New(), msg.ID)
	require.ErrorIs(t, err, domain.ErrMessageNotOwned)
	assert.Equal(t, domain.OutboundStatusDraft, msg.Status, "a foreign message must not be rejected")
}

func TestEditMessage_ForeignCaller_NotOwned(t *testing.T) {
	pid := uuid.New()
	msg := domain.NewOutboundMessage(pid, uuid.New(), 1, domain.StepChannelEmail, "original", time.Now())
	repo := &mockRepo{messages: []*domain.OutboundMessage{msg}}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{ID: pid, UserID: uuid.New()}
	uc := NewUseCase(repo, &mockAI{}, pr, &mockLeadCreator{})

	err := uc.EditMessage(context.Background(), uuid.New(), msg.ID, "rewritten")
	require.ErrorIs(t, err, domain.ErrMessageNotOwned)
	assert.Empty(t, repo.feedbackSaved, "a foreign message must not be edited")
}

// --- Owner-allowed (lock the allow-path against an over-broad future denial) ---

func TestGetSequence_Owner_Allowed(t *testing.T) {
	seqID := uuid.New()
	caller := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: seqID, UserID: caller, Name: "Mine"}}}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	s, err := uc.GetSequence(context.Background(), caller, seqID)
	require.NoError(t, err)
	require.NotNil(t, s)
	assert.Equal(t, "Mine", s.Name)
}

func TestApproveMessage_Owner_Allowed(t *testing.T) {
	pid := uuid.New()
	caller := uuid.New()
	msg := domain.NewOutboundMessage(pid, uuid.New(), 1, domain.StepChannelEmail, "hi", time.Now())
	repo := &mockRepo{messages: []*domain.OutboundMessage{msg}}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{ID: pid, UserID: caller}
	uc := NewUseCase(repo, &mockAI{}, pr, &mockLeadCreator{})

	require.NoError(t, uc.ApproveMessage(context.Background(), caller, msg.ID))
	assert.Equal(t, domain.OutboundStatusApproved, msg.Status)
}

// A message whose prospect can't be resolved (e.g. a non-existent message id,
// or — were the FK CASCADE ever weakened — an orphaned message) is treated as
// not-owned, never silently authorized.
func TestApproveMessage_ProspectMissing_NotOwned(t *testing.T) {
	pid := uuid.New()
	msg := domain.NewOutboundMessage(pid, uuid.New(), 1, domain.StepChannelEmail, "hi", time.Now())
	repo := &mockRepo{messages: []*domain.OutboundMessage{msg}}
	// prospect reader is empty — the message's prospect cannot be resolved.
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})

	err := uc.ApproveMessage(context.Background(), uuid.New(), msg.ID)
	require.ErrorIs(t, err, domain.ErrMessageNotOwned)
	assert.Equal(t, domain.OutboundStatusDraft, msg.Status)
}

// --- Handler level: foreign resource answers 404 (anti-enumeration) ---

func TestHandler_GetSequence_Foreign_NotFound(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: seqID, UserID: uuid.New(), Name: "Stranger's"}}}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodGet, "/api/sequences/"+seqID.String(), nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandler_UpdateSequence_Foreign_NotFound(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: seqID, UserID: uuid.New(), Name: "Old"}}}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPut, "/api/sequences/"+seqID.String(), map[string]string{"name": "Hacked"})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandler_DeleteSequence_Foreign_NotFound(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: seqID, UserID: uuid.New(), Name: "Stranger's"}}}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodDelete, "/api/sequences/"+seqID.String(), nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandler_ToggleActive_Foreign_NotFound(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: seqID, UserID: uuid.New(), Name: "x"}}}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPatch, "/api/sequences/"+seqID.String()+"/toggle", map[string]bool{"is_active": true})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandler_AddStep_ForeignSequence_NotFound(t *testing.T) {
	seqID := uuid.New()
	repo := &mockRepo{sequences: []domain.Sequence{{ID: seqID, UserID: uuid.New(), Name: "Stranger's"}}}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	body := map[string]interface{}{"step_order": 1, "delay_days": 0, "channel": "email"}
	rr := doRequest(router, http.MethodPost, "/api/sequences/"+seqID.String()+"/steps", body)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandler_DeleteStep_ForeignSequence_NotFound(t *testing.T) {
	seqID := uuid.New()
	stepID := uuid.New()
	repo := &mockRepo{
		sequences: []domain.Sequence{{ID: seqID, UserID: uuid.New(), Name: "Stranger's"}},
		steps:     []domain.SequenceStep{{ID: stepID, SequenceID: seqID, StepOrder: 1, Channel: domain.StepChannelEmail}},
	}
	uc := NewUseCase(repo, &mockAI{}, newMockProspectReader(), &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodDelete, "/api/sequences/"+seqID.String()+"/steps/"+stepID.String(), nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandler_ApproveMessage_Foreign_NotFound(t *testing.T) {
	pid := uuid.New()
	msg := domain.NewOutboundMessage(pid, uuid.New(), 1, domain.StepChannelEmail, "hi", time.Now())
	repo := &mockRepo{messages: []*domain.OutboundMessage{msg}}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{ID: pid, UserID: uuid.New()}
	uc := NewUseCase(repo, &mockAI{}, pr, &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/outbound/"+msg.ID.String()+"/approve", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandler_RejectMessage_Foreign_NotFound(t *testing.T) {
	pid := uuid.New()
	msg := domain.NewOutboundMessage(pid, uuid.New(), 1, domain.StepChannelEmail, "hi", time.Now())
	repo := &mockRepo{messages: []*domain.OutboundMessage{msg}}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{ID: pid, UserID: uuid.New()}
	uc := NewUseCase(repo, &mockAI{}, pr, &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/outbound/"+msg.ID.String()+"/reject", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHandler_EditMessage_Foreign_NotFound(t *testing.T) {
	pid := uuid.New()
	msg := domain.NewOutboundMessage(pid, uuid.New(), 1, domain.StepChannelEmail, "original", time.Now())
	repo := &mockRepo{messages: []*domain.OutboundMessage{msg}}
	pr := newMockProspectReader()
	pr.prospects[pid] = &domain.ProspectView{ID: pid, UserID: uuid.New()}
	uc := NewUseCase(repo, &mockAI{}, pr, &mockLeadCreator{})
	router := setupRouter(uc, uuid.New())

	rr := doRequest(router, http.MethodPost, "/api/outbound/"+msg.ID.String()+"/edit", map[string]string{"body": "rewritten"})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
