package prospects

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/daniil/floq/internal/normalize"
	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/google/uuid"
)

// SkippedRow records a single CSV row that ImportCSV was unable to import.
// Line is 1-indexed and counts the header (so the first data row is line 2),
// matching what a user sees in a spreadsheet editor.
type SkippedRow struct {
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

// ImportReport is the structured result of a CSV import. Imported holds the
// number of prospects successfully persisted; Skipped enumerates rows that
// were not imported together with a human-readable reason. The caller (HTTP
// handler) surfaces both pieces to the UI so users can see what was dropped
// and why instead of silently observing "Импортировано 0".
type ImportReport struct {
	Imported int          `json:"imported"`
	Skipped  []SkippedRow `json:"skipped"`
}

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
	email := normalize.Email(input.Email)
	if email != "" {
		existing, err := uc.repo.FindByEmail(ctx, input.UserID, email)
		if err != nil {
			return nil, fmt.Errorf("prospect dedup: %w", err)
		}
		if existing != nil {
			return nil, fmt.Errorf("проспект с таким email уже существует")
		}
	}
	if email != "" && uc.leadChecker != nil {
		exists, err := uc.leadChecker.LeadExistsByEmail(ctx, input.UserID, email)
		if err != nil {
			return nil, fmt.Errorf("lead check: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("лид с таким email уже существует")
		}
	}
	p, err := domain.NewProspect(input.UserID, input.Name, input.Company, input.Title, email, "manual")
	if err != nil {
		return nil, fmt.Errorf("construct prospect: %w", err)
	}
	p.SetPhone(input.Phone)
	p.WhatsApp = input.WhatsApp
	p.SetTelegramUsername(input.TelegramUsername)
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

// ImportCSV parses a CSV payload and persists one prospect per non-skipped
// data row. When a row lacks an explicit contact name, the importer falls
// back to the company column, then to the email, so a CSV that only carries
// company-level identifiers (e.g. "info@acme.com") still produces an
// addressable prospect instead of being silently dropped. Rows without any
// identifier are recorded in the returned ImportReport so the UI can show
// the user exactly which lines were ignored and why.
func (uc *UseCase) ImportCSV(ctx context.Context, userID uuid.UUID, csvData []byte) (*ImportReport, error) {
	csvData = stripBOM(csvData)
	delimiter := detectDelimiter(csvData)

	reader := csv.NewReader(bytes.NewReader(csvData))
	reader.Comma = delimiter
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}

	colMap := mapColumns(header)
	if _, ok := colMap["name"]; !ok {
		return nil, fmt.Errorf("invalid csv header: required column 'name' (or alias: имя, имя в tg) not found")
	}

	getCol := func(record []string, canonical string) string {
		if idx, ok := colMap[canonical]; ok && idx < len(record) {
			return strings.TrimSpace(record[idx])
		}
		return ""
	}

	var (
		prospects []domain.Prospect
		skipped   []SkippedRow
		lineNum   = 1 // header consumed; first data row will become line 2
	)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv record: %w", err)
		}
		lineNum++

		email := normalize.Email(getCol(record, "email"))
		tgUsername := normalize.TelegramUsername(getCol(record, "telegram_username"))
		company := getCol(record, "company")

		// Fallback chain for the contact name. Domain invariant
		// (prospect.name != "") is enforced by the factory; the use case
		// supplies a sensible identifier when the CSV did not.
		name := getCol(record, "name")
		if name == "" {
			switch {
			case company != "":
				name = company
			case email != "":
				name = email
			}
		}

		if name == "" {
			skipped = append(skipped, SkippedRow{
				Line:   lineNum,
				Reason: "row has no identifier (name, company, or email is required)",
			})
			continue
		}

		if email != "" {
			dup, err := uc.repo.FindByEmail(ctx, userID, email)
			if err != nil {
				return nil, fmt.Errorf("dedup prospect check: %w", err)
			}
			if dup != nil {
				continue
			}
			if uc.leadChecker != nil {
				exists, err := uc.leadChecker.LeadExistsByEmail(ctx, userID, email)
				if err != nil {
					return nil, fmt.Errorf("dedup lead check: %w", err)
				}
				if exists {
					continue
				}
			}
		} else if tgUsername != "" {
			dup, err := uc.repo.FindByTelegramUsername(ctx, userID, tgUsername)
			if err != nil {
				return nil, fmt.Errorf("dedup prospect tg check: %w", err)
			}
			if dup != nil {
				continue
			}
		}

		p, err := domain.NewProspect(userID, name, company, getCol(record, "title"), email, "csv")
		if err != nil {
			skipped = append(skipped, SkippedRow{Line: lineNum, Reason: err.Error()})
			continue
		}
		p.SetPhone(getCol(record, "phone"))
		p.WhatsApp = getCol(record, "whatsapp")
		p.SetTelegramUsername(tgUsername)
		p.Industry = getCol(record, "industry")
		p.CompanySize = getCol(record, "company_size")
		p.Context = getCol(record, "context")
		prospects = append(prospects, *p)
	}

	if err := uc.repo.CreateProspectsBatch(ctx, prospects); err != nil {
		return nil, err
	}

	return &ImportReport{Imported: len(prospects), Skipped: skipped}, nil
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
