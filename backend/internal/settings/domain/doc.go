// Package domain models the Settings bounded context — per-user configuration
// (API keys, IMAP/SMTP/Resend credentials, Telegram bot token, notification
// preferences, AI provider).
//
// Invariants
//
//   - ResolveConfig(stored, fallback) returns the effective value for a
//     credential field: user-supplied takes precedence; fallback (env /
//     .env defaults) only fills holes. This rule lives in the domain
//     because the "hide secrets but apply fallbacks" semantics affect
//     several downstream callers (AI, Outbound, Inbox).
//
// Design notes
//
//   - The settings package is intentionally thin — most "settings" operations
//     are passthrough to the DB. The value here is the ResolveConfig rule
//     and the TelegramValidator port (which the infrastructure layer
//     fulfills with HTTPTelegramValidator).
package domain
