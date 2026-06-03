package onec

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrMappingNotFound is returned when a user has no stored mapping config.
var ErrMappingNotFound = errors.New("onec: mapping config not found")

// mappingRuleDTO is the JSON serialization shape for a rule. Kept separate from
// domain.MappingRule so the domain stays free of storage/json concerns.
type mappingRuleDTO struct {
	ExternalType string `json:"external_type"`
	Kind         string `json:"kind"`
	EmailField   string `json:"email_field"`
	NameField    string `json:"name_field,omitempty"`
	CompanyField string `json:"company_field,omitempty"`
}

// SaveMappingConfig upserts a user's mapping rules (one config per user). The
// rules are stored as a jsonb array; isActive gates whether mapping is applied.
func (r *Repository) SaveMappingConfig(ctx context.Context, cfg *domain.MappingConfig, isActive bool) error {
	dtos := make([]mappingRuleDTO, len(cfg.Rules))
	for i, rule := range cfg.Rules {
		dtos[i] = mappingRuleDTO{
			ExternalType: rule.ExternalType,
			Kind:         rule.Kind.String(),
			EmailField:   rule.EmailField,
			NameField:    rule.NameField,
			CompanyField: rule.CompanyField,
		}
	}
	raw, err := json.Marshal(dtos)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO onec_mapping_configs (user_id, rules, is_active, updated_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id) DO UPDATE
			SET rules = EXCLUDED.rules, is_active = EXCLUDED.is_active, updated_at = NOW()`,
		cfg.UserID, raw, isActive)
	return err
}

// GetMappingConfig loads and reconstructs a user's mapping config regardless of
// active state (for the settings UI). Returns ErrMappingNotFound when absent.
func (r *Repository) GetMappingConfig(ctx context.Context, userID uuid.UUID) (*domain.MappingConfig, error) {
	return r.loadMappingConfig(ctx, userID, false)
}

// GetActiveMappingConfig loads a user's config only when is_active = TRUE, so an
// inactive config disables application. Returns ErrMappingNotFound otherwise.
// This is the MappingStore method the inbound flow uses.
func (r *Repository) GetActiveMappingConfig(ctx context.Context, userID uuid.UUID) (*domain.MappingConfig, error) {
	return r.loadMappingConfig(ctx, userID, true)
}

func (r *Repository) loadMappingConfig(ctx context.Context, userID uuid.UUID, activeOnly bool) (*domain.MappingConfig, error) {
	query := `SELECT rules FROM onec_mapping_configs WHERE user_id = $1`
	if activeOnly {
		query += ` AND is_active = TRUE`
	}
	var raw []byte
	err := r.pool.QueryRow(ctx, query, userID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMappingNotFound
	}
	if err != nil {
		return nil, err
	}

	var dtos []mappingRuleDTO
	if err := json.Unmarshal(raw, &dtos); err != nil {
		return nil, err
	}
	rules := make([]domain.MappingRule, len(dtos))
	for i, d := range dtos {
		kind, err := domain.ParseEventKind(d.Kind)
		if err != nil {
			return nil, err
		}
		rules[i] = domain.MappingRule{
			ExternalType: d.ExternalType,
			Kind:         kind,
			EmailField:   d.EmailField,
			NameField:    d.NameField,
			CompanyField: d.CompanyField,
		}
	}
	return domain.NewMappingConfig(userID, rules)
}
