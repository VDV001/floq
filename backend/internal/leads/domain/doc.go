// Package domain models the Leads bounded context — conversations with people
// who reached out to us (inbound), plus the AI-generated artifacts attached
// to each conversation (qualification, draft reply, reminder).
//
// Ubiquitous language
//
//   - Lead           the aggregate root. A single person in a single inbound
//                    channel conversation. Identified by ID; owned by UserID.
//   - LeadStatus     state machine: new → qualified → in_conversation →
//                    followup → closed|won. Terminal: closed, won.
//   - Channel        telegram | email — how the inbound arrived.
//   - Qualification  AI-scored assessment of budget/need/timeline. Score is
//                    clamped to [0,100] at construction (NewQualification)
//                    or rehydration (RehydrateQualification).
//   - Draft          AI-generated reply body. Non-empty invariant enforced.
//   - Reminder       "this lead has gone quiet" prompt; dismissible.
//   - Message        inbound/outbound exchange on a lead.
//
// Invariants enforced by the domain
//
//   - Lead.TransitionTo — illegal status transitions return errors; see
//     allowedTransitions.
//   - Lead.OnOutboundSent — sending a message to a Qualified lead auto-
//     transitions to InConversation (the rule lives on the entity, not in
//     the usecase).
//   - Lead.InheritsSourceFrom / SetSource — manual operator decisions on
//     source_id are preserved; cross-channel dedup may only fill an empty
//     source slot. Returns (newSource, changed) so callers persist only
//     when needed; pointer-equality guessing is not required.
//   - Qualification score is clamped to [0,100] at every construction path;
//     IsHot / IsWarm thresholds are declared as domain constants.
//   - Draft body must be non-empty.
//   - Reminder requires leadID and message; Dismiss is idempotent.
//
// Errors
//
//   - ErrLeadNotFound, ErrProspectNotFound are deliberately
//     indistinguishable from "not yours" — cross-tenant probing cannot
//     leak existence through error-message diffing.
//
// Ports (interfaces fulfilled by outer layers)
//
//   - Repository     persistence; transaction-aware via db.ConnFromCtx.
//   - AIService      qualification + reply-draft generation.
//   - MessageSender  outbound telegram/email delivery.
//   - Notifier       operator-facing alerts.
//   - ProspectSuggestionFinder  cross-channel dedup (issue #6): loads
//     candidate matches, atomically links via MarkConvertedToLead on
//     Prospect, copies source via InheritsSourceFrom/SetSource on Lead.
//
// Design notes
//
//   - Read-model separation: LeadWithSource embeds Lead plus the JOIN-
//     projected SourceName, keeping projection fields off the aggregate.
//   - Domain services (file prefix service_*) are stateless functions that
//     don't belong on any single entity — see service_call_detection.go.
package domain
