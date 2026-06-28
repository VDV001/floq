package inbox

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- #208: email intake retry cap / quarantine ---

// reconcileIntake decides, from a processEmail outcome, whether the source email
// should be marked \Seen (consumed) or left unseen for retry. A success consumes
// immediately; a transient error leaves it unseen until the retry cap is reached,
// at which point the poison email is quarantined: consumed (so it stops
// hot-looping) and reported via the quarantine observer (#208).
func TestEmailReconcileIntake_RetryThenQuarantine(t *testing.T) {
	var quarantined []string
	poller := &EmailPoller{
		retries:      newRetryTracker(3),
		onQuarantine: func(ch string) { quarantined = append(quarantined, ch) },
	}
	procErr := errors.New("deterministic intake failure")

	// Attempts 1 and 2: below the cap → leave unseen, no quarantine.
	if poller.reconcileIntake("uid-1", "x@example.com", procErr) {
		t.Fatalf("attempt 1 must leave the email unseen for retry")
	}
	if poller.reconcileIntake("uid-1", "x@example.com", procErr) {
		t.Fatalf("attempt 2 must leave the email unseen for retry")
	}
	require.Empty(t, quarantined, "no quarantine before the cap is reached")

	// Attempt 3: cap reached → mark seen (consume) and quarantine.
	if !poller.reconcileIntake("uid-1", "x@example.com", procErr) {
		t.Fatalf("attempt 3 (cap) must consume the email so it stops hot-looping")
	}
	assert.Equal(t, []string{"email"}, quarantined, "the cap must fire one email quarantine signal")
}

// A successful intake consumes the email and never quarantines, and it resets the
// failure count so an unrelated later failure for the same UID starts fresh.
func TestEmailReconcileIntake_SuccessConsumesAndResets(t *testing.T) {
	var quarantined []string
	poller := &EmailPoller{
		retries:      newRetryTracker(2),
		onQuarantine: func(ch string) { quarantined = append(quarantined, ch) },
	}
	procErr := errors.New("transient")

	poller.reconcileIntake("uid-9", "y@example.com", procErr) // 1 failure
	if !poller.reconcileIntake("uid-9", "y@example.com", nil) {
		t.Fatalf("a nil error must consume the email")
	}
	// Count was reset, so the next failure is attempt 1, not the cap.
	if poller.reconcileIntake("uid-9", "y@example.com", procErr) {
		t.Fatalf("after a success the failure count must reset, leaving the email unseen")
	}
	require.Empty(t, quarantined, "success path must never quarantine")
}

// A poller built without a quarantine observer (nil onQuarantine) must not panic
// when the retry cap is reached — the observer is optional, mirroring the
// nil-safety of the tracker itself.
func TestEmailReconcileIntake_NilObserverDoesNotPanic(t *testing.T) {
	poller := &EmailPoller{retries: newRetryTracker(1)} // onQuarantine left nil
	if !poller.reconcileIntake("uid-x", "z@example.com", errors.New("boom")) {
		t.Fatalf("cap of 1 must consume on the first failure")
	}
}

// --- #208: telegram intake retry cap / quarantine ---

// poisonFetcher re-delivers the same update on every poll while the offset has
// not advanced past it, mimicking Telegram re-delivering an un-confirmed update.
// Once the receive loop advances the offset past the poison update (quarantine),
// it cancels the context to terminate the loop. A hard call cap is a backstop so
// a buggy loop fails fast instead of hanging the test.
type poisonFetcher struct {
	mu      sync.Mutex
	update  tgbotapi.Update
	offsets []int
	cancel  context.CancelFunc
}

func (f *poisonFetcher) GetUpdates(cfg tgbotapi.UpdateConfig) ([]tgbotapi.Update, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.offsets = append(f.offsets, cfg.Offset)
	if cfg.Offset > f.update.UpdateID || len(f.offsets) > 50 {
		f.cancel()
		return nil, nil
	}
	return []tgbotapi.Update{f.update}, nil
}

// A deterministically-failing update must not be re-delivered forever: after the
// retry cap the loop advances the offset past it (quarantine) and fires the
// quarantine observer, so the poller resumes consuming later updates.
func TestReceiveLoop_QuarantinesPoisonUpdateAfterCap(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetcher := &poisonFetcher{
		update: tgbotapi.Update{UpdateID: 500, Message: makeTgMessage(7, "Poison", "", "boom")},
		cancel: cancel,
	}

	var attempts int
	handle := func(_ context.Context, _ *tgbotapi.Message) error {
		attempts++
		return errors.New("deterministic intake failure")
	}

	var quarantined []string
	bot := &TelegramBot{
		logger:       slog.Default(),
		retries:      newRetryTracker(3),
		onQuarantine: func(ch string) { quarantined = append(quarantined, ch) },
	}
	bot.receiveLoop(ctx, fetcher, handle)

	assert.Equal(t, 3, attempts, "handler is attempted exactly cap times before quarantine")
	assert.Equal(t, []string{"telegram"}, quarantined, "the cap must fire one telegram quarantine signal")

	fetcher.mu.Lock()
	offsets := fetcher.offsets
	fetcher.mu.Unlock()
	assert.Equal(t, 501, offsets[len(offsets)-1],
		"after quarantine the loop must request past the poison update (500+1)")
}
