package inbox

// This file holds customer-visible presentation copy produced by the
// inbox reply pipeline. It lives in the inbox context — not in the
// composition root (cmd/server) or the transport dispatchers — because
// user-facing strings are context knowledge, not wiring (see CLAUDE.md:
// UI strings belong in the context's messages, not usecase/composition).

// EmailSubjectFor maps a pending-reply kind to the customer-visible
// email subject line. New kinds added to PendingReplyKind SHOULD add a
// case here; the generic default guarantees a non-empty subject even for
// an unknown/misconfigured kind so a dispatcher never sends "" as the
// subject.
func EmailSubjectFor(kind PendingReplyKind) string {
	switch kind {
	case PendingReplyKindBookingLink:
		return "Запись на встречу"
	default:
		return "Сообщение"
	}
}
