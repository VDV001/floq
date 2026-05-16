package domain

import (
	"errors"
	"time"

	"github.com/daniil/floq/internal/normalize"
	"github.com/google/uuid"
)

// ErrIdentityNoIdentifiers is returned by NewIdentity when none of the
// three identifier fields (email, phone, telegram_username) survive
// normalization. An identity without any handle to match against is
// useless to the resolver.
var ErrIdentityNoIdentifiers = errors.New("identity requires at least one of email, phone, telegram_username")

// Identity is the unified contact aggregate the IdentityResolver pivots
// on: a single person across Email, Telegram and prospect channels. All
// identifier fields are stored canonicalized (lowercase + trimmed for
// Email and TelegramUsername; digits with optional leading + for Phone)
// so deterministic equality checks suffice for matching.
type Identity struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	Email            string
	Phone            string
	TelegramUsername string
	CreatedAt        time.Time
}

// NewIdentity canonicalizes the supplied identifiers via the shared
// normalize kernel and constructs an Identity aggregate. At least one
// of email / phone / telegram_username must survive normalization,
// otherwise ErrIdentityNoIdentifiers is returned.
func NewIdentity(userID uuid.UUID, email, phone, telegramUsername string) (*Identity, error) {
	e := normalize.Email(email)
	p := normalize.Phone(phone)
	tg := normalize.TelegramUsername(telegramUsername)
	if e == "" && p == "" && tg == "" {
		return nil, ErrIdentityNoIdentifiers
	}
	return &Identity{
		ID:               uuid.New(),
		UserID:           userID,
		Email:            e,
		Phone:            p,
		TelegramUsername: tg,
		CreatedAt:        time.Now().UTC(),
	}, nil
}
