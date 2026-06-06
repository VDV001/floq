package prospects_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daniil/floq/internal/prospects"
	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const unsubSecret = "unsub-test-secret"

type mockUnsubStore struct {
	prospect       *domain.Prospect
	getErr         error
	added          []*domain.Suppression
	addErr         error
	consentUpdated []domain.Consent
	consentErr     error
}

func (m *mockUnsubStore) GetProspect(_ context.Context, _ uuid.UUID) (*domain.Prospect, error) {
	return m.prospect, m.getErr
}

func (m *mockUnsubStore) AddSuppression(_ context.Context, s *domain.Suppression) error {
	m.added = append(m.added, s)
	return m.addErr
}

func (m *mockUnsubStore) UpdateConsent(_ context.Context, _ uuid.UUID, c domain.Consent) error {
	m.consentUpdated = append(m.consentUpdated, c)
	return m.consentErr
}

func newConsentedProspect(t *testing.T) *domain.Prospect {
	t.Helper()
	p, err := domain.NewProspect(uuid.New(), "Bob", "Acme", "CEO", "Bob@Example.com", "manual")
	require.NoError(t, err)
	return p
}

func TestUnsubscribe_ValidToken_SuppressesAndWithdraws(t *testing.T) {
	p := newConsentedProspect(t)
	store := &mockUnsubStore{prospect: p}
	svc := prospects.NewUnsubscribeService(store, unsubSecret)

	token := domain.SignUnsubscribeToken(p.ID, unsubSecret)
	require.NoError(t, svc.Unsubscribe(context.Background(), token))

	require.Len(t, store.added, 1, "one suppression should be added")
	assert.Equal(t, domain.SuppressionChannelEmail, store.added[0].Channel)
	assert.Equal(t, "bob@example.com", store.added[0].Address, "address normalized")
	assert.Equal(t, "unsubscribe", store.added[0].Reason)

	require.Len(t, store.consentUpdated, 1, "consent should be updated")
	assert.Equal(t, domain.ConsentStatusWithdrawn, store.consentUpdated[0].Status)
	assert.Equal(t, "unsubscribe", store.consentUpdated[0].Source)
}

func TestUnsubscribe_InvalidToken(t *testing.T) {
	store := &mockUnsubStore{prospect: newConsentedProspect(t)}
	svc := prospects.NewUnsubscribeService(store, unsubSecret)

	err := svc.Unsubscribe(context.Background(), "not-a-valid-token")
	require.ErrorIs(t, err, domain.ErrInvalidUnsubscribeToken)
	assert.Empty(t, store.added, "no suppression on invalid token")
	assert.Empty(t, store.consentUpdated, "no consent change on invalid token")
}

func TestUnsubscribe_WrongSecret(t *testing.T) {
	p := newConsentedProspect(t)
	store := &mockUnsubStore{prospect: p}
	svc := prospects.NewUnsubscribeService(store, unsubSecret)

	// Token minted with a different secret must not verify.
	token := domain.SignUnsubscribeToken(p.ID, "attacker-secret")
	require.ErrorIs(t, svc.Unsubscribe(context.Background(), token), domain.ErrInvalidUnsubscribeToken)
}

func TestUnsubscribe_NilProspect_SilentNoOp(t *testing.T) {
	// Valid token, prospect gone → privacy-preserving no-op, not an error.
	store := &mockUnsubStore{prospect: nil}
	svc := prospects.NewUnsubscribeService(store, unsubSecret)

	token := domain.SignUnsubscribeToken(uuid.New(), unsubSecret)
	require.NoError(t, svc.Unsubscribe(context.Background(), token))
	assert.Empty(t, store.added)
	assert.Empty(t, store.consentUpdated)
}

func TestHandleUnsubscribe_StatusCodes(t *testing.T) {
	p := newConsentedProspect(t)
	store := &mockUnsubStore{prospect: p}
	svc := prospects.NewUnsubscribeService(store, unsubSecret)

	r := chi.NewRouter()
	prospects.RegisterUnsubscribeRoutes(r, svc)

	// Valid GET → 200.
	token := domain.SignUnsubscribeToken(p.ID, unsubSecret)
	req := httptest.NewRequest(http.MethodGet, "/unsubscribe/"+token, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Garbage token → 400.
	req = httptest.NewRequest(http.MethodGet, "/unsubscribe/garbage", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// RFC 8058 one-click POST with a valid token → 200.
	req = httptest.NewRequest(http.MethodPost, "/unsubscribe/"+domain.SignUnsubscribeToken(p.ID, unsubSecret), nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}
