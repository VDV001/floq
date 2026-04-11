package settings

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/daniil/floq/internal/settings/domain"
	"github.com/google/uuid"
)

// settingsInput is used for JSON deserialization of update requests.
type settingsInput struct {
	TelegramBotToken  string `json:"telegram_bot_token"`
	TelegramBotActive bool   `json:"telegram_bot_active"`
	IMAPHost          string `json:"imap_host"`
	IMAPPort          string `json:"imap_port"`
	IMAPUser          string `json:"imap_user"`
	IMAPPassword      string `json:"imap_password"`
	ResendAPIKey      string `json:"resend_api_key"`
	SMTPHost          string `json:"smtp_host"`
	SMTPPort          string `json:"smtp_port"`
	SMTPUser          string `json:"smtp_user"`
	SMTPPassword      string `json:"smtp_password"`
	AIProvider        string `json:"ai_provider"`
	AIModel           string `json:"ai_model"`
	AIAPIKey          string `json:"ai_api_key"`
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
}

type UseCase struct {
	repo      domain.Repository
	tgValidator domain.TelegramTokenValidator
}

func NewUseCase(repo domain.Repository, tgValidator domain.TelegramTokenValidator) *UseCase {
	return &UseCase{repo: repo, tgValidator: tgValidator}
}

func (uc *UseCase) GetSettings(ctx context.Context, userID uuid.UUID) (*domain.Settings, error) {
	s, err := uc.repo.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Mask sensitive fields before returning.
	s.TelegramBotToken = maskSecret(s.TelegramBotToken)
	s.IMAPPassword = maskSecret(s.IMAPPassword)
	s.ResendAPIKey = maskSecret(s.ResendAPIKey)
	s.SMTPPassword = maskSecret(s.SMTPPassword)
	s.AIAPIKey = maskSecret(s.AIAPIKey)

	return s, nil
}

func (uc *UseCase) UpdateSettings(ctx context.Context, userID uuid.UUID, rawBody []byte) (*domain.Settings, error) {
	// Decode into a map so we know which fields were actually sent.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON")
	}

	// Also decode into the struct for typed access.
	var input settingsInput
	if err := json.Unmarshal(rawBody, &input); err != nil {
		return nil, fmt.Errorf("invalid JSON")
	}

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
