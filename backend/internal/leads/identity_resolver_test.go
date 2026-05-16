package leads

import (
	"context"
	"sync"
	"testing"

	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inMemoryIdentityRepo is the test double used by identity-resolver unit
// tests. It indexes identities by each canonical identifier so the
// lookups exercise byte-equality (matching the SQL repo's contract).
type inMemoryIdentityRepo struct {
	mu      sync.Mutex
	byEmail map[string]*domain.Identity
	byPhone map[string]*domain.Identity
	byTg    map[string]*domain.Identity
	saved   []*domain.Identity
}

func newInMemoryIdentityRepo() *inMemoryIdentityRepo {
	return &inMemoryIdentityRepo{
		byEmail: make(map[string]*domain.Identity),
		byPhone: make(map[string]*domain.Identity),
		byTg:    make(map[string]*domain.Identity),
	}
}

func (r *inMemoryIdentityRepo) FindByEmail(_ context.Context, _ uuid.UUID, email string) (*domain.Identity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byEmail[email], nil
}

func (r *inMemoryIdentityRepo) FindByPhone(_ context.Context, _ uuid.UUID, phone string) (*domain.Identity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byPhone[phone], nil
}

func (r *inMemoryIdentityRepo) FindByTelegramUsername(_ context.Context, _ uuid.UUID, tg string) (*domain.Identity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.byTg[tg], nil
}

func (r *inMemoryIdentityRepo) Save(_ context.Context, id *domain.Identity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.saved = append(r.saved, id)
	if id.Email != "" {
		r.byEmail[id.Email] = id
	}
	if id.Phone != "" {
		r.byPhone[id.Phone] = id
	}
	if id.TelegramUsername != "" {
		r.byTg[id.TelegramUsername] = id
	}
	return nil
}

// preset stores an identity in all relevant indexes without going through
// Save (so the saved counter only tracks Resolver-driven inserts).
func (r *inMemoryIdentityRepo) preset(id *domain.Identity) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id.Email != "" {
		r.byEmail[id.Email] = id
	}
	if id.Phone != "" {
		r.byPhone[id.Phone] = id
	}
	if id.TelegramUsername != "" {
		r.byTg[id.TelegramUsername] = id
	}
}

func TestIdentityResolver_Resolve_CreatesNewWhenNoMatch(t *testing.T) {
	repo := newInMemoryIdentityRepo()
	resolver := NewIdentityResolver(repo)

	userID := uuid.New()
	id, err := resolver.Resolve(context.Background(), userID, "ALICE@Acme.COM", "+7 999 123-45-67", "@Alice")
	require.NoError(t, err)
	require.NotNil(t, id)

	assert.Equal(t, "alice@acme.com", id.Email)
	assert.Equal(t, "+79991234567", id.Phone)
	assert.Equal(t, "alice", id.TelegramUsername)
	assert.Equal(t, userID, id.UserID)

	require.Len(t, repo.saved, 1, "a new identity must be persisted exactly once")
	assert.Equal(t, id.ID, repo.saved[0].ID)
}

func TestIdentityResolver_Resolve_ReturnsExistingMatch(t *testing.T) {
	cases := []struct {
		name        string
		preset      func(t *testing.T) *domain.Identity
		inputEmail  string
		inputPhone  string
		inputTg     string
		expectField string // "Email", "Phone", or "TelegramUsername"
	}{
		{
			name: "match by email",
			preset: func(t *testing.T) *domain.Identity {
				id, err := domain.NewIdentity(uuid.New(), "alice@acme.com", "", "")
				require.NoError(t, err)
				return id
			},
			inputEmail:  "ALICE@Acme.COM",
			expectField: "Email",
		},
		{
			name: "match by phone",
			preset: func(t *testing.T) *domain.Identity {
				id, err := domain.NewIdentity(uuid.New(), "", "+79991234567", "")
				require.NoError(t, err)
				return id
			},
			inputPhone:  "+7 (999) 123-45-67",
			expectField: "Phone",
		},
		{
			name: "match by telegram username",
			preset: func(t *testing.T) *domain.Identity {
				id, err := domain.NewIdentity(uuid.New(), "", "", "alice_bot")
				require.NoError(t, err)
				return id
			},
			inputTg:     "@ALICE_BOT",
			expectField: "TelegramUsername",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := newInMemoryIdentityRepo()
			existing := c.preset(t)
			repo.preset(existing)
			resolver := NewIdentityResolver(repo)

			got, err := resolver.Resolve(context.Background(), uuid.New(), c.inputEmail, c.inputPhone, c.inputTg)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, existing.ID, got.ID, "must return the pre-existing identity, not a fresh one")
			assert.Empty(t, repo.saved, "match must not trigger Save")
		})
	}
}

func TestIdentityResolver_Resolve_RejectsEmptyInputs(t *testing.T) {
	repo := newInMemoryIdentityRepo()
	resolver := NewIdentityResolver(repo)

	_, err := resolver.Resolve(context.Background(), uuid.New(), "", "", "")
	require.ErrorIs(t, err, domain.ErrIdentityNoIdentifiers)
	assert.Empty(t, repo.saved)
}

// TestIdentityResolver_Resolve_LookupPriority documents the deterministic
// lookup order: email > phone > tg. When two distinct identities exist
// (one keyed by email, another by tg) and the caller supplies both, the
// resolver returns the email match.
func TestIdentityResolver_Resolve_LookupPriority(t *testing.T) {
	repo := newInMemoryIdentityRepo()

	byEmail, err := domain.NewIdentity(uuid.New(), "alice@acme.com", "", "")
	require.NoError(t, err)
	byTg, err := domain.NewIdentity(uuid.New(), "", "", "alice_bot")
	require.NoError(t, err)
	repo.preset(byEmail)
	repo.preset(byTg)
	require.NotEqual(t, byEmail.ID, byTg.ID)

	resolver := NewIdentityResolver(repo)
	got, err := resolver.Resolve(context.Background(), uuid.New(), "alice@acme.com", "", "alice_bot")
	require.NoError(t, err)
	assert.Equal(t, byEmail.ID, got.ID, "email match wins over tg match")
	assert.Empty(t, repo.saved)
}
