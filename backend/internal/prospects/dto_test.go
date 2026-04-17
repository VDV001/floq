package prospects

import (
	"testing"
	"time"

	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestProspectWithSourceToResponse(t *testing.T) {
	now := time.Now().UTC()
	sourceID := uuid.New()
	leadID := uuid.New()
	item := &domain.ProspectWithSource{
		Prospect: domain.Prospect{
			ID:               uuid.New(),
			UserID:           uuid.New(),
			Name:             "Alice",
			Company:          "Acme",
			Title:            "CEO",
			Email:            "alice@acme.com",
			Phone:            "+7999",
			WhatsApp:         "+7999wa",
			TelegramUsername: "@alice",
			Industry:         "SaaS",
			CompanySize:      "10-50",
			Context:          "met at conf",
			Source:           "manual",
			SourceID:         &sourceID,
			Status:           domain.ProspectStatusNew,
			VerifyStatus:     domain.VerifyStatusValid,
			VerifyScore:      95,
			VerifyDetails:    `{"ok":true}`,
			VerifiedAt:       &now,
			ConvertedLeadID:  &leadID,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		SourceName: "LinkedIn",
	}

	resp := ProspectWithSourceToResponse(item)

	assert.Equal(t, item.ID, resp.ID)
	assert.Equal(t, item.UserID, resp.UserID)
	assert.Equal(t, "Alice", resp.Name)
	assert.Equal(t, "Acme", resp.Company)
	assert.Equal(t, "CEO", resp.Title)
	assert.Equal(t, "alice@acme.com", resp.Email)
	assert.Equal(t, "+7999", resp.Phone)
	assert.Equal(t, "+7999wa", resp.WhatsApp)
	assert.Equal(t, "@alice", resp.TelegramUsername)
	assert.Equal(t, "SaaS", resp.Industry)
	assert.Equal(t, "10-50", resp.CompanySize)
	assert.Equal(t, "met at conf", resp.Context)
	assert.Equal(t, "manual", resp.Source)
	assert.Equal(t, &sourceID, resp.SourceID)
	assert.Equal(t, "LinkedIn", resp.SourceName)
	assert.Equal(t, "new", resp.Status)
	assert.Equal(t, "valid", resp.VerifyStatus)
	assert.Equal(t, 95, resp.VerifyScore)
	assert.Equal(t, `{"ok":true}`, resp.VerifyDetails)
	assert.Equal(t, &now, resp.VerifiedAt)
	assert.Equal(t, &leadID, resp.ConvertedLeadID)
	assert.Equal(t, now, resp.CreatedAt)
	assert.Equal(t, now, resp.UpdatedAt)
}

func TestProspectsToResponse(t *testing.T) {
	prospects := []domain.ProspectWithSource{
		{Prospect: domain.Prospect{ID: uuid.New(), Name: "Alice", Status: domain.ProspectStatusNew, VerifyStatus: domain.VerifyStatusNotChecked}},
		{Prospect: domain.Prospect{ID: uuid.New(), Name: "Bob", Status: domain.ProspectStatusConverted, VerifyStatus: domain.VerifyStatusValid}},
	}

	resp := ProspectsToResponse(prospects)

	assert.Len(t, resp, 2)
	assert.Equal(t, "Alice", resp[0].Name)
	assert.Equal(t, "new", resp[0].Status)
	assert.Equal(t, "Bob", resp[1].Name)
	assert.Equal(t, "converted", resp[1].Status)
}

func TestProspectsToResponse_Empty(t *testing.T) {
	resp := ProspectsToResponse([]domain.ProspectWithSource{})
	assert.Empty(t, resp)
}
