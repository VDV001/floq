package domain

import (
	"time"

	"github.com/google/uuid"
)

// ProspectStatus represents the status of a prospect in the pipeline.
type ProspectStatus string

const (
	ProspectStatusNew        ProspectStatus = "new"
	ProspectStatusInSequence ProspectStatus = "in_sequence"
	ProspectStatusConverted  ProspectStatus = "converted"
	ProspectStatusOptedOut   ProspectStatus = "opted_out"
)

// IsValid returns true if the ProspectStatus is one of the known values.
func (s ProspectStatus) IsValid() bool {
	switch s {
	case ProspectStatusNew, ProspectStatusInSequence, ProspectStatusConverted, ProspectStatusOptedOut:
		return true
	default:
		return false
	}
}

// String returns the string representation of the ProspectStatus.
func (s ProspectStatus) String() string {
	return string(s)
}

// VerifyStatus represents the email verification status of a prospect.
type VerifyStatus string

const (
	VerifyStatusNotChecked VerifyStatus = "not_checked"
	VerifyStatusValid      VerifyStatus = "valid"
	VerifyStatusInvalid    VerifyStatus = "invalid"
	VerifyStatusRisky      VerifyStatus = "risky"
)

// IsValid returns true if the VerifyStatus is one of the known values.
func (s VerifyStatus) IsValid() bool {
	switch s {
	case VerifyStatusNotChecked, VerifyStatusValid, VerifyStatusInvalid, VerifyStatusRisky:
		return true
	default:
		return false
	}
}

// String returns the string representation of the VerifyStatus.
func (s VerifyStatus) String() string {
	return string(s)
}

// Prospect is the domain entity representing a sales prospect.
type Prospect struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	Name             string
	Company          string
	Title            string
	Email            string
	Phone            string
	WhatsApp         string
	TelegramUsername string
	Industry         string
	CompanySize      string
	Context          string
	Source           string
	SourceID         *uuid.UUID
	Status           ProspectStatus
	VerifyStatus     VerifyStatus
	VerifyScore      int
	VerifyDetails    string
	VerifiedAt       *time.Time
	ConvertedLeadID  *uuid.UUID
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// NewProspect creates a new Prospect with generated ID, default statuses, and timestamps.
func NewProspect(userID uuid.UUID, name, company, title, email, source string) *Prospect {
	now := time.Now().UTC()
	return &Prospect{
		ID:            uuid.New(),
		UserID:        userID,
		Name:          name,
		Company:       company,
		Title:         title,
		Email:         email,
		Source:        source,
		Status:        ProspectStatusNew,
		VerifyStatus:  VerifyStatusNotChecked,
		VerifyDetails: "{}",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// CanLaunchSequence returns true if the prospect is eligible for a sequence launch.
func (p *Prospect) CanLaunchSequence() bool {
	if p.Status == ProspectStatusConverted || p.Status == ProspectStatusOptedOut || p.Status == ProspectStatusInSequence {
		return false
	}
	if p.VerifyStatus == VerifyStatusInvalid {
		return false
	}
	if p.VerifyStatus == VerifyStatusNotChecked && p.Email != "" {
		return false
	}
	return true
}
