// Package domain models the Sequences bounded context — multi-step outbound
// outreach campaigns. A Sequence has ordered Steps; each launch against a
// prospect produces OutboundMessages that are approved, sent, and
// potentially bounced.
//
// Ubiquitous language
//
//   - Sequence         aggregate root; user-owned, inactive by default. New
//                      sequences start inactive so a mis-saved sequence
//                      never blasts prospects before the operator toggles it.
//   - SequenceStep     ordered element: channel + delay + prompt hint.
//   - StepChannel      email | telegram | phone_call.
//   - OutboundMessage  per-prospect per-step instance: draft → approved →
//                      sent → (bounced). Operator may also reject draft or
//                      approved directly.
//   - OutboundStatus   state machine: see outboundTransitions. Terminal:
//                      rejected, bounced.
//
// Invariants enforced by the domain
//
//   - NewSequence rejects uuid.Nil userID and empty/whitespace name.
//     Activate/Deactivate are explicit methods — no public IsActive toggle.
//   - OutboundMessage.TransitionTo rejects illegal status moves (e.g.
//     draft→sent without going through approved, rejected→approved).
//   - MarkSent and MarkBounced are the only ways the entity moves into
//     the terminal states; both take a caller-supplied timestamp, which
//     the entity records (SentAt / BouncedAt). This is deliberate: the
//     entity owns the clock, the repository merely persists what the
//     entity decided — see ports.go Repository.MarkSent contract ("must
//     not fall back to SQL NOW()").
//
// Cross-context read models
//
//   - ProspectView carries a pre-computed IsEligibleForSequence bool
//     populated by the adapter from prospects.Prospect.CanLaunchSequence.
//     This context never names a ProspectStatus value directly — the
//     ProspectReader port exposes MarkInSequence / MarkConverted instead,
//     keeping the prospect-status vocabulary entirely within prospects/domain.
//
// Design notes
//
//   - TxManager port enables sequences.UseCase.Launch and .ConvertToLead
//     to wrap cross-repo operations (e.g. lead creation + prospect status
//     update) in a single transaction via db.TxManager.WithTx.
package domain
