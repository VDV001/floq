// Package domain holds the enrichment bounded context's entities and value
// objects: a CompanyEnrichment is a per-user, per-domain record of publicly
// scraped company data plus its processing status. Business invariants
// (valid company domain, legal status, attempt accounting) live here.
package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidDomain is returned by NewDomain when the email has no usable
// company domain (malformed, missing parts, or a dot-less host).
var ErrInvalidDomain = errors.New("enrichment: email has no valid company domain")

// ErrFreeEmailProvider is returned by NewDomain when the email's host is a
// known free/personal provider (gmail, yandex, mail.ru, …) whose domain does
// not identify a company and so is not worth enriching.
var ErrFreeEmailProvider = errors.New("enrichment: email domain is a free provider")

// freeEmailProviders are personal-email hosts where the email domain is NOT a
// company domain. Curated and self-contained (kept here rather than importing
// another context) — covers the common RU + global providers seen in B2B leads.
var freeEmailProviders = map[string]struct{}{
	"gmail.com": {}, "googlemail.com": {},
	"yandex.ru": {}, "yandex.com": {}, "ya.ru": {},
	"mail.ru": {}, "bk.ru": {}, "inbox.ru": {}, "list.ru": {}, "internet.ru": {},
	"rambler.ru": {}, "outlook.com": {}, "hotmail.com": {}, "live.com": {},
	"icloud.com": {}, "me.com": {}, "proton.me": {}, "protonmail.com": {},
	"gmx.com": {}, "gmx.net": {}, "web.de": {}, "aol.com": {},
	"qq.com": {}, "163.com": {}, "126.com": {},
}

// Domain is a normalized company domain extracted from an email address. The
// zero value is invalid — always construct via NewDomain.
type Domain struct {
	value string
}

// NewDomain extracts and normalizes the company domain from an email address:
// lowercased, trimmed, with a leading "www." removed. It rejects malformed
// emails (ErrInvalidDomain) and free/personal providers (ErrFreeEmailProvider).
func NewDomain(email string) (Domain, error) {
	e := strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndex(e, "@")
	if at <= 0 || at == len(e)-1 {
		return Domain{}, ErrInvalidDomain
	}
	host := strings.TrimPrefix(e[at+1:], "www.")
	if host == "" || !strings.Contains(host, ".") {
		return Domain{}, ErrInvalidDomain
	}
	if _, free := freeEmailProviders[host]; free {
		return Domain{}, ErrFreeEmailProvider
	}
	return Domain{value: host}, nil
}

// String returns the normalized domain.
func (d Domain) String() string { return d.value }

// DomainFromStorage rehydrates a Domain from a trusted, already-normalized
// value read from persistence. Unlike NewDomain it performs no validation —
// the value was validated when it was first created.
func DomainFromStorage(value string) Domain { return Domain{value: value} }

// Status is the processing state of a CompanyEnrichment.
type Status string

const (
	StatusPending  Status = "pending"
	StatusEnriched Status = "enriched"
	StatusFailed   Status = "failed"
)

// IsValid reports whether s is a known status.
func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusEnriched, StatusFailed:
		return true
	default:
		return false
	}
}

// String returns the string representation of the Status.
func (s Status) String() string { return string(s) }

// CompanyProfile is the publicly scraped company data value object.
type CompanyProfile struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Emails      []string `json:"emails"`
	Phones      []string `json:"phones"`
	Socials     []string `json:"socials"`
}

// IsEmpty reports whether nothing useful was extracted.
func (p CompanyProfile) IsEmpty() bool {
	return p.Title == "" && p.Description == "" &&
		len(p.Emails) == 0 && len(p.Phones) == 0 && len(p.Socials) == 0
}

// CompanyEnrichment is the per-user, per-domain enrichment record/entity.
type CompanyEnrichment struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Domain     Domain
	Status     Status
	Profile    CompanyProfile
	Error      string
	Attempts   int
	EnrichedAt *time.Time
	ExpiresAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewPendingEnrichment creates a fresh enrichment record in the pending state,
// ready to be claimed by the worker. userID and a valid domain are required.
func NewPendingEnrichment(userID uuid.UUID, d Domain) (*CompanyEnrichment, error) {
	if userID == uuid.Nil {
		return nil, errors.New("enrichment: userID is required")
	}
	if d.value == "" {
		return nil, ErrInvalidDomain
	}
	now := time.Now().UTC()
	return &CompanyEnrichment{
		ID:        uuid.New(),
		UserID:    userID,
		Domain:    d,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// MarkEnriched records a successful scrape: stores the profile, clears any
// prior error, and sets enriched/expiry timestamps (TTL drives refresh).
func (e *CompanyEnrichment) MarkEnriched(profile CompanyProfile, ttlSeconds int) {
	now := time.Now().UTC()
	exp := now.Add(time.Duration(ttlSeconds) * time.Second)
	e.Profile = profile
	e.Status = StatusEnriched
	e.Error = ""
	e.EnrichedAt = &now
	e.ExpiresAt = &exp
	e.UpdatedAt = now
}

// MarkFailed records a failed scrape attempt: sets the failed status, stores
// the reason, and increments the attempt counter (drives the retry cap).
func (e *CompanyEnrichment) MarkFailed(reason string) {
	e.Status = StatusFailed
	e.Error = reason
	e.Attempts++
	e.UpdatedAt = time.Now().UTC()
}
