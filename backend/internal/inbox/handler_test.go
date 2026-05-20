package inbox

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// --- fakes ---

type fakePendingReplyUseCase struct {
	mu sync.Mutex

	listFn              func(ctx context.Context, userID, leadID uuid.UUID) ([]*PendingReply, error)
	listPendingByUserFn func(ctx context.Context, userID uuid.UUID) ([]*PendingReplyWithLead, error)
	approveFn           func(ctx context.Context, userID, id uuid.UUID) error
	rejectFn            func(ctx context.Context, userID, id uuid.UUID) error
	updateBodyFn        func(ctx context.Context, userID, id uuid.UUID, body string) (*PendingReply, error)
}

func (f *fakePendingReplyUseCase) ListByLead(ctx context.Context, userID, leadID uuid.UUID) ([]*PendingReply, error) {
	if f.listFn != nil {
		return f.listFn(ctx, userID, leadID)
	}
	return nil, nil
}

func (f *fakePendingReplyUseCase) ListPendingByUser(ctx context.Context, userID uuid.UUID) ([]*PendingReplyWithLead, error) {
	if f.listPendingByUserFn != nil {
		return f.listPendingByUserFn(ctx, userID)
	}
	return nil, nil
}

func (f *fakePendingReplyUseCase) Approve(ctx context.Context, userID, id uuid.UUID) error {
	if f.approveFn != nil {
		return f.approveFn(ctx, userID, id)
	}
	return nil
}

func (f *fakePendingReplyUseCase) Reject(ctx context.Context, userID, id uuid.UUID) error {
	if f.rejectFn != nil {
		return f.rejectFn(ctx, userID, id)
	}
	return nil
}

func (f *fakePendingReplyUseCase) UpdateBody(ctx context.Context, userID, id uuid.UUID, body string) (*PendingReply, error) {
	if f.updateBodyFn != nil {
		return f.updateBodyFn(ctx, userID, id, body)
	}
	return nil, nil
}

type fakeLeadOwnership struct {
	owned map[uuid.UUID]bool
	err   error
}

func (f *fakeLeadOwnership) OwnsLead(_ context.Context, _ uuid.UUID, leadID uuid.UUID) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.owned[leadID], nil
}

// --- helpers ---

func newTestServer(uc PendingReplyUseCaseAPI, leads LeadOwnershipChecker) http.Handler {
	r := chi.NewRouter()
	RegisterPendingReplyRoutes(r, uc, leads, nil)
	return r
}

func authedRequest(t *testing.T, method, path string, userID uuid.UUID) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req = req.WithContext(httputil.WithUserID(req.Context(), userID))
	return req
}

// --- Response DTO carries DecidedBy ---

func TestHandler_ListByLead_ResponseIncludesDecidedBy(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	operator := uuid.New()

	pr, err := NewPendingReply(userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "first")
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a row that was approved by 'operator' — the response
	// must carry decided_by as a string. omitempty would otherwise
	// drop a non-stamped row's field, but here it MUST be present.
	if err := pr.Approve(time.Now().UTC(), operator); err != nil {
		t.Fatal(err)
	}

	uc := &fakePendingReplyUseCase{
		listFn: func(_ context.Context, _, _ uuid.UUID) ([]*PendingReply, error) {
			return []*PendingReply{pr}, nil
		},
	}
	leads := &fakeLeadOwnership{owned: map[uuid.UUID]bool{leadID: true}}

	srv := newTestServer(uc, leads)
	req := authedRequest(t, http.MethodGet, "/api/leads/"+leadID.String()+"/pending-replies", userID)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var got []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("response len = %d, want 1", len(got))
	}
	if got[0]["decided_by"] != operator.String() {
		t.Errorf("decided_by = %v, want %v — DTO must carry attribution for decided rows", got[0]["decided_by"], operator)
	}
}

// --- GET /api/leads/{id}/pending-replies ---

func TestHandler_ListByLead_HappyPath(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	pr, err := NewPendingReply(userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "first")
	if err != nil {
		t.Fatal(err)
	}

	uc := &fakePendingReplyUseCase{
		listFn: func(_ context.Context, u, l uuid.UUID) ([]*PendingReply, error) {
			if u != userID || l != leadID {
				t.Errorf("usecase received wrong ids: u=%v l=%v", u, l)
			}
			return []*PendingReply{pr}, nil
		},
	}
	leads := &fakeLeadOwnership{owned: map[uuid.UUID]bool{leadID: true}}

	srv := newTestServer(uc, leads)
	req := authedRequest(t, http.MethodGet, "/api/leads/"+leadID.String()+"/pending-replies", userID)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var got []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("response len = %d, want 1", len(got))
	}
	if got[0]["id"] != pr.ID.String() {
		t.Errorf("id = %v, want %v", got[0]["id"], pr.ID)
	}
	if got[0]["status"] != "pending" {
		t.Errorf("status field = %v, want pending", got[0]["status"])
	}
	if got[0]["body"] != "first" {
		t.Errorf("body field = %v", got[0]["body"])
	}
}

func TestHandler_ListByLead_EmptyReturnsJSONArrayNotNull(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	uc := &fakePendingReplyUseCase{
		listFn: func(_ context.Context, _, _ uuid.UUID) ([]*PendingReply, error) {
			return nil, nil
		},
	}
	leads := &fakeLeadOwnership{owned: map[uuid.UUID]bool{leadID: true}}

	srv := newTestServer(uc, leads)
	req := authedRequest(t, http.MethodGet, "/api/leads/"+leadID.String()+"/pending-replies", userID)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != "[]" {
		t.Errorf("empty list body = %q, want %q (must not be JSON null)", body, "[]")
	}
}

func TestHandler_ListByLead_NoUserReturns401(t *testing.T) {
	srv := newTestServer(&fakePendingReplyUseCase{}, &fakeLeadOwnership{})
	req := httptest.NewRequest(http.MethodGet, "/api/leads/"+uuid.New().String()+"/pending-replies", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHandler_ListByLead_InvalidLeadIDReturns400(t *testing.T) {
	srv := newTestServer(&fakePendingReplyUseCase{}, &fakeLeadOwnership{})
	req := authedRequest(t, http.MethodGet, "/api/leads/not-a-uuid/pending-replies", uuid.New())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandler_ListByLead_CrossTenantReturns404(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New() // not owned by userID
	uc := &fakePendingReplyUseCase{
		listFn: func(_ context.Context, _, _ uuid.UUID) ([]*PendingReply, error) {
			t.Fatal("usecase must NOT be called when lead is not owned")
			return nil, nil
		},
	}
	leads := &fakeLeadOwnership{owned: map[uuid.UUID]bool{}}

	srv := newTestServer(uc, leads)
	req := authedRequest(t, http.MethodGet, "/api/leads/"+leadID.String()+"/pending-replies", userID)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (uniform — no info leak)", rec.Code)
	}
}

// --- GET /api/pending-replies (operator queue) ---

func TestHandler_ListPendingByUser_HappyPath_EnrichedWithLeadSnippet(t *testing.T) {
	userID := uuid.New()
	leadID := uuid.New()
	pr, err := NewPendingReply(userID, leadID, ChannelTelegram, PendingReplyKindBookingLink, "queue body")
	if err != nil {
		t.Fatal(err)
	}
	chat := int64(987654)
	uc := &fakePendingReplyUseCase{
		listPendingByUserFn: func(_ context.Context, u uuid.UUID) ([]*PendingReplyWithLead, error) {
			if u != userID {
				t.Errorf("usecase received wrong user id: %v", u)
			}
			return []*PendingReplyWithLead{{
				Reply: pr,
				Lead: LeadSnippet{
					ContactName:    "Иван Петров",
					Company:        "ACME",
					Channel:        ChannelTelegram,
					TelegramChatID: &chat,
				},
			}}, nil
		},
	}
	srv := newTestServer(uc, &fakeLeadOwnership{})
	req := authedRequest(t, http.MethodGet, "/api/pending-replies?status=pending", userID)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var got []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("response len = %d, want 1", len(got))
	}
	row := got[0]
	if row["id"] != pr.ID.String() {
		t.Errorf("id = %v, want %v", row["id"], pr.ID)
	}
	if row["body"] != "queue body" {
		t.Errorf("body = %v, want queue body", row["body"])
	}
	if row["status"] != "pending" {
		t.Errorf("status = %v, want pending", row["status"])
	}
	lead, ok := row["lead"].(map[string]any)
	if !ok {
		t.Fatalf("lead field missing or wrong shape: %v", row["lead"])
	}
	if lead["contact_name"] != "Иван Петров" {
		t.Errorf("lead.contact_name = %v", lead["contact_name"])
	}
	if lead["company"] != "ACME" {
		t.Errorf("lead.company = %v", lead["company"])
	}
	if lead["channel"] != "telegram" {
		t.Errorf("lead.channel = %v", lead["channel"])
	}
	// JSON numbers decode to float64. Compare via that lens.
	if got, want := lead["telegram_chat_id"], float64(987654); got != want {
		t.Errorf("lead.telegram_chat_id = %v, want %v", got, want)
	}
	if _, present := lead["email_address"]; present {
		t.Errorf("lead.email_address must be omitempty when nil, got %v", lead["email_address"])
	}
	// Pending rows have no decision yet — the wire DTO must NOT carry
	// decided_at / decided_by / sent_at to avoid confusing the queue
	// UI ("why does this row look approved?"). omitempty contract.
	for _, field := range []string{"decided_at", "decided_by", "sent_at"} {
		if _, present := row[field]; present {
			t.Errorf("%s must be omitempty on a pending row, got %v", field, row[field])
		}
	}
}

func TestHandler_ListPendingByUser_DefaultsStatusToPendingWhenAbsent(t *testing.T) {
	userID := uuid.New()
	called := false
	uc := &fakePendingReplyUseCase{
		listPendingByUserFn: func(_ context.Context, _ uuid.UUID) ([]*PendingReplyWithLead, error) {
			called = true
			return nil, nil
		},
	}
	srv := newTestServer(uc, &fakeLeadOwnership{})
	// No ?status= param.
	req := authedRequest(t, http.MethodGet, "/api/pending-replies", userID)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (default to pending) — operators hit this URL with no filter for the queue", rec.Code)
	}
	if !called {
		t.Error("usecase must have been invoked when ?status is absent — handler defaults to pending")
	}
}

func TestHandler_ListPendingByUser_EmptyReturnsJSONArrayNotNull(t *testing.T) {
	uc := &fakePendingReplyUseCase{
		listPendingByUserFn: func(_ context.Context, _ uuid.UUID) ([]*PendingReplyWithLead, error) {
			return nil, nil
		},
	}
	srv := newTestServer(uc, &fakeLeadOwnership{})
	req := authedRequest(t, http.MethodGet, "/api/pending-replies?status=pending", uuid.New())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := strings.TrimSpace(rec.Body.String())
	if body != "[]" {
		t.Errorf("empty list body = %q, want %q (must not be JSON null)", body, "[]")
	}
}

func TestHandler_ListPendingByUser_NoUserReturns401(t *testing.T) {
	srv := newTestServer(&fakePendingReplyUseCase{}, &fakeLeadOwnership{})
	req := httptest.NewRequest(http.MethodGet, "/api/pending-replies?status=pending", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHandler_ListPendingByUser_UnsupportedStatusReturns400(t *testing.T) {
	uc := &fakePendingReplyUseCase{
		listPendingByUserFn: func(_ context.Context, _ uuid.UUID) ([]*PendingReplyWithLead, error) {
			t.Fatal("usecase must NOT be called when status filter is rejected")
			return nil, nil
		},
	}
	srv := newTestServer(uc, &fakeLeadOwnership{})
	req := authedRequest(t, http.MethodGet, "/api/pending-replies?status=approved", uuid.New())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (only pending is supported today)", rec.Code)
	}
}

func TestHandler_ListPendingByUser_UsecaseErrorReturns500(t *testing.T) {
	uc := &fakePendingReplyUseCase{
		listPendingByUserFn: func(_ context.Context, _ uuid.UUID) ([]*PendingReplyWithLead, error) {
			return nil, errors.New("db hiccup")
		},
	}
	srv := newTestServer(uc, &fakeLeadOwnership{})
	req := authedRequest(t, http.MethodGet, "/api/pending-replies?status=pending", uuid.New())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// --- POST /api/pending-replies/{id}/approve ---

func TestHandler_Approve_HappyPath_Returns204(t *testing.T) {
	userID := uuid.New()
	prID := uuid.New()
	uc := &fakePendingReplyUseCase{
		approveFn: func(_ context.Context, u, id uuid.UUID) error {
			if u != userID || id != prID {
				t.Errorf("approve got u=%v id=%v want u=%v id=%v", u, id, userID, prID)
			}
			return nil
		},
	}

	srv := newTestServer(uc, &fakeLeadOwnership{})
	req := authedRequest(t, http.MethodPost, "/api/pending-replies/"+prID.String()+"/approve", userID)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestHandler_Approve_NotFoundReturns404(t *testing.T) {
	uc := &fakePendingReplyUseCase{
		approveFn: func(_ context.Context, _, _ uuid.UUID) error {
			return ErrPendingReplyNotFound
		},
	}

	srv := newTestServer(uc, &fakeLeadOwnership{})
	req := authedRequest(t, http.MethodPost, "/api/pending-replies/"+uuid.New().String()+"/approve", uuid.New())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandler_Approve_AlreadyDecidedReturns409(t *testing.T) {
	uc := &fakePendingReplyUseCase{
		approveFn: func(_ context.Context, _, _ uuid.UUID) error {
			return ErrPendingReplyAlreadyDecided
		},
	}

	srv := newTestServer(uc, &fakeLeadOwnership{})
	req := authedRequest(t, http.MethodPost, "/api/pending-replies/"+uuid.New().String()+"/approve", uuid.New())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
}

func TestHandler_Approve_DispatcherErrorReturns500(t *testing.T) {
	uc := &fakePendingReplyUseCase{
		approveFn: func(_ context.Context, _, _ uuid.UUID) error {
			return errors.New("dispatch boom")
		},
	}

	srv := newTestServer(uc, &fakeLeadOwnership{})
	req := authedRequest(t, http.MethodPost, "/api/pending-replies/"+uuid.New().String()+"/approve", uuid.New())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestHandler_Approve_NoUserReturns401(t *testing.T) {
	srv := newTestServer(&fakePendingReplyUseCase{}, &fakeLeadOwnership{})
	req := httptest.NewRequest(http.MethodPost, "/api/pending-replies/"+uuid.New().String()+"/approve", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestHandler_Approve_InvalidIDReturns400(t *testing.T) {
	srv := newTestServer(&fakePendingReplyUseCase{}, &fakeLeadOwnership{})
	req := authedRequest(t, http.MethodPost, "/api/pending-replies/not-a-uuid/approve", uuid.New())
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// --- POST /api/pending-replies/{id}/reject ---

func TestHandler_Reject_HappyPath_Returns204(t *testing.T) {
	userID := uuid.New()
	prID := uuid.New()
	uc := &fakePendingReplyUseCase{
		rejectFn: func(_ context.Context, u, id uuid.UUID) error {
			if u != userID || id != prID {
				t.Errorf("reject got u=%v id=%v want u=%v id=%v", u, id, userID, prID)
			}
			return nil
		},
	}

	srv := newTestServer(uc, &fakeLeadOwnership{})
	req := authedRequest(t, http.MethodPost, "/api/pending-replies/"+prID.String()+"/reject", userID)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
}

func TestHandler_Reject_NotFoundAlreadyDecidedAuthAndInvalidID(t *testing.T) {
	cases := []struct {
		name       string
		ucErr      error
		userID     uuid.UUID
		pathID     string
		wantStatus int
	}{
		{name: "not found -> 404", ucErr: ErrPendingReplyNotFound, userID: uuid.New(), pathID: uuid.New().String(), wantStatus: http.StatusNotFound},
		{name: "already decided -> 409", ucErr: ErrPendingReplyAlreadyDecided, userID: uuid.New(), pathID: uuid.New().String(), wantStatus: http.StatusConflict},
		{name: "internal -> 500", ucErr: errors.New("boom"), userID: uuid.New(), pathID: uuid.New().String(), wantStatus: http.StatusInternalServerError},
		{name: "no user -> 401", ucErr: nil, userID: uuid.Nil, pathID: uuid.New().String(), wantStatus: http.StatusUnauthorized},
		{name: "invalid id -> 400", ucErr: nil, userID: uuid.New(), pathID: "not-a-uuid", wantStatus: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc := &fakePendingReplyUseCase{
				rejectFn: func(_ context.Context, _, _ uuid.UUID) error { return tc.ucErr },
			}
			srv := newTestServer(uc, &fakeLeadOwnership{})

			path := "/api/pending-replies/" + tc.pathID + "/reject"
			var req *http.Request
			if tc.userID == uuid.Nil {
				req = httptest.NewRequest(http.MethodPost, path, nil)
			} else {
				req = authedRequest(t, http.MethodPost, path, tc.userID)
			}
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

// --- UpdateBody (#48) PATCH /api/pending-replies/{id} ---

func TestHandler_UpdateBody_HappyPath_Returns200WithBody(t *testing.T) {
	userID := uuid.New()
	prID := uuid.New()

	uc := &fakePendingReplyUseCase{
		updateBodyFn: func(_ context.Context, _, id uuid.UUID, body string) (*PendingReply, error) {
			if id != prID {
				t.Fatalf("path id = %v, want %v", id, prID)
			}
			if body != "edited body" {
				t.Fatalf("body = %q, want 'edited body'", body)
			}
			return &PendingReply{
				ID:        prID,
				UserID:    userID,
				LeadID:    uuid.New(),
				Channel:   ChannelTelegram,
				Kind:      PendingReplyKindBookingLink,
				Body:      body,
				Status:    PendingReplyStatusPending,
				CreatedAt: time.Now().UTC(),
			}, nil
		},
	}
	srv := newTestServer(uc, &fakeLeadOwnership{})

	req := httptest.NewRequest(http.MethodPatch, "/api/pending-replies/"+prID.String(), strings.NewReader(`{"body":"edited body"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(httputil.WithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var got PendingReplyResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Body != "edited body" {
		t.Errorf("response body = %q, want 'edited body'", got.Body)
	}
	if got.Status != string(PendingReplyStatusPending) {
		t.Errorf("response status = %q, want pending", got.Status)
	}
}

func TestHandler_UpdateBody_ErrorMatrix(t *testing.T) {
	// Table-driven over the 400/404/409/401/400-id matrix. Mirrors the
	// existing Approve/Reject ErrorMatrix style.
	type tc struct {
		name       string
		ucErr      error
		body       string
		userID     uuid.UUID
		pathID     string
		wantStatus int
	}
	cases := []tc{
		{name: "not found -> 404", ucErr: ErrPendingReplyNotFound, body: `{"body":"x"}`, userID: uuid.New(), pathID: uuid.New().String(), wantStatus: http.StatusNotFound},
		{name: "already decided -> 409", ucErr: ErrPendingReplyAlreadyDecided, body: `{"body":"x"}`, userID: uuid.New(), pathID: uuid.New().String(), wantStatus: http.StatusConflict},
		{name: "empty body domain error -> 400", ucErr: ErrPendingReplyEmptyBody, body: `{"body":"   "}`, userID: uuid.New(), pathID: uuid.New().String(), wantStatus: http.StatusBadRequest},
		{name: "internal -> 500", ucErr: errors.New("boom"), body: `{"body":"x"}`, userID: uuid.New(), pathID: uuid.New().String(), wantStatus: http.StatusInternalServerError},
		{name: "no user -> 401", ucErr: nil, body: `{"body":"x"}`, userID: uuid.Nil, pathID: uuid.New().String(), wantStatus: http.StatusUnauthorized},
		{name: "invalid id -> 400", ucErr: nil, body: `{"body":"x"}`, userID: uuid.New(), pathID: "not-a-uuid", wantStatus: http.StatusBadRequest},
		{name: "malformed JSON -> 400", ucErr: nil, body: `not json`, userID: uuid.New(), pathID: uuid.New().String(), wantStatus: http.StatusBadRequest},
		{name: "missing body field -> 400", ucErr: nil, body: `{"other":"x"}`, userID: uuid.New(), pathID: uuid.New().String(), wantStatus: http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			uc := &fakePendingReplyUseCase{
				updateBodyFn: func(_ context.Context, _, _ uuid.UUID, _ string) (*PendingReply, error) {
					return nil, c.ucErr
				},
			}
			srv := newTestServer(uc, &fakeLeadOwnership{})

			path := "/api/pending-replies/" + c.pathID
			req := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(c.body))
			req.Header.Set("Content-Type", "application/json")
			if c.userID != uuid.Nil {
				req = req.WithContext(httputil.WithUserID(req.Context(), c.userID))
			}
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code != c.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, c.wantStatus, rec.Body.String())
			}
		})
	}
}
