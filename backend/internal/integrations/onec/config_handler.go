package onec

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/integrations/onec/domain"
	"github.com/go-chi/chi/v5"
)

// ConfigHandler exposes the settings-UI surface for the 1C integration (#110).
// Thin: it parses, calls the usecase, and maps results/errors to HTTP — no
// business logic, no persistence, no secret handling beyond what the usecase
// already masked.
type ConfigHandler struct {
	uc *ConfigUseCase
}

// NewConfigHandler builds the config handler over its usecase.
func NewConfigHandler(uc *ConfigUseCase) *ConfigHandler {
	return &ConfigHandler{uc: uc}
}

// RegisterConfigRoutes mounts the 1C config endpoints. All require auth — the
// caller installs this inside the JWT-protected group (the public webhook
// RegisterRoutes is separate).
func RegisterConfigRoutes(r chi.Router, h *ConfigHandler) {
	r.Get("/api/onec/config", h.getConfig)
	r.Put("/api/onec/config", h.updateConfig)
	r.Post("/api/onec/config/regenerate-webhook", h.regenerateWebhook)
	r.Post("/api/onec/test", h.testConnection)
	r.Get("/api/onec/mapping", h.getMapping)
	r.Put("/api/onec/mapping", h.updateMapping)
}

// --- wire DTOs ---

type configWire struct {
	BaseURL       string `json:"base_url"`
	AuthType      string `json:"auth_type"`
	AuthSecret    string `json:"auth_secret"`
	WebhookSecret string `json:"webhook_secret"`
	IsActive      bool   `json:"is_active"`
}

type configUpdateWire struct {
	BaseURL    *string `json:"base_url"`
	AuthType   *string `json:"auth_type"`
	AuthSecret *string `json:"auth_secret"`
	IsActive   *bool   `json:"is_active"`
}

type configTestWire struct {
	BaseURL    *string `json:"base_url"`
	AuthType   *string `json:"auth_type"`
	AuthSecret *string `json:"auth_secret"`
}

type testResultWire struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type mappingRuleWire struct {
	ExternalType string `json:"external_type"`
	Kind         string `json:"kind"`
	EmailField   string `json:"email_field"`
	NameField    string `json:"name_field,omitempty"`
	CompanyField string `json:"company_field,omitempty"`
}

type mappingWire struct {
	Rules []mappingRuleWire `json:"rules"`
}

func toConfigWire(v *ConfigView) configWire {
	return configWire{
		BaseURL:       v.BaseURL,
		AuthType:      v.AuthType,
		AuthSecret:    v.AuthSecret,
		WebhookSecret: v.WebhookSecret,
		IsActive:      v.IsActive,
	}
}

// --- handlers ---

func (h *ConfigHandler) getConfig(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	v, err := h.uc.GetConfig(r.Context(), userID)
	if err != nil {
		status, msg := configErrorToHTTP(err)
		httputil.WriteError(w, status, msg)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, toConfigWire(v))
}

func (h *ConfigHandler) updateConfig(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var in configUpdateWire
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	v, err := h.uc.UpdateConfig(r.Context(), userID, ConfigUpdate{
		BaseURL:    in.BaseURL,
		AuthType:   in.AuthType,
		AuthSecret: in.AuthSecret,
		IsActive:   in.IsActive,
	})
	if err != nil {
		status, msg := configErrorToHTTP(err)
		httputil.WriteError(w, status, msg)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, toConfigWire(v))
}

func (h *ConfigHandler) regenerateWebhook(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	secret, err := h.uc.RegenerateWebhook(r.Context(), userID)
	if err != nil {
		status, msg := configErrorToHTTP(err)
		httputil.WriteError(w, status, msg)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]string{"webhook_secret": secret})
}

func (h *ConfigHandler) testConnection(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var in configTestWire
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	err := h.uc.TestConnection(r.Context(), userID, ConfigTestOverride{
		BaseURL:    in.BaseURL,
		AuthType:   in.AuthType,
		AuthSecret: in.AuthSecret,
	})
	if err != nil {
		// A failed connection test is a 200 with success:false (mirrors the
		// settings testers) — it's a probe result, not an HTTP-level error.
		httputil.WriteJSON(w, http.StatusOK, testResultWire{Success: false, Error: onecTestErrorToUserMessage(err)})
		return
	}
	httputil.WriteJSON(w, http.StatusOK, testResultWire{Success: true})
}

func (h *ConfigHandler) getMapping(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	rules, err := h.uc.GetMapping(r.Context(), userID)
	if err != nil {
		status, msg := configErrorToHTTP(err)
		httputil.WriteError(w, status, msg)
		return
	}
	out := mappingWire{Rules: make([]mappingRuleWire, len(rules))}
	for i, ru := range rules {
		out.Rules[i] = mappingRuleWire{
			ExternalType: ru.ExternalType,
			Kind:         ru.Kind,
			EmailField:   ru.EmailField,
			NameField:    ru.NameField,
			CompanyField: ru.CompanyField,
		}
	}
	httputil.WriteJSON(w, http.StatusOK, out)
}

func (h *ConfigHandler) updateMapping(w http.ResponseWriter, r *http.Request) {
	userID, ok := httputil.UserIDFromContext(r.Context())
	if !ok {
		httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var in mappingWire
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	inputs := make([]MappingRuleInput, len(in.Rules))
	for i, ru := range in.Rules {
		inputs[i] = MappingRuleInput{
			ExternalType: ru.ExternalType,
			Kind:         ru.Kind,
			EmailField:   ru.EmailField,
			NameField:    ru.NameField,
			CompanyField: ru.CompanyField,
		}
	}
	if err := h.uc.UpdateMapping(r.Context(), userID, inputs); err != nil {
		status, msg := configErrorToHTTP(err)
		httputil.WriteError(w, status, msg)
		return
	}
	httputil.WriteJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

// configErrorToHTTP maps a usecase/domain error to an HTTP status and message.
// Validation/invariant errors are client errors (400); anything else is 500.
func configErrorToHTTP(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrActiveRequiresBaseURL):
		return http.StatusBadRequest, "Нельзя включить интеграцию без адреса 1С."
	case errors.Is(err, domain.ErrInvalidAuthType):
		return http.StatusBadRequest, "Некорректный тип авторизации (basic или token)."
	case errors.Is(err, domain.ErrInvalidWebhookSecretFormat):
		return http.StatusBadRequest, "Некорректный формат webhook-секрета."
	case errors.Is(err, domain.ErrNoRules):
		return http.StatusBadRequest, "Добавьте хотя бы одно правило маппинга."
	case errors.Is(err, domain.ErrEmptyExternalType):
		return http.StatusBadRequest, "У каждого правила должен быть тип объекта 1С."
	case errors.Is(err, domain.ErrInvalidEventKind):
		return http.StatusBadRequest, "Недопустимый тип события в правиле маппинга."
	case errors.Is(err, domain.ErrDuplicateExternalType):
		return http.StatusBadRequest, "Типы объектов 1С в правилах не должны повторяться."
	default:
		return http.StatusInternalServerError, "Не удалось выполнить операцию."
	}
}

// onecTestErrorToUserMessage maps a connection-test failure to a Russian
// operator-facing string.
func onecTestErrorToUserMessage(err error) string {
	switch {
	case errors.Is(err, ErrOnecAuth):
		return "1С отклонила учётные данные — проверьте логин/пароль или токен."
	case errors.Is(err, ErrOnecUnreachable):
		return "Не удалось подключиться к 1С — проверьте адрес и доступность сервера."
	case errors.Is(err, ErrOnecBadResponse):
		return "1С ответила неожиданно — проверьте адрес OData-сервиса."
	case errors.Is(err, domain.ErrEmptyBaseURL):
		return "Укажите адрес 1С."
	case errors.Is(err, domain.ErrInvalidAuthType):
		return "Некорректный тип авторизации (basic или token)."
	default:
		return "Не удалось проверить соединение с 1С."
	}
}
