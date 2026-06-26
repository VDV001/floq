package onec_test

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/integrations/onec"
	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type fakeConfigStore struct {
	cfg         *domain.CredentialsConfig
	found       bool
	getErr      error
	upserted    *domain.CredentialsConfig
	upsertErr   error
	upsertCalls int
}

func (f *fakeConfigStore) GetCredentialsConfig(_ context.Context, _ uuid.UUID) (*domain.CredentialsConfig, bool, error) {
	return f.cfg, f.found, f.getErr
}

func (f *fakeConfigStore) UpsertCredentialsConfig(_ context.Context, _ uuid.UUID, cfg *domain.CredentialsConfig) error {
	f.upsertCalls++
	f.upserted = cfg
	return f.upsertErr
}

type fakeMappingStore struct {
	cfg         *domain.MappingConfig
	getErr      error
	saved       *domain.MappingConfig
	savedActive bool
	saveErr     error
	saveCalls   int
}

func (f *fakeMappingStore) GetMappingConfig(_ context.Context, _ uuid.UUID) (*domain.MappingConfig, error) {
	return f.cfg, f.getErr
}

func (f *fakeMappingStore) SaveMappingConfig(_ context.Context, cfg *domain.MappingConfig, isActive bool) error {
	f.saveCalls++
	f.saved = cfg
	f.savedActive = isActive
	return f.saveErr
}

type fakeTester struct {
	gotCreds *domain.OutboundCredentials
	err      error
	calls    int
}

func (f *fakeTester) TestConnection(_ context.Context, creds *domain.OutboundCredentials) error {
	f.calls++
	f.gotCreds = creds
	return f.err
}

type fakeSecretGen struct {
	secret string
	calls  int
}

func (f *fakeSecretGen) WebhookSecret() (string, error) {
	f.calls++
	return f.secret, nil
}

func ptr[T any](v T) *T { return &v }

func storedCfg(t *testing.T, baseURL string, at domain.AuthType, secret, webhook string, active bool) *domain.CredentialsConfig {
	t.Helper()
	c, err := domain.NewCredentialsConfig(baseURL, at, secret, webhook, active)
	require.NoError(t, err)
	return c
}

func newUC(store onec.ConfigStore, mapping onec.MappingConfigStore, tester onec.ConnectionTester, gen onec.SecretGenerator) *onec.ConfigUseCase {
	return onec.NewConfigUseCase(store, mapping, tester, gen)
}

const validWebhook = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 64 hex

// --- GetConfig ---

func TestConfigUseCase_GetConfig_MasksSecrets(t *testing.T) {
	store := &fakeConfigStore{found: true, cfg: storedCfg(t, "https://1c.example.com", domain.AuthTypeToken, "abcdef123456", validWebhook, true)}
	uc := newUC(store, &fakeMappingStore{}, &fakeTester{}, &fakeSecretGen{})

	v, err := uc.GetConfig(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "https://1c.example.com", v.BaseURL)
	assert.Equal(t, "token", v.AuthType)
	assert.Equal(t, "...3456", v.AuthSecret, "secret masked to last 4")
	assert.Equal(t, "...aaaa", v.WebhookSecret, "webhook masked")
	assert.True(t, v.IsActive)
	assert.NotContains(t, v.AuthSecret, "abcdef")
}

func TestConfigUseCase_GetConfig_MasksShortSecretFully(t *testing.T) {
	// A short secret (≤4 chars) must not be revealed by the mask — return a
	// fixed placeholder instead of leaking the whole value.
	store := &fakeConfigStore{found: true, cfg: storedCfg(t, "https://1c.example.com", domain.AuthTypeBasic, "abc", "", false)}
	uc := newUC(store, &fakeMappingStore{}, &fakeTester{}, &fakeSecretGen{})

	v, err := uc.GetConfig(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.NotContains(t, v.AuthSecret, "abc", "short secret must not leak")
	assert.NotEqual(t, "", v.AuthSecret, "but still indicates a secret is set")
}

func TestConfigUseCase_GetConfig_DefaultsWhenNoRow(t *testing.T) {
	store := &fakeConfigStore{found: false}
	uc := newUC(store, &fakeMappingStore{}, &fakeTester{}, &fakeSecretGen{})

	v, err := uc.GetConfig(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "", v.BaseURL)
	assert.Equal(t, "basic", v.AuthType)
	assert.Equal(t, "", v.AuthSecret)
	assert.Equal(t, "", v.WebhookSecret)
	assert.False(t, v.IsActive)
}

// --- UpdateConfig: use-stored secret ---

func TestConfigUseCase_UpdateConfig_UseStoredSecret(t *testing.T) {
	cases := []struct {
		name       string
		authSecret *string
		wantSecret string
	}{
		{"absent keeps stored", nil, "abcdef123456"},
		{"empty keeps stored", ptr(""), "abcdef123456"},
		{"masked value keeps stored", ptr("...3456"), "abcdef123456"},
		{"new value replaces", ptr("brandnewsecret"), "brandnewsecret"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeConfigStore{found: true, cfg: storedCfg(t, "https://1c.example.com", domain.AuthTypeBasic, "abcdef123456", validWebhook, false)}
			uc := newUC(store, &fakeMappingStore{}, &fakeTester{}, &fakeSecretGen{})

			_, err := uc.UpdateConfig(context.Background(), uuid.New(), onec.ConfigUpdate{AuthSecret: tc.authSecret})
			require.NoError(t, err)
			require.NotNil(t, store.upserted)
			assert.Equal(t, tc.wantSecret, store.upserted.AuthSecret)
		})
	}
}

func TestConfigUseCase_UpdateConfig_AutoGenWebhookOnActivate(t *testing.T) {
	store := &fakeConfigStore{found: true, cfg: storedCfg(t, "https://1c.example.com", domain.AuthTypeBasic, "s", "", false)}
	gen := &fakeSecretGen{secret: validWebhook}
	uc := newUC(store, &fakeMappingStore{}, &fakeTester{}, gen)

	_, err := uc.UpdateConfig(context.Background(), uuid.New(), onec.ConfigUpdate{IsActive: ptr(true)})
	require.NoError(t, err)
	assert.Equal(t, 1, gen.calls, "webhook generated when activating with no secret")
	assert.Equal(t, validWebhook, store.upserted.WebhookSecret)
	assert.True(t, store.upserted.IsActive)
}

func TestConfigUseCase_UpdateConfig_ActiveRequiresBaseURL(t *testing.T) {
	store := &fakeConfigStore{found: false}
	uc := newUC(store, &fakeMappingStore{}, &fakeTester{}, &fakeSecretGen{secret: validWebhook})

	_, err := uc.UpdateConfig(context.Background(), uuid.New(), onec.ConfigUpdate{IsActive: ptr(true)})
	assert.True(t, errors.Is(err, domain.ErrActiveRequiresBaseURL), "got %v", err)
	assert.Equal(t, 0, store.upsertCalls, "invalid config must not be persisted")
}

func TestConfigUseCase_UpdateConfig_ToggleOffKeepsSecrets(t *testing.T) {
	store := &fakeConfigStore{found: true, cfg: storedCfg(t, "https://1c.example.com", domain.AuthTypeToken, "mysecret", validWebhook, true)}
	uc := newUC(store, &fakeMappingStore{}, &fakeTester{}, &fakeSecretGen{})

	_, err := uc.UpdateConfig(context.Background(), uuid.New(), onec.ConfigUpdate{IsActive: ptr(false)})
	require.NoError(t, err)
	assert.False(t, store.upserted.IsActive)
	assert.Equal(t, "mysecret", store.upserted.AuthSecret, "auth secret retained")
	assert.Equal(t, validWebhook, store.upserted.WebhookSecret, "webhook secret retained on disable")
}

func TestConfigUseCase_UpdateConfig_AuthTypeSwitchKeepsSecret(t *testing.T) {
	store := &fakeConfigStore{found: true, cfg: storedCfg(t, "https://1c.example.com", domain.AuthTypeBasic, "mysecret", validWebhook, false)}
	uc := newUC(store, &fakeMappingStore{}, &fakeTester{}, &fakeSecretGen{})

	_, err := uc.UpdateConfig(context.Background(), uuid.New(), onec.ConfigUpdate{AuthType: ptr("token")})
	require.NoError(t, err)
	assert.Equal(t, domain.AuthTypeToken, store.upserted.AuthType)
	assert.Equal(t, "mysecret", store.upserted.AuthSecret)
}

// --- RegenerateWebhook ---

func TestConfigUseCase_RegenerateWebhook_ReturnsFullAndPersists(t *testing.T) {
	store := &fakeConfigStore{found: true, cfg: storedCfg(t, "https://1c.example.com", domain.AuthTypeBasic, "s", "", false)}
	gen := &fakeSecretGen{secret: validWebhook}
	uc := newUC(store, &fakeMappingStore{}, &fakeTester{}, gen)

	full, err := uc.RegenerateWebhook(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, validWebhook, full, "returns the full secret once")
	assert.Equal(t, validWebhook, store.upserted.WebhookSecret, "persisted")
}

// --- TestConnection ---

func TestConfigUseCase_TestConnection_UsesStoredWhenNoOverride(t *testing.T) {
	store := &fakeConfigStore{found: true, cfg: storedCfg(t, "https://1c.example.com", domain.AuthTypeToken, "storedsecret", "", true)}
	tester := &fakeTester{}
	uc := newUC(store, &fakeMappingStore{}, tester, &fakeSecretGen{})

	err := uc.TestConnection(context.Background(), uuid.New(), onec.ConfigTestOverride{})
	require.NoError(t, err)
	require.Equal(t, 1, tester.calls)
	assert.Equal(t, "https://1c.example.com", tester.gotCreds.BaseURL)
	assert.Equal(t, domain.AuthTypeToken, tester.gotCreds.AuthType)
	assert.Equal(t, "storedsecret", tester.gotCreds.AuthSecret)
}

func TestConfigUseCase_TestConnection_EmptyBaseURLErrors(t *testing.T) {
	store := &fakeConfigStore{found: false}
	tester := &fakeTester{}
	uc := newUC(store, &fakeMappingStore{}, tester, &fakeSecretGen{})

	err := uc.TestConnection(context.Background(), uuid.New(), onec.ConfigTestOverride{})
	assert.True(t, errors.Is(err, domain.ErrEmptyBaseURL), "got %v", err)
	assert.Equal(t, 0, tester.calls, "no client call without a base url")
}

func TestConfigUseCase_TestConnection_OverrideUsesStoredSecretWhenMasked(t *testing.T) {
	store := &fakeConfigStore{found: true, cfg: storedCfg(t, "https://old.example.com", domain.AuthTypeBasic, "storedsecret", "", false)}
	tester := &fakeTester{}
	uc := newUC(store, &fakeMappingStore{}, tester, &fakeSecretGen{})

	err := uc.TestConnection(context.Background(), uuid.New(), onec.ConfigTestOverride{
		BaseURL:    ptr("https://new.example.com"),
		AuthSecret: ptr("...cret"), // masked → use stored
	})
	require.NoError(t, err)
	assert.Equal(t, "https://new.example.com", tester.gotCreds.BaseURL)
	assert.Equal(t, "storedsecret", tester.gotCreds.AuthSecret)
}

// --- Mapping ---

func TestConfigUseCase_GetMapping_EmptyWhenNotFound(t *testing.T) {
	mapping := &fakeMappingStore{getErr: onec.ErrMappingNotFound}
	uc := newUC(&fakeConfigStore{}, mapping, &fakeTester{}, &fakeSecretGen{})

	rules, err := uc.GetMapping(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, rules)
}

func TestConfigUseCase_UpdateMapping(t *testing.T) {
	t.Run("valid rules saved active", func(t *testing.T) {
		mapping := &fakeMappingStore{}
		uc := newUC(&fakeConfigStore{}, mapping, &fakeTester{}, &fakeSecretGen{})
		err := uc.UpdateMapping(context.Background(), uuid.New(), []onec.MappingRuleInput{
			{ExternalType: "Документ.ОплатаПокупателя", Kind: "payment", EmailField: "email"},
		})
		require.NoError(t, err)
		require.Equal(t, 1, mapping.saveCalls)
		assert.True(t, mapping.savedActive)
		require.Len(t, mapping.saved.Rules, 1)
		assert.Equal(t, domain.EventKindPayment, mapping.saved.Rules[0].Kind)
	})

	t.Run("invalid kind rejected, nothing saved", func(t *testing.T) {
		mapping := &fakeMappingStore{}
		uc := newUC(&fakeConfigStore{}, mapping, &fakeTester{}, &fakeSecretGen{})
		err := uc.UpdateMapping(context.Background(), uuid.New(), []onec.MappingRuleInput{
			{ExternalType: "X", Kind: "not_a_kind", EmailField: "email"},
		})
		assert.Error(t, err)
		assert.Equal(t, 0, mapping.saveCalls)
	})

	t.Run("empty rules rejected", func(t *testing.T) {
		mapping := &fakeMappingStore{}
		uc := newUC(&fakeConfigStore{}, mapping, &fakeTester{}, &fakeSecretGen{})
		err := uc.UpdateMapping(context.Background(), uuid.New(), nil)
		assert.True(t, errors.Is(err, domain.ErrNoRules), "got %v", err)
		assert.Equal(t, 0, mapping.saveCalls)
	})
}
