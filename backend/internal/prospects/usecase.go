package prospects

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

type UseCase struct {
	repo *Repository
}

func NewUseCase(repo *Repository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) ListProspects(ctx context.Context, userID uuid.UUID) ([]Prospect, error) {
	return uc.repo.ListProspects(ctx, userID)
}

func (uc *UseCase) GetProspect(ctx context.Context, id uuid.UUID) (*Prospect, error) {
	return uc.repo.GetProspect(ctx, id)
}

func (uc *UseCase) CreateProspect(ctx context.Context, prospect *Prospect) error {
	return uc.repo.CreateProspect(ctx, prospect)
}

func (uc *UseCase) DeleteProspect(ctx context.Context, id uuid.UUID) error {
	return uc.repo.DeleteProspect(ctx, id)
}

func (uc *UseCase) ImportCSV(ctx context.Context, userID uuid.UUID, csvData []byte) (int, error) {
	reader := csv.NewReader(bytes.NewReader(csvData))

	// Read and validate header
	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("read csv header: %w", err)
	}
	if len(header) < 4 || header[0] != "name" || header[1] != "company" || header[2] != "title" || header[3] != "email" {
		return 0, fmt.Errorf("invalid csv header: expected name,company,title,email")
	}

	now := time.Now().UTC()
	var prospects []Prospect
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("read csv record: %w", err)
		}

		prospects = append(prospects, Prospect{
			ID:        uuid.New(),
			UserID:    userID,
			Name:      record[0],
			Company:   record[1],
			Title:     record[2],
			Email:     record[3],
			Source:    "csv",
			Status:    "new",
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	if err := uc.repo.CreateProspectsBatch(ctx, prospects); err != nil {
		return 0, err
	}

	return len(prospects), nil
}
