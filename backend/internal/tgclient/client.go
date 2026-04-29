package tgclient

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/message/peer"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

const (
	defaultAPIID   = 611335
	defaultAPIHash = "d524b414d21f4d37f08684c1df41ac9c"
)

// memStorage implements session.Storage backed by an in-memory byte slice.
// It is safe for concurrent use.
type memStorage struct {
	mu   sync.RWMutex
	data []byte
}

func (s *memStorage) LoadSession(_ context.Context) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.data) == 0 {
		return nil, session.ErrNotFound
	}
	cp := make([]byte, len(s.data))
	copy(cp, s.data)
	return cp, nil
}

func (s *memStorage) StoreSession(_ context.Context, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make([]byte, len(data))
	copy(s.data, data)
	return nil
}

// ContextDialer is an interface for dialers that support context (e.g. SOCKS5 proxy).
type ContextDialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// Option configures a Client.
type Option func(*Client)

// WithDialer sets a custom dialer (e.g. SOCKS5 proxy) for MTProto connections.
func WithDialer(d ContextDialer) Option {
	return func(c *Client) { c.dialer = d }
}

// Client wraps a gotd/td MTProto client for personal Telegram account messaging.
type Client struct {
	apiID   int
	apiHash string
	storage *memStorage
	dialer  ContextDialer
}

// NewClient creates a new MTProto client wrapper with default Telethon credentials.
func NewClient(opts ...Option) *Client {
	c := &Client{
		apiID:   defaultAPIID,
		apiHash: defaultAPIHash,
		storage: &memStorage{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GetSessionData returns the raw session bytes for persistence.
func (c *Client) GetSessionData() []byte {
	c.storage.mu.RLock()
	defer c.storage.mu.RUnlock()
	if len(c.storage.data) == 0 {
		return nil
	}
	cp := make([]byte, len(c.storage.data))
	copy(cp, c.storage.data)
	return cp
}

// LoadSession loads previously persisted session data into the client.
func (c *Client) LoadSession(data []byte) {
	c.storage.mu.Lock()
	defer c.storage.mu.Unlock()
	if len(data) == 0 {
		c.storage.data = nil
		return
	}
	c.storage.data = make([]byte, len(data))
	copy(c.storage.data, data)
}

// newTelegramClient creates a new underlying gotd telegram.Client.
func (c *Client) newTelegramClient() *telegram.Client {
	opts := telegram.Options{
		SessionStorage: c.storage,
		NoUpdates:      true,
	}
	if c.dialer != nil {
		opts.Resolver = dcs.Plain(dcs.PlainOptions{Dial: c.dialer.DialContext})
	}
	return telegram.NewClient(c.apiID, c.apiHash, opts)
}

// normalizePhone ensures phone number starts with +.
func normalizePhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone != "" && !strings.HasPrefix(phone, "+") {
		phone = "+" + phone
	}
	return phone
}

// SendCode sends an auth code to the given phone number.
// Returns the code hash needed for SignIn.
func (c *Client) SendCode(ctx context.Context, phone string) (codeHash string, err error) {
	phone = normalizePhone(phone)
	client := c.newTelegramClient()

	err = client.Run(ctx, func(ctx context.Context) error {
		sentCode, sendErr := client.Auth().SendCode(ctx, phone, auth.SendCodeOptions{})
		if sendErr != nil {
			return fmt.Errorf("send code: %w", sendErr)
		}

		switch sc := sentCode.(type) {
		case *tg.AuthSentCode:
			codeHash = sc.PhoneCodeHash
		default:
			return fmt.Errorf("unexpected sent code type: %T", sentCode)
		}
		return nil
	})
	return codeHash, err
}

// SignIn completes authentication with the code received via SMS/Telegram.
func (c *Client) SignIn(ctx context.Context, phone, code, codeHash string) error {
	phone = normalizePhone(phone)
	client := c.newTelegramClient()

	return client.Run(ctx, func(ctx context.Context) error {
		_, err := client.Auth().SignIn(ctx, phone, code, codeHash)
		if err != nil {
			return fmt.Errorf("sign in: %w", err)
		}
		return nil
	})
}

// IsAuthorized checks whether the current session is authorized.
func (c *Client) IsAuthorized(ctx context.Context) bool {
	client := c.newTelegramClient()

	var authorized bool
	err := client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		authorized = status.Authorized
		return nil
	})
	if err != nil {
		return false
	}
	return authorized
}

// SendMessage sends a text message to a Telegram user.
// Accepts phone number (+7...) or username (@user).
// Uses message.Sender fluent API (recommended by gotd/td).
// Handles FloodWait errors by sleeping and retrying once.
func (c *Client) SendMessage(ctx context.Context, target, text string) error {
	client := c.newTelegramClient()

	return client.Run(ctx, func(ctx context.Context) error {
		api := tg.NewClient(client)
		sender := message.NewSender(api).WithResolver(peer.DefaultResolver(api))

		return c.sendWithRetry(ctx, sender, target, text)
	})
}

// SendMessageByUsername sends a message to a user by their @username.
func (c *Client) SendMessageByUsername(ctx context.Context, username, text string) error {
	return c.SendMessage(ctx, "@"+username, text)
}

func (c *Client) sendWithRetry(ctx context.Context, sender *message.Sender, target, text string) error {
	err := c.doSend(ctx, sender, target, text)
	if err == nil {
		return nil
	}

	// Handle FloodWait: sleep and retry once.
	if d, ok := tgerr.AsFloodWait(err); ok {
		log.Printf("[tgclient] flood wait: sleeping %v", d)
		timer := time.NewTimer(d + time.Second)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
		return c.doSend(ctx, sender, target, text)
	}

	return err
}

func (c *Client) doSend(ctx context.Context, sender *message.Sender, target, text string) error {
	var reqBuilder *message.RequestBuilder

	if strings.HasPrefix(target, "@") {
		// Resolve by username
		reqBuilder = sender.Resolve(target)
	} else {
		// Resolve by phone number
		reqBuilder = sender.ResolvePhone(target)
	}

	_, err := reqBuilder.Text(ctx, text)
	if err != nil {
		return fmt.Errorf("send message to %s: %w", target, err)
	}
	return nil
}
