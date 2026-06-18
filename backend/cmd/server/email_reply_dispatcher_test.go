package main

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/daniil/floq/internal/inbox"
	"github.com/google/uuid"
)

// --- fakes ---

type fakeEmailSender struct {
	mu       sync.Mutex
	calls    []emailSendCall
	failWith error
}

type emailSendCall struct {
	userID  uuid.UUID
	to      string
	subject string
	body    string
}

func (f *fakeEmailSender) SendEmail(_ context.Context, userID uuid.UUID, to, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, emailSendCall{userID: userID, to: to, subject: subject, body: body})
	return f.failWith
}

func (f *fakeEmailSender) Calls() []emailSendCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]emailSendCall(nil), f.calls...)
}

func targetWithEmail(email string) *inbox.ReplyTarget {
	return &inbox.ReplyTarget{EmailAddress: &email}
}

// --- Dispatch ---

func TestEmailReplyDispatcher_HappyPath_SendsAndPersists(t *testing.T) {
	leadID := uuid.New()
	sender := &fakeEmailSender{}
	targets := &fakeReplyTargetLookup{targets: map[uuid.UUID]*inbox.ReplyTarget{leadID: targetWithEmail("lead@example.com")}}
	writer := &fakeInboxMessageWriter{}

	d := newEmailReplyDispatcher(sender, targets, writer)
	pr := newPendingReplyT(t, leadID, inbox.ChannelEmail)

	if err := d.Dispatch(context.Background(), pr); err != nil {
		t.Fatalf("Dispatch error: %v", err)
	}

	calls := sender.Calls()
	if len(calls) != 1 {
		t.Fatalf("sender called %d times, want 1", len(calls))
	}
	if calls[0].to != "lead@example.com" {
		t.Errorf("to = %q, want lead@example.com", calls[0].to)
	}
	if calls[0].userID != pr.UserID {
		t.Errorf("userID = %v, want %v — dispatcher must forward pr.UserID for multi-tenant config resolution", calls[0].userID, pr.UserID)
	}
	if calls[0].subject != "Запись на встречу" {
		t.Errorf("subject = %q, want booking-link subject", calls[0].subject)
	}
	if calls[0].body != pr.Body {
		t.Errorf("body = %q, want %q", calls[0].body, pr.Body)
	}
	if writer.writtenCount() != 1 {
		t.Errorf("outbound message persisted %d times, want 1 — successful send must write the thread record", writer.writtenCount())
	}
}

func TestEmailReplyDispatcher_RejectsNonEmailChannel(t *testing.T) {
	d := newEmailReplyDispatcher(&fakeEmailSender{}, &fakeReplyTargetLookup{}, &fakeInboxMessageWriter{})
	pr := newPendingReplyT(t, uuid.New(), inbox.ChannelTelegram)

	err := d.Dispatch(context.Background(), pr)
	if err == nil {
		t.Fatal("expected unsupported-channel error, got nil")
	}
}

func TestEmailReplyDispatcher_LeadFetchError_PropagatesAndDoesNotSend(t *testing.T) {
	leadID := uuid.New()
	sender := &fakeEmailSender{}
	targets := &fakeReplyTargetLookup{getErr: errors.New("db hiccup")}
	d := newEmailReplyDispatcher(sender, targets, &fakeInboxMessageWriter{})
	pr := newPendingReplyT(t, leadID, inbox.ChannelEmail)

	if err := d.Dispatch(context.Background(), pr); err == nil {
		t.Fatal("expected error from lead fetch, got nil")
	}
	if len(sender.Calls()) != 0 {
		t.Errorf("sender fired %d times despite lead-fetch error — must not send when ownership lookup fails", len(sender.Calls()))
	}
}

func TestEmailReplyDispatcher_LeadWithoutEmailAddress_Errors(t *testing.T) {
	leadID := uuid.New()
	sender := &fakeEmailSender{}
	targets := &fakeReplyTargetLookup{targets: map[uuid.UUID]*inbox.ReplyTarget{leadID: { /* no EmailAddress */}}}
	d := newEmailReplyDispatcher(sender, targets, &fakeInboxMessageWriter{})
	pr := newPendingReplyT(t, leadID, inbox.ChannelEmail)

	if err := d.Dispatch(context.Background(), pr); err == nil {
		t.Fatal("expected error when lead has no email_address, got nil")
	}
	if len(sender.Calls()) != 0 {
		t.Errorf("sender fired despite no email on lead")
	}
}

func TestEmailReplyDispatcher_SendFailure_DoesNotPersistOutbound(t *testing.T) {
	leadID := uuid.New()
	sender := &fakeEmailSender{failWith: errors.New("smtp 5xx")}
	targets := &fakeReplyTargetLookup{targets: map[uuid.UUID]*inbox.ReplyTarget{leadID: targetWithEmail("lead@example.com")}}
	writer := &fakeInboxMessageWriter{}

	d := newEmailReplyDispatcher(sender, targets, writer)
	pr := newPendingReplyT(t, leadID, inbox.ChannelEmail)

	if err := d.Dispatch(context.Background(), pr); err == nil {
		t.Fatal("expected error to propagate from sender")
	}
	if writer.writtenCount() != 0 {
		t.Errorf("outbound persisted %d times despite send failure — must not write the thread record for a failed send (mirror telegramReplyDispatcher contract)", writer.writtenCount())
	}
}

// --- channelReplyDispatcher ---

func TestChannelReplyDispatcher_RoutesTelegramToTelegramBranch(t *testing.T) {
	tg := &recordingDispatcher{name: "telegram"}
	email := &recordingDispatcher{name: "email"}
	d := newChannelReplyDispatcher(tg, email)
	pr := newPendingReplyT(t, uuid.New(), inbox.ChannelTelegram)

	if err := d.Dispatch(context.Background(), pr); err != nil {
		t.Fatalf("Dispatch error: %v", err)
	}
	if tg.calls != 1 || email.calls != 0 {
		t.Errorf("telegram=%d email=%d, want telegram=1 email=0", tg.calls, email.calls)
	}
}

func TestChannelReplyDispatcher_RoutesEmailToEmailBranch(t *testing.T) {
	tg := &recordingDispatcher{name: "telegram"}
	email := &recordingDispatcher{name: "email"}
	d := newChannelReplyDispatcher(tg, email)
	pr := newPendingReplyT(t, uuid.New(), inbox.ChannelEmail)

	if err := d.Dispatch(context.Background(), pr); err != nil {
		t.Fatalf("Dispatch error: %v", err)
	}
	if email.calls != 1 || tg.calls != 0 {
		t.Errorf("email=%d telegram=%d, want email=1 telegram=0", email.calls, tg.calls)
	}
}

func TestChannelReplyDispatcher_NilBranchReturnsUnsupported(t *testing.T) {
	d := newChannelReplyDispatcher(&recordingDispatcher{}, nil)
	pr := newPendingReplyT(t, uuid.New(), inbox.ChannelEmail)

	err := d.Dispatch(context.Background(), pr)
	if !errors.Is(err, ErrChannelDispatcherUnsupported) {
		t.Errorf("err = %v, want ErrChannelDispatcherUnsupported — unwired channel must surface a clear sentinel, not panic", err)
	}
}

type recordingDispatcher struct {
	name    string
	calls   int
	failErr error
}

func (r *recordingDispatcher) Dispatch(_ context.Context, _ *inbox.PendingReply) error {
	r.calls++
	return r.failErr
}
