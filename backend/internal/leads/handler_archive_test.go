package leads

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_ArchiveLead_Success(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusQualified}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, reqWithUser("POST", "/api/leads/"+leadID.String()+"/archive", nil, userID))

	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, repo.leads[leadID].ArchivedAt, "lead should be archived")
	assert.Equal(t, domain.StatusQualified, repo.leads[leadID].Status)
}

func TestHandler_ArchiveLead_AlreadyArchived(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	lead := &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusNew}
	require.NoError(t, lead.Archive())
	repo.leads[leadID] = lead

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, reqWithUser("POST", "/api/leads/"+leadID.String()+"/archive", nil, userID))

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandler_ArchiveLead_Foreign404(t *testing.T) {
	repo := newMockRepo()
	owner := uuid.New()
	attacker := uuid.New()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: owner, Status: domain.StatusNew}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, reqWithUser("POST", "/api/leads/"+leadID.String()+"/archive", nil, attacker))

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Nil(t, repo.leads[leadID].ArchivedAt, "foreign archive must not mutate the lead")
}

func TestHandler_ArchiveLead_NoAuth401(t *testing.T) {
	repo := newMockRepo()
	leadID := uuid.New()
	repo.leads[leadID] = &domain.Lead{ID: leadID, UserID: uuid.New(), Status: domain.StatusNew}

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/leads/"+leadID.String()+"/archive", nil))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_UnarchiveLead_Success(t *testing.T) {
	repo := newMockRepo()
	userID := uuid.New()
	leadID := uuid.New()
	lead := &domain.Lead{ID: leadID, UserID: userID, Status: domain.StatusNew}
	require.NoError(t, lead.Archive())
	repo.leads[leadID] = lead

	r := newTestRouter(NewUseCase(repo, &mockAI{}, nil))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, reqWithUser("POST", "/api/leads/"+leadID.String()+"/unarchive", nil, userID))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Nil(t, repo.leads[leadID].ArchivedAt, "lead should be unarchived")
}
