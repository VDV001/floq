package settings

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResendErrorToUserMessage(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "auth wraps ErrResendAuth",
			err:  ErrResendAuth,
			want: "Неверный API ключ Resend",
		},
		{
			name: "request transport failure wraps ErrResendRequest",
			err:  fmt.Errorf("%w: connection reset", ErrResendRequest),
			want: "Ошибка запроса",
		},
		{
			name: "unrecognized error falls back to generic message",
			err:  errors.New("something else"),
			want: "Ошибка Resend",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, resendErrorToUserMessage(tc.err))
		})
	}
}
