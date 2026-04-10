package tgclient

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
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

// Client wraps a gotd/td MTProto client for personal Telegram account messaging.
type Client struct {
	apiID   int
	apiHash string
	storage *memStorage
}

// NewClient creates a new MTProto client wrapper with default Telethon credentials.
func NewClient() *Client {
	return &Client{
		apiID:   defaultAPIID,
		apiHash: defaultAPIHash,
		storage: &memStorage{},
	}
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
	return telegram.NewClient(c.apiID, c.apiHash, telegram.Options{
		SessionStorage: c.storage,
		NoUpdates:      true,
	})
}

// SendCode sends an auth code to the given phone number.
// Returns the code hash needed for SignIn.
func (c *Client) SendCode(ctx context.Context, phone string) (codeHash string, err error) {
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

// SendMessage resolves a phone number to a Telegram peer and sends a text message.
// Handles FloodWait errors by sleeping and retrying once.
func (c *Client) SendMessage(ctx context.Context, phone, text string) error {
	client := c.newTelegramClient()

	return client.Run(ctx, func(ctx context.Context) error {
		api := tg.NewClient(client)

		return c.sendMessageWithRetry(ctx, api, phone, text)
	})
}

func (c *Client) sendMessageWithRetry(ctx context.Context, api *tg.Client, phone, text string) error {
	err := c.doSendMessage(ctx, api, phone, text)
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
		return c.doSendMessage(ctx, api, phone, text)
	}

	return err
}

func (c *Client) doSendMessage(ctx context.Context, api *tg.Client, phone, text string) error {
	resolved, err := api.ContactsResolvePhone(ctx, phone)
	if err != nil {
		return fmt.Errorf("resolve phone %s: %w", phone, err)
	}

	if len(resolved.Users) == 0 {
		return fmt.Errorf("no user found for phone %s", phone)
	}

	user, ok := resolved.Users[0].AsNotEmpty()
	if !ok {
		return fmt.Errorf("resolved user is empty for phone %s", phone)
	}

	accessHash, ok := user.GetAccessHash()
	if !ok {
		return fmt.Errorf("no access hash for user %d", user.ID)
	}

	peer := &tg.InputPeerUser{
		UserID:     user.ID,
		AccessHash: accessHash,
	}

	randomID, err := cryptoRandomInt64()
	if err != nil {
		return fmt.Errorf("generate random id: %w", err)
	}

	_, err = api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:     peer,
		Message:  text,
		RandomID: randomID,
	})
	if err != nil {
		return fmt.Errorf("send message to %s: %w", phone, err)
	}

	return nil
}

// cryptoRandomInt64 generates a cryptographically random int64 for message dedup.
func cryptoRandomInt64() (int64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return int64(b[0]) | int64(b[1])<<8 | int64(b[2])<<16 | int64(b[3])<<24 |
		int64(b[4])<<32 | int64(b[5])<<40 | int64(b[6])<<48 | int64(b[7])<<56, nil
}
