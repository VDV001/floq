package inbox

import (
	"context"
	"errors"
	"sync"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

// --- fakes ---

type fakeBotSender struct {
	mu       sync.Mutex
	sent     []tgbotapi.Chattable
	failWith error
}

func (f *fakeBotSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, c)
	if f.failWith != nil {
		return tgbotapi.Message{}, f.failWith
	}
	return tgbotapi.Message{MessageID: 42}, nil
}

func (f *fakeBotSender) sentCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.sent)
}

type fakeReplyTargetLookup struct {
	targets map[uuid.UUID]*ReplyTarget
	getErr  error
}

func (f *fakeReplyTargetLookup) LookupReplyTarget(_ context.Context, id uuid.UUID) (*ReplyTarget, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.targets[id], nil
}

type fakeInboxMessageWriter struct {
	mu      sync.Mutex
	written []*InboxMessage
	failErr error
}

func (f *fakeInboxMessageWriter) CreateMessage(_ context.Context, msg *InboxMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.written = append(f.written, msg)
	if f.failErr != nil {
		return f.failErr
	}
	return nil
}

func (f *fakeInboxMessageWriter) writtenCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.written)
}

// --- helpers ---

func newPendingReplyT(t *testing.T, leadID uuid.UUID, channel Channel) *PendingReply {
	t.Helper()
	pr, err := NewPendingReply(uuid.New(), leadID, channel, PendingReplyKindBookingLink, "hi")
	if err != nil {
		t.Fatalf("fixture build: %v", err)
	}
	return pr
}

func targetWithChat(chatID int64) *ReplyTarget {
	return &ReplyTarget{TelegramChatID: &chatID}
}

// --- Dispatch ---

func TestTelegramReplyDispatcher_HappyPath_SendsAndPersists(t *testing.T) {
	leadID := uuid.New()
	bot := &fakeBotSender{}
	targets := &fakeReplyTargetLookup{targets: map[uuid.UUID]*ReplyTarget{leadID: targetWithChat(12345)}}
	writer := &fakeInboxMessageWriter{}

	d := NewTelegramReplyDispatcher(bot, targets, writer)
	pr := newPendingReplyT(t, leadID, ChannelTelegram)

	if err := d.Dispatch(context.Background(), pr); err != nil {
		t.Fatalf("Dispatch error: %v", err)
	}

	if bot.sentCount() != 1 {
		t.Errorf("bot.Send calls = %d, want 1", bot.sentCount())
	}
	if writer.writtenCount() != 1 {
		t.Errorf("CreateMessage calls = %d, want 1 (outbound history)", writer.writtenCount())
	}
	if got := writer.written[0]; got.Direction != DirectionOutbound || got.LeadID != leadID || got.Body != pr.Body {
		t.Errorf("written message = %+v, want outbound for lead %v with body %q", got, leadID, pr.Body)
	}
	if msg, ok := bot.sent[0].(tgbotapi.MessageConfig); !ok || msg.ChatID != 12345 || msg.Text != pr.Body {
		t.Errorf("bot received = %+v, want chat=12345 body=%q", bot.sent[0], pr.Body)
	}
}

func TestTelegramReplyDispatcher_RejectsNonTelegramChannel(t *testing.T) {
	d := NewTelegramReplyDispatcher(&fakeBotSender{}, &fakeReplyTargetLookup{}, &fakeInboxMessageWriter{})
	pr := newPendingReplyT(t, uuid.New(), ChannelEmail)

	err := d.Dispatch(context.Background(), pr)
	if err == nil {
		t.Fatal("Dispatch must reject non-telegram channels until per-channel routing is added")
	}
}

func TestTelegramReplyDispatcher_LeadNotFoundReturnsError(t *testing.T) {
	bot := &fakeBotSender{}
	targets := &fakeReplyTargetLookup{targets: map[uuid.UUID]*ReplyTarget{}} // empty
	writer := &fakeInboxMessageWriter{}

	d := NewTelegramReplyDispatcher(bot, targets, writer)
	pr := newPendingReplyT(t, uuid.New(), ChannelTelegram)

	if err := d.Dispatch(context.Background(), pr); err == nil {
		t.Fatal("Dispatch must error when lead is missing")
	}
	if bot.sentCount() != 0 {
		t.Error("must NOT call bot.Send when lead lookup yields nothing")
	}
	if writer.writtenCount() != 0 {
		t.Error("must NOT persist outbound message when lead lookup yields nothing")
	}
}

func TestTelegramReplyDispatcher_LeadWithoutChatIDReturnsError(t *testing.T) {
	leadID := uuid.New()
	bot := &fakeBotSender{}
	targets := &fakeReplyTargetLookup{targets: map[uuid.UUID]*ReplyTarget{leadID: { /* TelegramChatID nil */}}}
	writer := &fakeInboxMessageWriter{}

	d := NewTelegramReplyDispatcher(bot, targets, writer)
	pr := newPendingReplyT(t, leadID, ChannelTelegram)

	if err := d.Dispatch(context.Background(), pr); err == nil {
		t.Fatal("Dispatch must error when lead has no TelegramChatID")
	}
	if bot.sentCount() != 0 {
		t.Error("must NOT call bot.Send when no chat id")
	}
}

func TestTelegramReplyDispatcher_LeadFetchErrorPropagates(t *testing.T) {
	bot := &fakeBotSender{}
	targets := &fakeReplyTargetLookup{getErr: errors.New("db down")}
	writer := &fakeInboxMessageWriter{}

	d := NewTelegramReplyDispatcher(bot, targets, writer)
	pr := newPendingReplyT(t, uuid.New(), ChannelTelegram)

	err := d.Dispatch(context.Background(), pr)
	if err == nil || !errors.Is(err, targets.getErr) {
		t.Fatalf("want wrapped fetch error, got %v", err)
	}
	if bot.sentCount() != 0 {
		t.Error("must NOT send when lead lookup fails")
	}
}

func TestTelegramReplyDispatcher_BotSendErrorSkipsPersist(t *testing.T) {
	leadID := uuid.New()
	bot := &fakeBotSender{failWith: errors.New("telegram 500")}
	targets := &fakeReplyTargetLookup{targets: map[uuid.UUID]*ReplyTarget{leadID: targetWithChat(99)}}
	writer := &fakeInboxMessageWriter{}

	d := NewTelegramReplyDispatcher(bot, targets, writer)
	pr := newPendingReplyT(t, leadID, ChannelTelegram)

	err := d.Dispatch(context.Background(), pr)
	if err == nil || !errors.Is(err, bot.failWith) {
		t.Fatalf("want wrapped bot error, got %v", err)
	}
	if writer.writtenCount() != 0 {
		t.Error("CreateMessage must NOT run after a failed bot.Send — otherwise UI would show a sent message that never reached the customer")
	}
}

func TestTelegramReplyDispatcher_PersistErrorReturnsErrorAfterSendSucceeds(t *testing.T) {
	leadID := uuid.New()
	bot := &fakeBotSender{}
	targets := &fakeReplyTargetLookup{targets: map[uuid.UUID]*ReplyTarget{leadID: targetWithChat(1)}}
	writer := &fakeInboxMessageWriter{failErr: errors.New("db boom")}

	d := NewTelegramReplyDispatcher(bot, targets, writer)
	pr := newPendingReplyT(t, leadID, ChannelTelegram)

	err := d.Dispatch(context.Background(), pr)
	if err == nil {
		t.Fatal("Dispatch must surface persist failure")
	}
	if bot.sentCount() != 1 {
		t.Error("send already happened — count must still be 1")
	}
}
