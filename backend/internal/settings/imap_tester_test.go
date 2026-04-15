package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIMAPTester_TestConnection_ConnectionRefused(t *testing.T) {
	tester := &IMAPTester{}
	err := tester.TestConnection("127.0.0.1", "19993", "user", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Не удалось подключиться")
}
