package prospects

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/daniil/floq/internal/normalize"
	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/google/uuid"
)

// SkippedRow records a single CSV row that ImportCSV was unable to import.
// Line is 1-indexed and counts the header (so the first data row is line 2),
// matching what a user sees in a spreadsheet editor.
type SkippedRow struct {
	Line   int
	Reason string
}

// ImportReport is the structured result of a CSV import. Imported holds the
// number of prospects successfully persisted; Skipped enumerates rows that
// were not imported together with a human-readable reason. The transport
// layer maps this to a JSON DTO — the use case stays free of presentation
// concerns (no JSON tags, no empty-slice coercion).
type ImportReport struct {
	Imported int
	Skipped  []SkippedRow
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
	"consent": "consent", "согласие": "consent", "consent_status": "consent",
}

// consentDeclaredInCSV reports whether a CSV consent-column value declares
// obtained consent. Accepts common truthy forms (RU/EN), case-insensitive.
// Anything else (including empty) leaves the prospect at the cold 'none'
// default — consent is opt-in, never inferred from a blank cell.
func consentDeclaredInCSV(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "yes", "true", "1", "obtained", "y", "да":
		return true
	default:
		return false
	}
}

type LeadChecker interface {
	LeadExistsByEmail(ctx context.Context, userID uuid.UUID, email string) (bool, error)
}

// IdentityLinker is the narrow port prospects needs from the
// leads-context identity machinery: take a freshly persisted prospect's
// identifiers, resolve them to a unified Identity (creating one if no
// match), and link the prospect to it. The adapter in the composition
// root bridges this to leads.IdentityResolver + leads.IdentityRepository.
// LinkProspect.
//
// Implementations MUST be idempotent so a backfill re-run produces no
// duplicate link rows.
type IdentityLinker interface {
	LinkProspectToIdentity(ctx context.Context, userID, prospectID uuid.UUID, email, phone, telegramUsername string) error
}

// EnrichmentEnqueuer is the narrow port prospects needs from the enrichment
// context: enqueue a best-effort background company-data lookup for a newly
// created prospect's email. The adapter in the composition root bridges this to
// enrichment.UseCase.Enqueue. Implementations must be safe to call with a
// free/personal or empty email (a no-op); errors are logged, never fatal.
type EnrichmentEnqueuer interface {
	Enqueue(ctx context.Context, userID uuid.UUID, email string) error
}

type UseCase struct {
	repo           domain.Repository
	leadChecker    LeadChecker
	identityLinker IdentityLinker
	enricher       EnrichmentEnqueuer
	logger         *slog.Logger
}

func NewUseCase(repo domain.Repository, opts ...func(*UseCase)) *UseCase {
	uc := &UseCase{repo: repo, logger: slog.Default()}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

func WithLeadChecker(lc LeadChecker) func(*UseCase) {
	return func(uc *UseCase) { uc.leadChecker = lc }
}

// WithEnricher wires the cross-context enrichment enqueuer so a newly created
// prospect's company domain is queued for background scraping. Best-effort:
// omit it (or pass nil) to disable enrichment.
func WithEnricher(e EnrichmentEnqueuer) func(*UseCase) {
	return func(uc *UseCase) { uc.enricher = e }
}

// WithIdentityLinker wires the cross-context identity linker so each
// imported prospect gets resolved to a unified Identity and linked via
// prospect_identities. Pass nil (or omit the option) to keep the legacy
// flow.
func WithIdentityLinker(l IdentityLinker) func(*UseCase) {
	return func(uc *UseCase) { uc.identityLinker = l }
}

// WithLogger overrides the default slog.Logger so the use case routes
// structured warnings through the server-wide handler. Pass nil to keep
// slog.Default().
func WithLogger(l *slog.Logger) func(*UseCase) {
	return func(uc *UseCase) {
		if l != nil {
			uc.logger = l
		}
	}
}

// Use-case-level errors for the consent toggle.
var (
	// ErrProspectNotFound is returned when a prospect does not exist or is not
	// owned by the requesting user (kept indistinguishable to avoid leaking
	// cross-tenant existence).
	ErrProspectNotFound = errors.New("prospect not found")
	// ErrUnsupportedConsentStatus is returned when the manual toggle is asked
	// to set a status other than obtained/withdrawn ('none' is not an operator
	// action — a prospect starts cold and only the system clears consent).
	ErrUnsupportedConsentStatus = errors.New("consent status not supported via manual toggle")
)

// SetConsent applies an operator's manual consent decision to a prospect:
// obtained (grant) or withdrawn. Ownership is enforced via GetProspectForUser;
// the change is recorded with source "manual" and persisted.
func (uc *UseCase) SetConsent(ctx context.Context, userID, prospectID uuid.UUID, status domain.ConsentStatus) error {
	p, err := uc.repo.GetProspectForUser(ctx, userID, prospectID)
	if err != nil {
		return err
	}
	if p == nil {
		return ErrProspectNotFound
	}
	now := time.Now().UTC()
	switch status {
	case domain.ConsentStatusObtained:
		err = p.GrantConsent(domain.ConsentSourceManual, now)
	case domain.ConsentStatusWithdrawn:
		err = p.WithdrawConsent(domain.ConsentSourceManual, now)
	default:
		return ErrUnsupportedConsentStatus
	}
	if err != nil {
		return err
	}
	return uc.repo.UpdateConsent(ctx, p.ID, p.Consent)
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
	uc.enqueueEnrichment(ctx, p.UserID, p.Email)
	return p, nil
}

// enqueueEnrichment fires a best-effort background company-data lookup. Any
// failure is logged, never propagated — enrichment must not fail a create.
func (uc *UseCase) enqueueEnrichment(ctx context.Context, userID uuid.UUID, email string) {
	if uc.enricher == nil || email == "" {
		return
	}
	if err := uc.enricher.Enqueue(ctx, userID, email); err != nil {
		uc.logger.WarnContext(ctx, "prospects: enrichment enqueue failed", "err", err)
	}
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
		// Optional declared consent: only an explicit truthy cell opts the
		// prospect in (source "import"); a blank cell stays at 'none'.
		if consentDeclaredInCSV(getCol(record, "consent")) {
			if err := p.GrantConsent(domain.ConsentSourceImport, time.Now().UTC()); err != nil {
				skipped = append(skipped, SkippedRow{Line: lineNum, Reason: err.Error()})
				continue
			}
		}
		prospects = append(prospects, *p)
	}

	if err := uc.repo.CreateProspectsBatch(ctx, prospects); err != nil {
		return nil, err
	}

	if uc.identityLinker != nil {
		for _, p := range prospects {
			if err := uc.identityLinker.LinkProspectToIdentity(ctx, userID, p.ID, p.Email, p.Phone, p.TelegramUsername); err != nil {
				uc.logger.WarnContext(ctx, "prospects: identity link failed",
					"prospect", p.ID, "err", err)
			}
		}
	}
	for _, p := range prospects {
		uc.enqueueEnrichment(ctx, userID, p.Email)
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
	_ = w.Write([]string{"name", "company", "title", "email", "phone", "whatsapp", "telegram_username", "industry", "company_size", "context", "consent"})
	_ = w.Write([]string{"Иван Петров", "ООО Рога и Копыта", "Менеджер", "ivan@example.com", "+79991234567", "", "ivan_petrov", "IT", "10-50", "Заинтересован в интеграции", "yes"})
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
	header := []string{"name", "company", "title", "email", "phone", "whatsapp", "telegram_username", "industry", "company_size", "context", "consent_status", "consent_source", "source", "status"}
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
			p.Consent.Status.String(),
			p.Consent.Source,
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
