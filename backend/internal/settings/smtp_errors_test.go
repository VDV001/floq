package settings

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSmtpErrorToUserMessage(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "proxy dial wraps ErrSMTPProxyDial",
			err:  fmt.Errorf("%w: connection refused", ErrSMTPProxyDial),
			want: "Не удалось подключиться через прокси",
		},
		{
			name: "direct dial wraps ErrSMTPDial",
			err:  fmt.Errorf("%w: i/o timeout", ErrSMTPDial),
			want: "Не удалось подключиться",
		},
		{
			name: "smtp client wraps ErrSMTPClient",
			err:  fmt.Errorf("%w: bad greeting", ErrSMTPClient),
			want: "Ошибка создания SMTP-клиента",
		},
		{
			name: "starttls wraps ErrSMTPStartTLS",
			err:  fmt.Errorf("%w: x509 unknown authority", ErrSMTPStartTLS),
			want: "Ошибка STARTTLS",
		},
		{
			name: "auth wraps ErrSMTPAuth",
			err:  fmt.Errorf("%w: 535 5.7.8", ErrSMTPAuth),
			want: "Неверный логин или пароль SMTP",
		},
		{
			name: "unrecognized error falls back to generic message",
			err:  errors.New("something else"),
			want: "Ошибка SMTP",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, smtpErrorToUserMessage(tc.err))
		})
	}
}
