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
	// IMAPVerified is true once a connection test passed for the current
	// IMAP credentials; cleared when they change. Drives the honest
	// "Готово" onboarding signal (#222), not "fields present".
	IMAPVerified bool

	// Resend
	ResendAPIKey string
	// ResendVerified — see IMAPVerified; true once a Resend connection
	// test passed for the current key (#241).
	ResendVerified bool

	// SMTP (outbound email)
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	// SMTPVerified — see IMAPVerified; true once an SMTP connection test
	// passed for the current credentials (#222).
	SMTPVerified bool

	// AI
	AIProvider          string
	AIModel             string
	AIAPIKey            string
	AIStyleCheckEnabled bool
	// AIVerified — see IMAPVerified; true once an AI connection test
	// passed for the current provider/model/key (#222).
	AIVerified bool

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

	// Inbox view preference: when true (default), the lead detail page
	// uses the unified-identity view that merges messages from every
	// lead sharing the same Identity. When false, the page stays in
	// strict per-source mode (legacy behaviour).
	AggregatedInboxView bool
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
