package prospects

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/google/uuid"
)

var columnAliases = map[string]string{
	"name": "name", "имя": "name", "имя в tg": "name", "full_name": "name",
	"company": "company", "компания": "company", "организация": "company",
	"title": "title", "должность": "title", "позиция": "title",
	"email": "email", "почта": "email", "e-mail": "email",
	"phone": "phone", "телефон": "phone", "тел": "phone",
	"whatsapp": "whatsapp",
	"telegram_username": "telegram_username", "tg-контакты": "telegram_username", "tg": "telegram_username", "telegram": "telegram_username",
	"industry": "industry", "отрасль": "industry",
	"company_size": "company_size",
	"context": "context", "комментарий": "context", "описание": "context", "превью вакансии": "context",
}

type LeadChecker interface {
	LeadExistsByEmail(ctx context.Context, userID uuid.UUID, email string) (bool, error)
}

type UseCase struct {
	repo        domain.Repository
	leadChecker LeadChecker
}

func NewUseCase(repo domain.Repository, opts ...func(*UseCase)) *UseCase {
	uc := &UseCase{repo: repo}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

func WithLeadChecker(lc LeadChecker) func(*UseCase) {
	return func(uc *UseCase) { uc.leadChecker = lc }
}

func (uc *UseCase) ListProspects(ctx context.Context, userID uuid.UUID) ([]domain.ProspectWithSource, error) {
	return uc.repo.ListProspects(ctx, userID)
}

func (uc *UseCase) GetProspect(ctx context.Context, id uuid.UUID) (*domain.Prospect, error) {
	return uc.repo.GetProspect(ctx, id)
}

func (uc *UseCase) FindByEmail(ctx context.Context, userID uuid.UUID, email string) (*domain.Prospect, error) {
	return uc.repo.FindByEmail(ctx, userID, email)
}

// CreateProspectInput holds the data needed to create a prospect.
type CreateProspectInput struct {
	UserID           uuid.UUID
	Name             string
	Company          string
	Title            string
	Email            string
	Phone            string
	WhatsApp         string
	TelegramUsername string
	Industry         string
	CompanySize      string
	Context          string
	SourceID         *uuid.UUID
}

func (uc *UseCase) CreateProspect(ctx context.Context, input CreateProspectInput) (*domain.Prospect, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("prospect name is required")
	}
	if input.Email != "" {
		existing, err := uc.repo.FindByEmail(ctx, input.UserID, input.Email)
		if err != nil {
			return nil, fmt.Errorf("prospect dedup: %w", err)
		}
		if existing != nil {
			return nil, fmt.Errorf("проспект с таким email уже существует")
		}
	}
	if input.Email != "" && uc.leadChecker != nil {
		exists, err := uc.leadChecker.LeadExistsByEmail(ctx, input.UserID, input.Email)
		if err != nil {
			return nil, fmt.Errorf("lead check: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("лид с таким email уже существует")
		}
	}
	p, err := domain.NewProspect(input.UserID, input.Name, input.Company, input.Title, input.Email, "manual")
	if err != nil {
		return nil, fmt.Errorf("construct prospect: %w", err)
	}
	p.Phone = input.Phone
	p.WhatsApp = input.WhatsApp
	p.TelegramUsername = input.TelegramUsername
	p.Industry = input.Industry
	p.CompanySize = input.CompanySize
	p.Context = input.Context
	p.SourceID = input.SourceID
	if err := uc.repo.CreateProspect(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (uc *UseCase) DeleteProspect(ctx context.Context, id uuid.UUID) error {
	return uc.repo.DeleteProspect(ctx, id)
}

func (uc *UseCase) ImportCSV(ctx context.Context, userID uuid.UUID, csvData []byte) (int, error) {
	csvData = stripBOM(csvData)
	delimiter := detectDelimiter(csvData)

	reader := csv.NewReader(bytes.NewReader(csvData))
	reader.Comma = delimiter
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("read csv header: %w", err)
	}

	colMap := mapColumns(header)
	if _, ok := colMap["name"]; !ok {
		return 0, fmt.Errorf("invalid csv header: required column 'name' (or alias: имя, имя в tg) not found")
	}

	getCol := func(record []string, canonical string) string {
		if idx, ok := colMap[canonical]; ok && idx < len(record) {
			return strings.TrimSpace(record[idx])
		}
		return ""
	}

	var prospects []domain.Prospect
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("read csv record: %w", err)
		}

		email := getCol(record, "email")
		tgUsername := strings.TrimPrefix(getCol(record, "telegram_username"), "@")

		if email != "" {
			dup, err := uc.repo.FindByEmail(ctx, userID, email)
			if err != nil {
				return 0, fmt.Errorf("dedup prospect check: %w", err)
			}
			if dup != nil {
				continue
			}
			if uc.leadChecker != nil {
				exists, err := uc.leadChecker.LeadExistsByEmail(ctx, userID, email)
				if err != nil {
					return 0, fmt.Errorf("dedup lead check: %w", err)
				}
				if exists {
					continue
				}
			}
		} else if tgUsername != "" {
			dup, err := uc.repo.FindByTelegramUsername(ctx, userID, tgUsername)
			if err != nil {
				return 0, fmt.Errorf("dedup prospect tg check: %w", err)
			}
			if dup != nil {
				continue
			}
		}

		name := getCol(record, "name")
		p, err := domain.NewProspect(userID, name, getCol(record, "company"), getCol(record, "title"), email, "csv")
		if err != nil {
			continue
		}
		p.Phone = getCol(record, "phone")
		p.WhatsApp = getCol(record, "whatsapp")
		p.TelegramUsername = tgUsername
		p.Industry = getCol(record, "industry")
		p.CompanySize = getCol(record, "company_size")
		p.Context = getCol(record, "context")
		prospects = append(prospects, *p)
	}

	if err := uc.repo.CreateProspectsBatch(ctx, prospects); err != nil {
		return 0, err
	}

	return len(prospects), nil
}

func stripBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return data[3:]
	}
	return data
}

func detectDelimiter(data []byte) rune {
	idx := bytes.IndexByte(data, '\n')
	firstLine := string(data)
	if idx > 0 {
		firstLine = string(data[:idx])
	}
	if strings.Count(firstLine, ";") > strings.Count(firstLine, ",") {
		return ';'
	}
	return ','
}

func mapColumns(header []string) map[string]int {
	result := make(map[string]int, len(header))
	for i, raw := range header {
		normalized := strings.ToLower(strings.TrimSpace(raw))
		if canonical, ok := columnAliases[normalized]; ok {
			if _, exists := result[canonical]; !exists {
				result[canonical] = i
			}
		}
	}
	return result
}

func (uc *UseCase) TemplateCSV() []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"name", "company", "title", "email", "phone", "whatsapp", "telegram_username", "industry", "company_size", "context"})
	_ = w.Write([]string{"Иван Петров", "ООО Рога и Копыта", "Менеджер", "ivan@example.com", "+79991234567", "", "ivan_petrov", "IT", "10-50", "Заинтересован в интеграции"})
	w.Flush()
	return buf.Bytes()
}

func (uc *UseCase) ExportCSV(ctx context.Context, userID uuid.UUID) ([]byte, error) {
	prospects, err := uc.repo.ListProspects(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list prospects: %w", err)
	}

	var buf bytes.Buffer
	// BOM for Excel compatibility
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(&buf)
	header := []string{"name", "company", "title", "email", "phone", "whatsapp", "telegram_username", "industry", "company_size", "context", "source", "status"}
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}

	for _, p := range prospects {
		record := []string{
			p.Name,
			p.Company,
			p.Title,
			p.Email,
			p.Phone,
			p.WhatsApp,
			p.TelegramUsername,
			p.Industry,
			p.CompanySize,
			p.Context,
			p.Source,
			p.Status.String(),
		}
		if err := w.Write(record); err != nil {
			return nil, fmt.Errorf("write csv record: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flush csv: %w", err)
	}

	return buf.Bytes(), nil
}
