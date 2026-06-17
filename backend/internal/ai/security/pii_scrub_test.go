package security

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPIIScrub_Email(t *testing.T) {
	s := NewPIIScrubber()
	r := s.Scrub("Напишите мне на ivan.petrov@example.com пожалуйста")
	assert.NotContains(t, r.Scrubbed, "ivan.petrov@example.com")
	assert.Contains(t, r.Scrubbed, "[EMAIL_1]")
	assert.Equal(t, "ivan.petrov@example.com", r.Mapping["[EMAIL_1]"])
}

func TestPIIScrub_Phone(t *testing.T) {
	s := NewPIIScrubber()
	for _, raw := range []string{"+7 (912) 345-67-89", "89123456789", "+79123456789"} {
		r := s.Scrub("Телефон " + raw + " звоните")
		assert.NotContains(t, r.Scrubbed, raw, "phone %q must be scrubbed", raw)
		assert.Contains(t, r.Scrubbed, "[PHONE_1]")
	}
}

func TestPIIScrub_INN(t *testing.T) {
	s := NewPIIScrubber()
	r := s.Scrub("ИНН 7707083893 для договора")
	assert.NotContains(t, r.Scrubbed, "7707083893")
	assert.Contains(t, r.Scrubbed, "[INN_1]")
}

func TestPIIScrub_FullName(t *testing.T) {
	s := NewPIIScrubber()
	r := s.Scrub("Договор подписывает Иванов Иван Иванович от лица компании")
	assert.NotContains(t, r.Scrubbed, "Иванов Иван Иванович")
	assert.Contains(t, r.Scrubbed, "[NAME_1]")
}

func TestPIIScrub_DedupSameValue(t *testing.T) {
	s := NewPIIScrubber()
	r := s.Scrub("a@b.com и снова a@b.com")
	assert.Equal(t, 1, len(r.Mapping), "same value must map to one placeholder")
	assert.Equal(t, 2, strings.Count(r.Scrubbed, "[EMAIL_1]"))
}

func TestPIIScrub_MultipleDistinct(t *testing.T) {
	s := NewPIIScrubber()
	r := s.Scrub("first@x.com second@y.com")
	assert.Equal(t, 2, len(r.Mapping))
	assert.Contains(t, r.Scrubbed, "[EMAIL_1]")
	assert.Contains(t, r.Scrubbed, "[EMAIL_2]")
}

func TestPIIScrub_RoundTripRestore(t *testing.T) {
	s := NewPIIScrubber()
	original := "Свяжитесь с Иванов Иван Иванович, ivan@example.com, +7 912 345-67-89"
	r := s.Scrub(original)
	restored := s.Restore(r.Scrubbed, r.Mapping)
	assert.Equal(t, original, restored)
}

func TestPIIScrub_BenignUnchanged(t *testing.T) {
	s := NewPIIScrubber()
	r := s.Scrub("Нужна интеграция CRM к третьему кварталу, бюджет около 500к")
	assert.Equal(t, "Нужна интеграция CRM к третьему кварталу, бюджет около 500к", r.Scrubbed)
	assert.Empty(t, r.Mapping)
}

func TestPIIScrub_RestoreInGeneratedDraft(t *testing.T) {
	s := NewPIIScrubber()
	// Simulates the LLM producing a draft referencing the placeholder.
	r := s.Scrub("Клиент ivan@example.com просит звонок")
	draft := "Здравствуйте! Перезвоним вам и продублируем на " + "[EMAIL_1]" + "."
	require.Contains(t, r.Mapping, "[EMAIL_1]")
	restored := s.Restore(draft, r.Mapping)
	assert.Contains(t, restored, "ivan@example.com")
	assert.NotContains(t, restored, "[EMAIL_1]")
}
