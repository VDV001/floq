//go:build integration

package onec_test

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/integrations/onec"
	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/daniil/floq/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestRepository_MappingConfig_RoundTrip(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := onec.NewRepository(pool, testCipher(t))
	ctx := context.Background()
	user := testutil.SeedUser(t, pool)

	cfg, err := domain.NewMappingConfig(user, []domain.MappingRule{
		{ExternalType: "Документ.ОплатаПокупателя", Kind: domain.EventKindPayment, EmailField: "counterparty_email"},
		{ExternalType: "Справочник.Контрагенты", Kind: domain.EventKindCounterpartyCreated, EmailField: "email"},
	})
	require.NoError(t, err)

	require.NoError(t, repo.SaveMappingConfig(ctx, cfg, true))

	loaded, err := repo.GetMappingConfig(ctx, user)
	require.NoError(t, err)
	require.Equal(t, user, loaded.UserID)
	require.Len(t, loaded.Rules, 2)

	r, ok := loaded.Resolve("Документ.ОплатаПокупателя")
	require.True(t, ok)
	require.Equal(t, domain.EventKindPayment, r.Kind)
	require.Equal(t, "counterparty_email", r.EmailField)
}

func TestRepository_GetMappingConfig_NotFound(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := onec.NewRepository(pool, testCipher(t))
	ctx := context.Background()
	user := testutil.SeedUser(t, pool)

	_, err := repo.GetMappingConfig(ctx, user)
	require.True(t, errors.Is(err, onec.ErrMappingNotFound), "got %v", err)
}

func TestRepository_GetActiveMappingConfig_RespectsIsActive(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := onec.NewRepository(pool, testCipher(t))
	ctx := context.Background()
	user := testutil.SeedUser(t, pool)

	cfg, err := domain.NewMappingConfig(user, []domain.MappingRule{
		{ExternalType: "Документ.Оплата", Kind: domain.EventKindPayment},
	})
	require.NoError(t, err)

	// Saved inactive → not returned by the active getter (mapping disabled).
	require.NoError(t, repo.SaveMappingConfig(ctx, cfg, false))
	_, err = repo.GetActiveMappingConfig(ctx, user)
	require.True(t, errors.Is(err, onec.ErrMappingNotFound), "inactive config must not be active; got %v", err)

	// The general getter still loads it (for the settings UI).
	_, err = repo.GetMappingConfig(ctx, user)
	require.NoError(t, err)

	// Activate → now returned by the active getter.
	require.NoError(t, repo.SaveMappingConfig(ctx, cfg, true))
	got, err := repo.GetActiveMappingConfig(ctx, user)
	require.NoError(t, err)
	require.Len(t, got.Rules, 1)
}

func TestRepository_SaveMappingConfig_Upsert(t *testing.T) {
	pool := testutil.TestDB(t)
	repo := onec.NewRepository(pool, testCipher(t))
	ctx := context.Background()
	user := testutil.SeedUser(t, pool)

	first, err := domain.NewMappingConfig(user, []domain.MappingRule{
		{ExternalType: "Документ.Оплата", Kind: domain.EventKindPayment},
	})
	require.NoError(t, err)
	require.NoError(t, repo.SaveMappingConfig(ctx, first, true))

	second, err := domain.NewMappingConfig(user, []domain.MappingRule{
		{ExternalType: "Документ.Отгрузка", Kind: domain.EventKindShipment},
		{ExternalType: "Справочник.Контрагенты", Kind: domain.EventKindCounterpartyCreated},
	})
	require.NoError(t, err)
	require.NoError(t, repo.SaveMappingConfig(ctx, second, false))

	loaded, err := repo.GetMappingConfig(ctx, user)
	require.NoError(t, err)
	require.Len(t, loaded.Rules, 2, "upsert must replace rules, not append")
	_, ok := loaded.Resolve("Документ.Отгрузка")
	require.True(t, ok)
	_, ok = loaded.Resolve("Документ.Оплата")
	require.False(t, ok, "old rule must be gone after upsert")
}
