package onec

import (
	"context"
	"errors"
	"fmt"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
)

// ConfigUseCase orchestrates the settings-UI management of a user's 1C
// integration (#110): reading the masked config, sparse updates with
// use-stored secret semantics, webhook-secret generation, connection testing
// and mapping CRUD. It owns no persistence or HTTP detail — those are ports.
type ConfigUseCase struct {
	store   ConfigStore
	mapping MappingConfigStore
	tester  ConnectionTester
	secrets SecretGenerator
}

// NewConfigUseCase wires the config usecase to its ports.
func NewConfigUseCase(store ConfigStore, mapping MappingConfigStore, tester ConnectionTester, secrets SecretGenerator) *ConfigUseCase {
	return &ConfigUseCase{store: store, mapping: mapping, tester: tester, secrets: secrets}
}

// ConfigView is the masked, read-side projection of a user's 1C config. Secrets
// are never returned in the clear — only their masked tails — so the settings
// API can be read without exposing credentials.
type ConfigView struct {
	BaseURL       string
	AuthType      string
	AuthSecret    string // masked
	WebhookSecret string // masked
	IsActive      bool
}

// ConfigUpdate is a sparse update: a nil pointer means "field absent, keep
// stored". AuthSecret additionally honours use-stored semantics (empty or a
// value equal to the masked form means keep the stored secret).
type ConfigUpdate struct {
	BaseURL    *string
	AuthType   *string
	AuthSecret *string
	IsActive   *bool
}

// ConfigTestOverride lets the "test connection" action try unsaved values. A
// nil field falls back to the stored config; a masked/empty AuthSecret falls
// back to the stored secret.
type ConfigTestOverride struct {
	BaseURL    *string
	AuthType   *string
	AuthSecret *string
}

// MappingRuleView / MappingRuleInput are the DTO shapes the handler maps to/from
// JSON, keeping the domain free of wire concerns.
type MappingRuleView struct {
	ExternalType string
	Kind         string
	EmailField   string
	NameField    string
	CompanyField string
}

type MappingRuleInput struct {
	ExternalType string
	Kind         string
	EmailField   string
	NameField    string
	CompanyField string
}

// GetConfig returns the user's masked config, or defaults when no row exists.
func (uc *ConfigUseCase) GetConfig(ctx context.Context, userID uuid.UUID) (*ConfigView, error) {
	cfg, err := uc.loadOrDefault(ctx, userID)
	if err != nil {
		return nil, err
	}
	return maskedView(cfg), nil
}

// UpdateConfig applies a sparse update over the stored config (read-merge-write
// in this layer — no dynamic SQL), generating a webhook secret on first
// activation, validating invariants via the domain factory before persisting,
// and returning the re-masked result.
func (uc *ConfigUseCase) UpdateConfig(ctx context.Context, userID uuid.UUID, in ConfigUpdate) (*ConfigView, error) {
	stored, err := uc.loadOrDefault(ctx, userID)
	if err != nil {
		return nil, err
	}

	baseURL := stored.BaseURL
	if in.BaseURL != nil {
		baseURL = *in.BaseURL
	}
	authType := string(stored.AuthType)
	if in.AuthType != nil {
		authType = *in.AuthType
	}
	authSecret := mergeSecret(in.AuthSecret, stored.AuthSecret)
	webhookSecret := stored.WebhookSecret
	isActive := stored.IsActive
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	// Activating with no webhook secret yet → generate one (256-bit random;
	// collision is cryptographically negligible, so no retry loop).
	if isActive && webhookSecret == "" {
		webhookSecret, err = uc.secrets.WebhookSecret()
		if err != nil {
			return nil, fmt.Errorf("onec: generate webhook secret: %w", err)
		}
	}

	merged, err := domain.NewCredentialsConfig(baseURL, domain.AuthType(authType), authSecret, webhookSecret, isActive)
	if err != nil {
		return nil, err
	}
	if err := uc.store.UpsertCredentialsConfig(ctx, userID, merged); err != nil {
		return nil, fmt.Errorf("onec: save config: %w", err)
	}
	return maskedView(merged), nil
}

// RegenerateWebhook mints a fresh webhook secret, persists it, and returns the
// full value ONCE so the operator can copy it into 1C. Subsequent GETs mask it.
func (uc *ConfigUseCase) RegenerateWebhook(ctx context.Context, userID uuid.UUID) (string, error) {
	stored, err := uc.loadOrDefault(ctx, userID)
	if err != nil {
		return "", err
	}
	secret, err := uc.secrets.WebhookSecret()
	if err != nil {
		return "", fmt.Errorf("onec: generate webhook secret: %w", err)
	}
	merged, err := domain.NewCredentialsConfig(stored.BaseURL, stored.AuthType, stored.AuthSecret, secret, stored.IsActive)
	if err != nil {
		return "", err
	}
	if err := uc.store.UpsertCredentialsConfig(ctx, userID, merged); err != nil {
		return "", fmt.Errorf("onec: save config: %w", err)
	}
	return secret, nil
}

// TestConnection probes the 1C endpoint, using stored values for any field the
// override leaves blank (and for a masked/empty secret). Returns
// domain.ErrEmptyBaseURL when there is nothing to connect to.
func (uc *ConfigUseCase) TestConnection(ctx context.Context, userID uuid.UUID, ov ConfigTestOverride) error {
	stored, err := uc.loadOrDefault(ctx, userID)
	if err != nil {
		return err
	}
	baseURL := stored.BaseURL
	if ov.BaseURL != nil {
		baseURL = *ov.BaseURL
	}
	authType := stored.AuthType
	if ov.AuthType != nil {
		authType = domain.AuthType(*ov.AuthType)
	}
	authSecret := mergeSecret(ov.AuthSecret, stored.AuthSecret)

	creds, err := domain.NewOutboundCredentials(baseURL, authType, authSecret)
	if err != nil {
		return err // ErrEmptyBaseURL / ErrInvalidAuthType
	}
	return uc.tester.TestConnection(ctx, creds)
}

// GetMapping returns the user's mapping rules (empty when none configured).
func (uc *ConfigUseCase) GetMapping(ctx context.Context, userID uuid.UUID) ([]MappingRuleView, error) {
	cfg, err := uc.mapping.GetMappingConfig(ctx, userID)
	if errors.Is(err, ErrMappingNotFound) {
		return []MappingRuleView{}, nil
	}
	if err != nil {
		return nil, err
	}
	views := make([]MappingRuleView, len(cfg.Rules))
	for i, r := range cfg.Rules {
		views[i] = MappingRuleView{
			ExternalType: r.ExternalType,
			Kind:         r.Kind.String(),
			EmailField:   r.EmailField,
			NameField:    r.NameField,
			CompanyField: r.CompanyField,
		}
	}
	return views, nil
}

// UpdateMapping validates the rules through the domain factory and saves them as
// the active config. Invalid input (empty rules, bad kind, dup external type)
// returns the domain error unchanged for the handler to map to a 400.
func (uc *ConfigUseCase) UpdateMapping(ctx context.Context, userID uuid.UUID, inputs []MappingRuleInput) error {
	rules := make([]domain.MappingRule, 0, len(inputs))
	for _, in := range inputs {
		kind, err := domain.ParseEventKind(in.Kind)
		if err != nil {
			return err
		}
		rules = append(rules, domain.MappingRule{
			ExternalType: in.ExternalType,
			Kind:         kind,
			EmailField:   in.EmailField,
			NameField:    in.NameField,
			CompanyField: in.CompanyField,
		})
	}
	cfg, err := domain.NewMappingConfig(userID, rules)
	if err != nil {
		return err
	}
	return uc.mapping.SaveMappingConfig(ctx, cfg, true)
}

// loadOrDefault returns the stored config or an empty, valid default (the shape
// a brand-new user sees) when no row exists.
func (uc *ConfigUseCase) loadOrDefault(ctx context.Context, userID uuid.UUID) (*domain.CredentialsConfig, error) {
	cfg, found, err := uc.store.GetCredentialsConfig(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("onec: load config: %w", err)
	}
	if !found {
		return domain.NewCredentialsConfig("", "", "", "", false)
	}
	return cfg, nil
}

// mergeSecret implements use-stored semantics: a nil, empty, or masked-equal
// input means "keep the stored secret"; anything else is a real new secret.
func mergeSecret(in *string, stored string) string {
	if in == nil || *in == "" || *in == maskSecret(stored) {
		return stored
	}
	return *in
}

// maskedView projects a config to its masked wire view.
func maskedView(c *domain.CredentialsConfig) *ConfigView {
	return &ConfigView{
		BaseURL:       c.BaseURL,
		AuthType:      string(c.AuthType),
		AuthSecret:    maskSecret(c.AuthSecret),
		WebhookSecret: maskSecret(c.WebhookSecret),
		IsActive:      c.IsActive,
	}
}

// maskSecret reveals only the last 4 characters of a secret (mirrors the
// settings package), so the read API never exposes a usable credential.
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return "..." + s
	}
	return "..." + s[len(s)-4:]
}
