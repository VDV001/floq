package prospects

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingProspectIdentityLinker struct {
	mu          sync.Mutex
	invocations []linkProspectInvocation
	returnErr   error
}

type linkProspectInvocation struct {
	UserID, ProspectID             uuid.UUID
	Email, Phone, TelegramUsername string
}

func (l *recordingProspectIdentityLinker) LinkProspectToIdentity(_ context.Context, userID, prospectID uuid.UUID, email, phone, tg string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.invocations = append(l.invocations, linkProspectInvocation{
		UserID:           userID,
		ProspectID:       prospectID,
		Email:            email,
		Phone:            phone,
		TelegramUsername: tg,
	})
	return l.returnErr
}

func (l *recordingProspectIdentityLinker) takeInvocations() []linkProspectInvocation {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]linkProspectInvocation, len(l.invocations))
	copy(out, l.invocations)
	return out
}

func TestImportCSV_CallsIdentityLinker_PerProspect(t *testing.T) {
	repo := newMockRepo()
	linker := &recordingProspectIdentityLinker{}
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}), WithIdentityLinker(linker))
	userID := uuid.New()

	csv := []byte("name,email,phone,telegram_username\n" +
		"Alice,ALICE@Acme.COM,+7 999 123-45-67,@Alice_Bot\n" +
		"Bob,bob@beta.com,,\n")

	report, err := uc.ImportCSV(context.Background(), userID, csv)
	require.NoError(t, err)
	require.NotNil(t, report)
	require.Equal(t, 2, report.Imported)
	require.Len(t, repo.batched, 2)

	invs := linker.takeInvocations()
	require.Len(t, invs, 2, "linker must be invoked exactly once per imported prospect")

	// Alice — every identifier present and canonicalised by NewProspect / setters.
	assert.Equal(t, userID, invs[0].UserID)
	assert.Equal(t, repo.batched[0].ID, invs[0].ProspectID)
	assert.Equal(t, "alice@acme.com", invs[0].Email)
	assert.Equal(t, "+79991234567", invs[0].Phone)
	assert.Equal(t, "alice_bot", invs[0].TelegramUsername)

	// Bob — only email; phone/tg must surface as empty (not garbage).
	assert.Equal(t, "bob@beta.com", invs[1].Email)
	assert.Empty(t, invs[1].Phone)
	assert.Empty(t, invs[1].TelegramUsername)
}

func TestImportCSV_LinkerError_DoesNotFailImport(t *testing.T) {
	repo := newMockRepo()
	linker := &recordingProspectIdentityLinker{returnErr: errors.New("identity backend down")}
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}), WithIdentityLinker(linker))

	csv := []byte("name,email\nAlice,alice@acme.com\n")
	report, err := uc.ImportCSV(context.Background(), uuid.New(), csv)
	require.NoError(t, err, "linker errors must be logged and swallowed, not returned")
	assert.Equal(t, 1, report.Imported)
	assert.Empty(t, report.Skipped, "linker failure does not count as a skipped row")
}

func TestImportCSV_NoLinker_NoOp(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo, WithLeadChecker(&mockLeadChecker{}))

	csv := []byte("name,email\nAlice,alice@acme.com\n")
	report, err := uc.ImportCSV(context.Background(), uuid.New(), csv)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Imported)
}
