package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/daniil/floq/internal/inbox"
)

// Compile-time check that *channelReplyDispatcher satisfies the inbox
// ReplyDispatcher port.
var _ inbox.ReplyDispatcher = (*channelReplyDispatcher)(nil)

// ErrChannelDispatcherUnsupported signals that a PendingReply arrived
// on a channel the router has no dispatcher for. Distinct sentinel so
// the usecase layer can map it to a clear operator-facing error
// rather than conflating with a transport-layer 5xx.
var ErrChannelDispatcherUnsupported = errors.New("channel reply dispatcher: unsupported channel")

// channelReplyDispatcher routes an approved PendingReply to the
// channel-specific dispatcher (telegram, email, …) based on
// pr.Channel. It is the single ReplyDispatcher the inbox usecase
// holds; channel-aware fanout lives behind this seam so the usecase
// stays oblivious to transport.
//
// Either dispatcher may be nil at construction so the composition
// root can build the router before every transport is initialised;
// a Dispatch call on an unwired channel returns the unsupported
// sentinel so misconfiguration surfaces as a clear error rather than
// a panic.
type channelReplyDispatcher struct {
	telegram inbox.ReplyDispatcher
	email    inbox.ReplyDispatcher
}

func newChannelReplyDispatcher(telegram, email inbox.ReplyDispatcher) *channelReplyDispatcher {
	return &channelReplyDispatcher{telegram: telegram, email: email}
}

// Dispatch picks the channel-specific dispatcher and delegates. A
// nil branch surfaces ErrChannelDispatcherUnsupported so an
// unwired channel returns a clear, errors.Is-matchable sentinel
// instead of panicking.
func (d *channelReplyDispatcher) Dispatch(ctx context.Context, pr *inbox.PendingReply) error {
	var target inbox.ReplyDispatcher
	switch pr.Channel {
	case inbox.ChannelTelegram:
		target = d.telegram
	case inbox.ChannelEmail:
		target = d.email
	default:
		return fmt.Errorf("%w: %q", ErrChannelDispatcherUnsupported, pr.Channel)
	}
	if target == nil {
		return fmt.Errorf("%w: %q", ErrChannelDispatcherUnsupported, pr.Channel)
	}
	return target.Dispatch(ctx, pr)
}
