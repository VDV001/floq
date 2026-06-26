package onec

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/google/uuid"
)

// counterpartyExternalType is the dedup external_type for counterparties Floq
// creates in 1C. Paired with a Floq-side external_id (the lead's email, else
// name), it keeps re-qualifying the same lead from creating duplicates.
const counterpartyExternalType = "counterparty"

// OutboundUseCase pushes Floq-side objects to a tenant's 1C endpoint. It is the
// Floq→1C direction, mirroring the inbound UseCase. Failures are recorded in the
// sync ledger so reconciliation (#109) and the settings UI can see them.
type OutboundUseCase struct {
	store  OutboundStore
	client OneCClient
	logger *slog.Logger
}

// NewOutboundUseCase wires the outbound use case. A nil logger falls back to the
// default so callers (and tests) need not supply one.
func NewOutboundUseCase(store OutboundStore, client OneCClient, logger *slog.Logger) *OutboundUseCase {
	if logger == nil {
		logger = slog.Default()
	}
	return &OutboundUseCase{store: store, client: client, logger: logger}
}

// PushCounterparty creates a counterparty in the user's 1C from a qualified
// lead. It is idempotent (a counterparty already pushed is skipped) and a no-op
// when the tenant has not configured an outbound endpoint. A 1C failure is
// recorded as an 'error' ledger entry and returned for observability — callers
// triggered by a qualification must NOT let that error fail their flow.
func (u *OutboundUseCase) PushCounterparty(ctx context.Context, userID uuid.UUID, draft *domain.CounterpartyDraft) error {
	creds, err := u.store.GetOutboundCredentials(ctx, userID)
	if errors.Is(err, ErrOutboundNotConfigured) {
		return nil // outbound disabled for this tenant — silent no-op
	}
	if err != nil {
		return fmt.Errorf("onec: resolve outbound credentials: %w", err)
	}

	externalID := draft.Email
	if externalID == "" {
		externalID = draft.Name
	}

	already, err := u.store.OutboundProcessedExists(ctx, userID, externalID, counterpartyExternalType)
	if err != nil {
		return fmt.Errorf("onec: dedup check: %w", err)
	}
	if already {
		return nil // counterparty already created in 1C
	}

	ref, pushErr := u.client.CreateCounterparty(ctx, creds, draft)
	status := domain.SyncStatusProcessed
	if pushErr != nil {
		status = domain.SyncStatusError
	}

	rec, err := domain.NewOutboundSyncRecord(userID, externalID, counterpartyExternalType, domain.EventKindCounterpartyCreated, status)
	if err != nil {
		return fmt.Errorf("onec: build outbound record: %w", err)
	}
	if upErr := u.store.UpsertOutboundRecord(ctx, rec); upErr != nil {
		// Recording failed too — keep both in the chain so errors.Is sees each.
		if pushErr != nil {
			return errors.Join(fmt.Errorf("onec: push counterparty: %w", pushErr),
				fmt.Errorf("onec: ledger write: %w", upErr))
		}
		return fmt.Errorf("onec: ledger write: %w", upErr)
	}

	if pushErr != nil {
		u.logger.Warn("onec: counterparty push to 1C failed; recorded as error",
			"user_id", userID, "external_id", externalID, "err", pushErr)
		return fmt.Errorf("onec: push counterparty: %w", pushErr)
	}

	u.logger.Info("onec: counterparty created in 1C",
		"user_id", userID, "external_id", externalID, "ref", ref)
	return nil
}
