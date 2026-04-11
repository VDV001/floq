package settings

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"time"
)

// IMAPTester tests IMAP server connectivity.
type IMAPTester struct{}

// TestConnection attempts a TLS connection to the IMAP server, sends a LOGIN
// command, and returns nil on success or an error describing the failure.
func (t *IMAPTester) TestConnection(host, port, user, password string) error {
	addr := net.JoinHostPort(host, port)
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp", addr,
		&tls.Config{ServerName: host},
	)
	if err != nil {
		return fmt.Errorf("Не удалось подключиться: %v", err)
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
