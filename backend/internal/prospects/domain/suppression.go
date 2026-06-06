package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/daniil/floq/internal/normalize"
	"github.com/google/uuid"
)

// SuppressionChannel is the outbound channel an address is suppressed on.
type SuppressionChannel string

const (
	SuppressionChannelEmail    SuppressionChannel = "email"
	SuppressionChannelTelegram SuppressionChannel = "telegram"
)

// IsValid reports whether c is a known suppression channel.
func (c SuppressionChannel) IsValid() bool {
	switch c {
	case SuppressionChannelEmail, SuppressionChannelTelegram:
		return true
	default:
		return false
	}
}

// String returns the string representation of the channel.
func (c SuppressionChannel) String() string { return string(c) }

// Domain errors guarding Suppression construction.
var (
	ErrInvalidSuppressionChannel  = errors.New("suppression: invalid channel")
	ErrSuppressionAddressRequired = errors.New("suppression: address is required")
	ErrSuppressionReasonRequired  = errors.New("suppression: reason is required")
)

// NormalizeSuppressionAddress canonicalizes an address for its channel so that
// writes and lookups match regardless of casing or "@"/whitespace noise. Email
// is lowercased+trimmed; Telegram handles drop a leading "@" and lowercase.
func NormalizeSuppressionAddress(channel SuppressionChannel, address string) string {
	switch channel {
	case SuppressionChannelEmail:
		return normalize.Email(address)
	case SuppressionChannelTelegram:
		return normalize.TelegramUsername(address)
	default:
		return strings.TrimSpace(address)
	}
}

// Suppression is an immutable record that an address must never be contacted
// again on a given channel — the hard pre-check ahead of the consent rule.
type Suppression struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Channel   SuppressionChannel
	Address   string
	Reason    string
	CreatedAt time.Time
}

// NewSuppression builds a validated Suppression: a known channel, a non-empty
// normalized address, and a non-empty reason (mandatory for the audit trail).
func NewSuppression(userID uuid.UUID, channel SuppressionChannel, address, reason string) (*Suppression, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("suppression userID is required")
	}
	if !channel.IsValid() {
		return nil, ErrInvalidSuppressionChannel
	}
	addr := NormalizeSuppressionAddress(channel, address)
	if addr == "" {
		return nil, ErrSuppressionAddressRequired
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrSuppressionReasonRequired
	}
	return &Suppression{
		ID:        uuid.New(),
		UserID:    userID,
		Channel:   channel,
		Address:   addr,
		Reason:    reason,
		CreatedAt: time.Now().UTC(),
	}, nil
}
