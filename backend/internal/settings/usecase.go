package settings

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/daniil/floq/internal/settings/domain"
	"github.com/google/uuid"
)

// SettingsInput is used for JSON deserialization of update requests.
type SettingsInput struct {
	TelegramBotToken  string `json:"telegram_bot_token"`
	TelegramBotActive bool   `json:"telegram_bot_active"`
	IMAPHost          string `json:"imap_host"`
	IMAPPort          string `json:"imap_port"`
	IMAPUser          string `json:"imap_user"`
	IMAPPassword      string `json:"imap_password"`
	IMAPVerified      bool   `json:"imap_verified"`
	ResendAPIKey      string `json:"resend_api_key"`
	ResendVerified    bool   `json:"resend_verified"`
	SMTPHost          string `json:"smtp_host"`
	SMTPPort          string `json:"smtp_port"`
	SMTPUser          string `json:"smtp_user"`
	SMTPPassword      string `json:"smtp_password"`
	SMTPVerified      bool   `json:"smtp_verified"`
	AIProvider          string `json:"ai_provider"`
	AIModel             string `json:"ai_model"`
	AIAPIKey            string `json:"ai_api_key"`
	AIStyleCheckEnabled bool   `json:"ai_style_check_enabled"`
	AIVerified          bool   `json:"ai_verified"`
	NotifyTelegram    bool   `json:"notify_telegram"`
	NotifyEmailDigest bool   `json:"notify_email_digest"`
	AutoQualify       bool   `json:"auto_qualify"`
	AutoDraft         bool   `json:"auto_draft"`
	AutoSend          bool   `json:"auto_send"`
	AutoSendDelayMin  int    `json:"auto_send_delay_min"`
	AutoFollowup      bool   `json:"auto_followup"`
	AutoFollowupDays  int    `json:"auto_followup_days"`
	AutoProspectToLead bool  `json:"auto_prospect_to_lead"`
	AutoVerifyImport  bool   `json:"auto_verify_import"`

	// AggregatedInboxView toggles the unified-identity lead detail
	// timeline (#27). Server default is TRUE; the field is part of
	// the SettingsInput so the UI can let users opt out per-account.
	AggregatedInboxView bool `json:"aggregated_inbox_view"`
}

type UseCase struct {
	repo      domain.Repository
	tgValidator domain.TelegramTokenValidator
}

func NewUseCase(repo domain.Repository, tgValidator domain.TelegramTokenValidator) *UseCase {
	return &UseCase{repo: repo, tgValidator: tgValidator}
}

func (uc *UseCase) GetSettings(ctx context.Context, userID uuid.UUID) (*domain.Settings, error) {
	// Returns raw domain values. Masking is a presentation concern applied
	// in the DTO mapping (handler layer); internal callers need the unmasked
	// secrets.
	return uc.repo.GetSettings(ctx, userID)
}

// setVerified applies the #222 rule for one channel's *_verified column:
// an explicit verifiedKey in the request wins (the client sends it true
// after a passing connection test); otherwise, if any of the channel's
// credential keys changed, the flag is cleared so a stale «Готово» cannot
// survive a credential edit. Absent both, the column is left untouched.
func setVerified(raw map[string]json.RawMessage, fields map[string]any, verifiedVal bool, verifiedKey string, credKeys ...string) {
	if _, ok := raw[verifiedKey]; ok {
		fields[verifiedKey] = verifiedVal
		return
	}
	for _, k := range credKeys {
		if _, ok := raw[k]; ok {
			fields[verifiedKey] = false
			return
		}
	}
}

func (uc *UseCase) UpdateSettings(ctx context.Context, userID uuid.UUID, raw map[string]json.RawMessage, input SettingsInput) (*domain.Settings, error) {
	// If telegram_bot_token is being set, validate it.
	if _, ok := raw["telegram_bot_token"]; ok && input.TelegramBotToken != "" {
		if err := uc.tgValidator.Validate(input.TelegramBotToken); err != nil {
			return nil, fmt.Errorf("invalid telegram bot token: %w", err)
		}
	}

	// Build the fields map based on which fields are present in the request.
	fields := make(map[string]any)

	if _, ok := raw["telegram_bot_token"]; ok {
		fields["telegram_bot_token"] = input.TelegramBotToken
		if input.TelegramBotToken != "" {
			fields["telegram_bot_active"] = true
		} else {
			fields["telegram_bot_active"] = false
		}
	} else if _, ok := raw["telegram_bot_active"]; ok {
		fields["telegram_bot_active"] = input.TelegramBotActive
	}
	if _, ok := raw["imap_host"]; ok {
		fields["imap_host"] = input.IMAPHost
	}
	if _, ok := raw["imap_port"]; ok {
		fields["imap_port"] = input.IMAPPort
	}
	if _, ok := raw["imap_user"]; ok {
		fields["imap_user"] = input.IMAPUser
	}
	if _, ok := raw["imap_password"]; ok {
		fields["imap_password"] = input.IMAPPassword
	}
	if _, ok := raw["resend_api_key"]; ok {
		fields["resend_api_key"] = input.ResendAPIKey
	}
	if _, ok := raw["smtp_host"]; ok {
		fields["smtp_host"] = input.SMTPHost
	}
	if _, ok := raw["smtp_port"]; ok {
		fields["smtp_port"] = input.SMTPPort
	}
	if _, ok := raw["smtp_user"]; ok {
		fields["smtp_user"] = input.SMTPUser
	}
	if _, ok := raw["smtp_password"]; ok {
		fields["smtp_password"] = input.SMTPPassword
	}
	if _, ok := raw["ai_provider"]; ok {
		fields["ai_provider"] = input.AIProvider
	}
	if _, ok := raw["ai_model"]; ok {
		fields["ai_model"] = input.AIModel
	}
	if _, ok := raw["ai_api_key"]; ok {
		fields["ai_api_key"] = input.AIAPIKey
	}
	if _, ok := raw["ai_style_check_enabled"]; ok {
		fields["ai_style_check_enabled"] = input.AIStyleCheckEnabled
	}

	// Honest onboarding «Готово» (#222): a channel's *_verified flag mirrors
	// a passed connection test. The client sends {channel}_verified:true
	// right after a test succeeds; any change to the channel's credentials
	// without an accompanying verified field invalidates it (must re-test).
	setVerified(raw, fields, input.AIVerified, "ai_verified", "ai_provider", "ai_model", "ai_api_key")
	setVerified(raw, fields, input.SMTPVerified, "smtp_verified", "smtp_host", "smtp_port", "smtp_user", "smtp_password")
	setVerified(raw, fields, input.IMAPVerified, "imap_verified", "imap_host", "imap_port", "imap_user", "imap_password")
	if _, ok := raw["notify_telegram"]; ok {
		fields["notify_telegram"] = input.NotifyTelegram
	}
	if _, ok := raw["notify_email_digest"]; ok {
		fields["notify_email_digest"] = input.NotifyEmailDigest
	}
	if _, ok := raw["auto_qualify"]; ok {
		fields["auto_qualify"] = input.AutoQualify
	}
	if _, ok := raw["auto_draft"]; ok {
		fields["auto_draft"] = input.AutoDraft
	}
	if _, ok := raw["auto_send"]; ok {
		fields["auto_send"] = input.AutoSend
	}
	if _, ok := raw["auto_send_delay_min"]; ok {
		fields["auto_send_delay_min"] = input.AutoSendDelayMin
	}
	if _, ok := raw["auto_followup"]; ok {
		fields["auto_followup"] = input.AutoFollowup
	}
	if _, ok := raw["auto_followup_days"]; ok {
		fields["auto_followup_days"] = input.AutoFollowupDays
	}
	if _, ok := raw["auto_prospect_to_lead"]; ok {
		fields["auto_prospect_to_lead"] = input.AutoProspectToLead
	}
	if _, ok := raw["aggregated_inbox_view"]; ok {
		fields["aggregated_inbox_view"] = input.AggregatedInboxView
	}
	if _, ok := raw["auto_verify_import"]; ok {
		fields["auto_verify_import"] = input.AutoVerifyImport
	}

	if len(fields) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	if err := uc.repo.UpdateSettings(ctx, userID, fields); err != nil {
		return nil, err
	}

	return uc.GetSettings(ctx, userID)
}

// GetStoredIMAPPassword returns the stored IMAP password from the database.
func (uc *UseCase) GetStoredIMAPPassword(ctx context.Context, userID uuid.UUID) (string, error) {
	return uc.repo.GetStoredIMAPPassword(ctx, userID)
}

// GetStoredResendKey returns the stored Resend API key from the database.
func (uc *UseCase) GetStoredResendKey(ctx context.Context, userID uuid.UUID) (string, error) {
	s, err := uc.repo.GetSettings(ctx, userID)
	if err != nil {
		return "", err
	}
	return s.ResendAPIKey, nil
}

// StoredAISettings holds provider/model/key read from DB.
type StoredAISettings struct {
	Provider string
	Model    string
	APIKey   string
}

// GetStoredAISettings returns the stored AI settings from the database.
func (uc *UseCase) GetStoredAISettings(ctx context.Context, userID uuid.UUID) (*StoredAISettings, error) {
	s, err := uc.repo.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &StoredAISettings{
		Provider: s.AIProvider,
		Model:    s.AIModel,
		APIKey:   s.AIAPIKey,
	}, nil
}
