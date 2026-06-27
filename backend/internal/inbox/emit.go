package inbox

import "context"

// writeThenEmit runs the domain write and, when a transactional outbox emitter
// is wired (hasEmitter && tx != nil), the emit inside ONE db.WithTx — so the
// inbox state change and its webhook enqueue commit together or not at all
// (#199, fail-closed: a failed emit rolls the write back). With no emitter or no
// TxManager it runs write alone, leaving the caller to fire its legacy
// best-effort observer. Shared by the Telegram and email pollers, whose intake
// (lead.created) and auto-qualification (lead.qualified) paths share this shape.
func writeThenEmit(ctx context.Context, tx TxManager, hasEmitter bool, write, emit func(context.Context) error) error {
	if hasEmitter && tx != nil {
		return tx.WithTx(ctx, func(txCtx context.Context) error {
			if err := write(txCtx); err != nil {
				return err
			}
			return emit(txCtx)
		})
	}
	return write(ctx)
}
