package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ProspectStatus represents the status of a prospect in the pipeline.
type ProspectStatus string

const (
	ProspectStatusNew        ProspectStatus = "new"
	ProspectStatusInSequence ProspectStatus = "in_sequence"
	ProspectStatusReplied    ProspectStatus = "replied"
	ProspectStatusConverted  ProspectStatus = "converted"
	ProspectStatusOptedOut   ProspectStatus = "opted_out"
)

// IsValid returns true if the ProspectStatus is one of the known values.
func (s ProspectStatus) IsValid() bool {
	switch s {
	case ProspectStatusNew, ProspectStatusInSequence, ProspectStatusReplied, ProspectStatusConverted, ProspectStatusOptedOut:
		return true
	default:
		return false
	}
}

// String returns the string representation of the ProspectStatus.
func (s ProspectStatus) String() string {
	return string(s)
}

// prospectTransitions encodes the business-legal state machine for prospects:
//   - new → in_sequence (launched into outreach), converted (manual move to leads),
//     opted_out (user unsubscribed)
//   - in_sequence → replied (prospect answered), converted, opted_out
//   - replied → converted (turned into a lead), opted_out
//   - converted / opted_out are terminal.
//
// Converted is terminal because a prospect that became a lead lives on via
// ConvertedLeadID — resurrecting it would fork identity. Opted_out is terminal
// out of GDPR/user-intent respect: once they said no, we don't re-target.
var prospectTransitions = map[ProspectStatus][]ProspectStatus{
	ProspectStatusNew:        {ProspectStatusInSequence, ProspectStatusConverted, ProspectStatusOptedOut},
	ProspectStatusInSequence: {ProspectStatusReplied, ProspectStatusConverted, ProspectStatusOptedOut},
	ProspectStatusReplied:    {ProspectStatusConverted, ProspectStatusOptedOut},
}

// CanTransitionTo reports whether target is a legal next state for s.
func (s ProspectStatus) CanTransitionTo(target ProspectStatus) bool {
	for _, allowed := range prospectTransitions[s] {
		if allowed == target {
			return true
		}
	}
	return false
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

// ProspectWithSource is a read model for list views: Prospect plus its
// joined source_name projection. Kept out of the entity proper so the
// aggregate stays pure of persistence-driven projections. Only assembled
// by the repository in ListProspects.
type ProspectWithSource struct {
	Prospect
	SourceName string
}

// NewProspect creates a new Prospect with generated ID, default statuses, and
// timestamps. Returns an error if required invariants are violated:
//   - userID must not be the zero UUID (every prospect belongs to someone);
//   - name must be non-empty after trimming (a prospect without a name is
//     unaddressable in the UI and the cross-channel matcher).
//
// Mirrors the invariant enforcement in NewLead — upstream callers (CSV
// import, manual create, AI-extracted) must not produce malformed aggregates.
func NewProspect(userID uuid.UUID, name, company, title, email, source string) (*Prospect, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("prospect userID is required")
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("prospect name is required")
	}
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
	}, nil
}

// CanLaunchSequence returns true if the prospect is eligible for a sequence launch.
//
// This is the single source of truth for sequence eligibility — both the
// prospects and sequences contexts consult it (sequences calls via a port
// that delegates here, so the rule lives in one place).
func (p *Prospect) CanLaunchSequence() bool {
	if p.Status == ProspectStatusConverted || p.Status == ProspectStatusOptedOut || p.Status == ProspectStatusInSequence || p.Status == ProspectStatusReplied {
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

// TransitionTo validates and applies a status transition. Returns an error if
// the target is invalid or the current state forbids it — callers must not
// persist when TransitionTo fails.
func (p *Prospect) TransitionTo(target ProspectStatus) error {
	if !target.IsValid() {
		return fmt.Errorf("invalid prospect status: %q", target)
	}
	if !p.Status.CanTransitionTo(target) {
		return fmt.Errorf("cannot transition prospect from %q to %q", p.Status, target)
	}
	p.Status = target
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// MarkConvertedToLead performs the new/in_sequence/replied → converted
// transition and records the resulting lead ID on the prospect.
func (p *Prospect) MarkConvertedToLead(leadID uuid.UUID) error {
	if err := p.TransitionTo(ProspectStatusConverted); err != nil {
		return err
	}
	p.ConvertedLeadID = &leadID
	return nil
}
