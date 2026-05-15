package security

import (
	"fmt"
	"strings"
)

// ToolCall is a single agent action about to be executed (send an
// email, post a Telegram message, update a CRM record). The firewall
// inspects (action, args, preceding-input severity) and decides
// pass / require-confirm / refuse.
type ToolCall struct {
	Action string
	Args   map[string]any
	// InputSeverity is the verdict the InputFirewall returned for the
	// inbound payload that triggered this tool call. Block-flagged
	// input cannot fan out into write actions even if the action
	// itself looks benign.
	InputSeverity Severity
}

// ToolDecision is the firewall verdict on a tool call. RequiresHumanConfirm
// means: the executor MUST pause for human approval; this is not a
// block, it's a policy gate. Allowed=false is an outright refusal.
type ToolDecision struct {
	Allowed              bool
	RequiresHumanConfirm bool
	Reason               string
}

// ToolCallPolicy is the per-deployment configuration. Empty defaults
// are intentionally permissive on the read-side and restrictive on
// the write-side (default-deny for unknown actions if KnownActions
// is non-empty).
type ToolCallPolicy struct {
	// AllowedExternalDomains: email recipient domains that pass
	// without human confirm. Empty = any domain requires confirm.
	AllowedExternalDomains []string
	// KnownActions: if non-empty, any Action not in this list is
	// refused. If empty, unknown actions are *not* refused (relaxed
	// mode for early integration). Production should always set this.
	KnownActions []string
}

// destructiveActions is the static set of agent actions that mutate
// external state. Read actions like qualify_lead, classify_intent are
// not in this list — they don't need human confirm even when input
// was warn-flagged because they don't reach the user.
var destructiveActions = map[string]bool{
	"send_email":      true,
	"send_telegram":   true,
	"forward_message": true,
	"update_record":   true,
	"update_prospect": true,
	"create_task":     true,
	"delete_lead":     true,
	"create_draft":    false, // draft is internal, not destructive
}

// ToolCallFirewall is the second defensive layer. Constructed with a
// policy; thread-safe (no internal mutable state after construction).
type ToolCallFirewall struct {
	policy ToolCallPolicy
}

func NewToolCallFirewall(policy ToolCallPolicy) *ToolCallFirewall {
	return &ToolCallFirewall{policy: policy}
}

// Inspect returns the firewall verdict. Decision tree:
//  1. If Action is unknown (and KnownActions is set) → refuse.
//  2. If Action is read-only (qualify, classify, get_*) → always
//     pass: read actions don't reach the user, so input severity
//     is moot.
//  3. If Action is destructive AND input was Block → refuse
//     (a blocked payload cannot launder into a tool call).
//  4. If Action is destructive AND input was Warn → require confirm.
//  5. If Action is send_email AND recipient domain is not allowlisted
//     → require confirm.
//  6. Otherwise → pass.
func (f *ToolCallFirewall) Inspect(call ToolCall) ToolDecision {
	// 1. Unknown action default-deny (when KnownActions configured).
	if len(f.policy.KnownActions) > 0 {
		known := false
		for _, a := range f.policy.KnownActions {
			if a == call.Action {
				known = true
				break
			}
		}
		if !known {
			return ToolDecision{
				Allowed: false,
				Reason:  fmt.Sprintf("unknown action %q — default-deny", call.Action),
			}
		}
	}

	isDestructive := destructiveActions[call.Action]

	// 2. Read-only actions pass regardless of input severity. They
	// don't reach the user — only the AI's internal pipeline. A
	// jailbreak attempt arriving at qualify_lead can produce a
	// garbage qualification, but cannot leak data or send messages.
	if !isDestructive {
		return ToolDecision{Allowed: true}
	}

	// 3. Blocked input cannot fan out to destructive writes.
	if call.InputSeverity == SeverityBlock {
		return ToolDecision{
			Allowed: false,
			Reason:  "blocked input cannot trigger tool call",
		}
	}

	// 4. Warn-flagged input + destructive action → require confirm.
	if call.InputSeverity == SeverityWarn {
		return ToolDecision{
			Allowed:              true,
			RequiresHumanConfirm: true,
			Reason:               "destructive action on warn-flagged input requires human confirm",
		}
	}

	// 5. send_email recipient domain check.
	if call.Action == "send_email" {
		to, _ := call.Args["to"].(string)
		if to != "" && !f.isDomainAllowed(to) {
			return ToolDecision{
				Allowed:              true,
				RequiresHumanConfirm: true,
				Reason:               fmt.Sprintf("recipient %q is on external (non-allowlisted) domain", to),
			}
		}
	}

	return ToolDecision{Allowed: true}
}

// isDomainAllowed returns true if the recipient's domain is in the
// allowlist. Empty allowlist = no domain passes (everything requires
// confirm) — defensive default for unconfigured deployments.
func (f *ToolCallFirewall) isDomainAllowed(recipient string) bool {
	at := strings.LastIndex(recipient, "@")
	if at < 0 {
		return false
	}
	domain := strings.ToLower(recipient[at+1:])
	for _, d := range f.policy.AllowedExternalDomains {
		if strings.EqualFold(d, domain) {
			return true
		}
	}
	return false
}
