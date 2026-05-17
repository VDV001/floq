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

	"github.com/daniil/floq/internal/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// --- fakes ---

type fakePendingReplyUseCase struct {
	mu sync.Mutex

	listFn    func(ctx context.Context, userID, leadID uuid.UUID) ([]*PendingReply, error)
	approveFn func(ctx context.Context, userID, id uuid.UUID) error
	rejectFn  func(ctx context.Context, userID, id uuid.UUID) error
}

func (f *fakePendingReplyUseCase) ListByLead(ctx context.Context, userID, leadID uuid.UUID) ([]*PendingReply, error) {
	if f.listFn != nil {
		return f.listFn(ctx, userID, leadID)
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
	RegisterPendingReplyRoutes(r, uc, leads)
	return r
}

func authedRequest(t *testing.T, method, path string, userID uuid.UUID) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req = req.WithContext(httputil.WithUserID(req.Context(), userID))
	return req
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
