package settings

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"
)

// IMAPContextDialer allows dialing TCP connections through a proxy.
type IMAPContextDialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

// IMAPTester tests IMAP server connectivity.
type IMAPTester struct {
	Dialer IMAPContextDialer
}

// TestConnection attempts a TLS connection to the IMAP server, sends a LOGIN
// command, and returns nil on success or an error describing the failure.
func (t *IMAPTester) TestConnection(host, port, user, password string) error {
	addr := net.JoinHostPort(host, port)

	var conn *tls.Conn
	var err error
	if t.Dialer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		rawConn, dialErr := t.Dialer.DialContext(ctx, "tcp", addr)
		if dialErr != nil {
			return fmt.Errorf("Не удалось подключиться: %v", dialErr)
		}
		conn = tls.Client(rawConn, &tls.Config{ServerName: host})
	} else {
		conn, err = tls.DialWithDialer(
			&net.Dialer{Timeout: 10 * time.Second},
			"tcp", addr,
			&tls.Config{ServerName: host},
		)
		if err != nil {
			return fmt.Errorf("Не удалось подключиться: %v", err)
		}
	}
	defer conn.Close()

	// Read server greeting.
	buf := make([]byte, 1024)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Read(buf)
	if err != nil {
		return fmt.Errorf("Сервер не отвечает")
	}

	// Send LOGIN command with quoted strings.
	loginCmd := fmt.Sprintf("A1 LOGIN \"%s\" \"%s\"\r\n", user, password)
	if _, err = conn.Write([]byte(loginCmd)); err != nil {
		return fmt.Errorf("Ошибка отправки команды")
	}

	// Read response (may be multiple lines).
	var response string
	for i := 0; i < 5; i++ {
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			break
		}
		response += string(buf[:n])
		if strings.Contains(response, "A1 OK") || strings.Contains(response, "A1 NO") || strings.Contains(response, "A1 BAD") {
			break
		}
	}

	// Send LOGOUT.
	_, _ = conn.Write([]byte("A2 LOGOUT\r\n"))

	if strings.Contains(response, "A1 OK") {
		return nil
	}
	return fmt.Errorf("Неверный логин или пароль")
}
