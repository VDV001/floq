// Package domain models the Prospects bounded context — outbound target list:
// people we might contact proactively. A Prospect becomes a Lead once
// conversation starts (manual convert or auto-match on inbound arrival).
//
// Ubiquitous language
//
//   - Prospect              aggregate root; owned by UserID. Contact fields
//                           (email, telegram_username, phone, whatsapp) are
//                           all optional but at least a name is required.
//   - ProspectStatus        state machine: new → in_sequence → replied →
//                           converted|opted_out. Terminal: converted,
//                           opted_out. Converted is terminal because the
//                           prospect lives on via ConvertedLeadID;
//                           resurrecting it would fork identity.
//                           opted_out is terminal out of GDPR respect.
//   - VerifyStatus          result of email verification (valid, invalid,
//                           risky, not_checked).
//   - ProspectWithSource    list read-model: Prospect + JOINed SourceName.
//
// Invariants enforced by the domain
//
//   - NewProspect rejects uuid.Nil userID and empty-after-trim name,
//     symmetrically with NewLead/NewSequence/NewReminder.
//   - TransitionTo validates prospectTransitions (terminal states have no
//     outgoing edges); callers must not persist when TransitionTo errors.
//   - MarkConvertedToLead encapsulates the full "convert" operation:
//     transition + record ConvertedLeadID; fails if already terminal
//     (prevents double-conversion).
//   - CanLaunchSequence is the single source of truth for sequence
//     eligibility. The sequences context reads the pre-computed
//     IsEligibleForSequence on ProspectView; the rule itself lives here.
//
// Design notes
//
//   - Source (free-text) vs SourceID (FK to lead_sources) — both fields
//     exist; SourceID is the canonical reference once the sources
//     taxonomy is populated. Source is legacy and will be retired when
//     all rows carry SourceID.
package domain
