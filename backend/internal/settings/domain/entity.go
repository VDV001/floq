package domain

// Settings represents the full user settings entity.
type Settings struct {
	// Profile (read-only, from users table)
	FullName string
	Email    string

	// Telegram
	TelegramBotToken  string
	TelegramBotActive bool

	// IMAP
	IMAPHost     string
	IMAPPort     string
	IMAPUser     string
	IMAPPassword string

	// Resend
	ResendAPIKey string

	// SMTP (outbound email)
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string

	// AI
	AIProvider string
	AIModel    string
	AIAPIKey   string

	// Notifications
	NotifyTelegram    bool
	NotifyEmailDigest bool

	// Automations
	AutoQualify        bool
	AutoDraft          bool
	AutoSend           bool
	AutoSendDelayMin   int
	AutoFollowup       bool
	AutoFollowupDays   int
	AutoProspectToLead bool
	AutoVerifyImport   bool
}

// UserConfig holds credentials and provider settings needed by background
// services (AI, email sending, IMAP polling, Telegram).
type UserConfig struct {
	ResendAPIKey     string
	SMTPFrom         string
	SMTPHost         string
	SMTPPort         string
	SMTPUser         string
	SMTPPassword     string
	AIProvider       string
	AIModel          string
	AIAPIKey         string
	IMAPHost         string
	IMAPPort         string
	IMAPUser         string
	IMAPPassword     string
	TelegramBotToken string
}
