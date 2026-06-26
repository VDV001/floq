package verify

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProspectStore is a minimal in-memory ProspectStore used by usecase tests.
// It is independent of the mockProspectRepo used by handler tests so the two
// suites do not share state and the usecase can be tested in isolation.
type fakeProspectStore struct {
	prospects       []domain.ProspectWithSource
	listErr         error
	updateErr       error
	updatedIDs      []uuid.UUID
	updatedStatuses []domain.VerifyStatus
}

func (f *fakeProspectStore) ListProspects(_ context.Context, _ uuid.UUID) ([]domain.ProspectWithSource, error) {
	return f.prospects, f.listErr
}

func (f *fakeProspectStore) GetProspect(_ context.Context, _ uuid.UUID) (*domain.Prospect, error) {
	return nil, nil
}

func (f *fakeProspectStore) UpdateVerification(_ context.Context, id uuid.UUID, status domain.VerifyStatus, _ int, _ string, _ time.Time) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updatedIDs = append(f.updatedIDs, id)
	f.updatedStatuses = append(f.updatedStatuses, status)
	return nil
}

func TestUseCase_VerifyBatch_NoProspects_ReturnsZero(t *testing.T) {
	store := &fakeProspectStore{}
	uc := NewUseCase(store, nil, nil)

	count, err := uc.VerifyBatch(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestUseCase_VerifyBatch_ListError_PropagatesError(t *testing.T) {
	store := &fakeProspectStore{listErr: fmt.Errorf("db down")}
	uc := NewUseCase(store, nil, nil)

	count, err := uc.VerifyBatch(context.Background(), uuid.New())

	require.Error(t, err)
	assert.Equal(t, 0, count)
}

func TestUseCase_VerifyBatch_SkipsAlreadyChecked(t *testing.T) {
	store := &fakeProspectStore{
		prospects: []domain.ProspectWithSource{
			{Prospect: domain.Prospect{
				ID:           uuid.New(),
				Email:        "a@b.com",
				VerifyStatus: domain.VerifyStatusValid,
			}},
		},
	}
	uc := NewUseCase(store, nil, nil)

	count, err := uc.VerifyBatch(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.Empty(t, store.updatedIDs, "already-checked prospects must not be re-verified")
}

func TestUseCase_VerifyBatch_VerifiesAndUpdatesNotChecked(t *testing.T) {
	pID := uuid.New()
	store := &fakeProspectStore{
		prospects: []domain.ProspectWithSource{
			{Prospect: domain.Prospect{
				ID:           pID,
				Email:        "not-a-valid-email",
				VerifyStatus: domain.VerifyStatusNotChecked,
			}},
		},
	}
	uc := NewUseCase(store, nil, nil)

	count, err := uc.VerifyBatch(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, []uuid.UUID{pID}, store.updatedIDs)
	assert.Equal(t, domain.VerifyStatusInvalid, store.updatedStatuses[0],
		"invalid email syntax must produce VerifyStatusInvalid")
}

// fakeTelegramVerifier records what usernames the UseCase passed it
// and returns a canned result. It implements TelegramVerifier without
// pulling in the tgbotapi SDK, which is the whole point of having the
// interface.
type fakeTelegramVerifier struct {
	calledWith []string
	result     TelegramResult
}

func (f *fakeTelegramVerifier) Verify(username string) TelegramResult {
	f.calledWith = append(f.calledWith, username)
	return f.result
}

func TestUseCase_VerifyBatch_CallsTelegramVerifierForProspectsWithUsername(t *testing.T) {
	pID := uuid.New()
	store := &fakeProspectStore{
		prospects: []domain.ProspectWithSource{
			{Prospect: domain.Prospect{
				ID:               pID,
				Email:            "not-a-valid-email",
				TelegramUsername: "@someuser",
				VerifyStatus:     domain.VerifyStatusNotChecked,
			}},
		},
	}
	tg := &fakeTelegramVerifier{result: TelegramResult{Username: "someuser", Exists: true}}
	uc := NewUseCase(store, tg, nil)

	count, err := uc.VerifyBatch(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, 1, count)
	require.Len(t, tg.calledWith, 1, "TelegramVerifier must be called exactly once")
	assert.Equal(t, "@someuser", tg.calledWith[0])
}

func TestUseCase_VerifyBatch_NilTelegramVerifier_SkipsTelegramVerification(t *testing.T) {
	pID := uuid.New()
	store := &fakeProspectStore{
		prospects: []domain.ProspectWithSource{
			{Prospect: domain.Prospect{
				ID:               pID,
				Email:            "not-a-valid-email",
				TelegramUsername: "@someuser",
				VerifyStatus:     domain.VerifyStatusNotChecked,
			}},
		},
	}
	uc := NewUseCase(store, nil, nil)

	count, err := uc.VerifyBatch(context.Background(), uuid.New())

	require.NoError(t, err)
	assert.Equal(t, 1, count, "email verification must still proceed when TelegramVerifier is nil")
}

func TestUseCase_VerifyBatch_UpdateError_DoesNotIncrementCount(t *testing.T) {
	store := &fakeProspectStore{
		prospects: []domain.ProspectWithSource{
			{Prospect: domain.Prospect{
				ID:           uuid.New(),
				Email:        "not-valid",
				VerifyStatus: domain.VerifyStatusNotChecked,
			}},
		},
		updateErr: fmt.Errorf("update failed"),
	}
	uc := NewUseCase(store, nil, nil)

	count, err := uc.VerifyBatch(context.Background(), uuid.New())

	require.NoError(t, err, "individual update errors must not fail the whole batch")
	assert.Equal(t, 0, count, "failed updates must not be counted as verified")
}
