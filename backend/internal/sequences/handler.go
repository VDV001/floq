package sequences

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/daniil/floq/internal/httputil"
	"github.com/daniil/floq/internal/sequences/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RegisterPublicRoutes registers routes that don't require authentication (e.g. tracking pixel).
func RegisterPublicRoutes(router chi.Router, uc *UseCase) {
	router.Get("/api/track/open/{id}", trackOpen(uc))
}

func RegisterRoutes(router chi.Router, uc *UseCase) {
	router.Get("/api/sequences", listSequences(uc))
	router.Post("/api/sequences", createSequence(uc))
	router.Post("/api/sequences/preview", previewMessage(uc))
	router.Get("/api/sequences/{id}", getSequence(uc))
	router.Put("/api/sequences/{id}", updateSequence(uc))
	router.Delete("/api/sequences/{id}", deleteSequence(uc))
	router.Post("/api/sequences/{id}/steps", addStep(uc))
	router.Delete("/api/sequences/{id}/steps/{stepId}", deleteStep(uc))
	router.Post("/api/sequences/{id}/launch", launchSequence(uc))
	router.Patch("/api/sequences/{id}/toggle", toggleActive(uc))

	router.Get("/api/outbound/queue", getQueue(uc))
	router.Get("/api/outbound/sent", getSent(uc))
	router.Post("/api/outbound/{id}/approve", approveMessage(uc))
	router.Post("/api/outbound/{id}/reject", rejectMessage(uc))
	router.Post("/api/outbound/{id}/edit", editMessage(uc))
	router.Get("/api/outbound/stats", getStats(uc))
}

func listSequences(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		seqs, err := uc.ListSequences(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list sequences")
			return
		}
		if seqs == nil {
			seqs = []domain.Sequence{}
		}
		httputil.WriteJSON(w, http.StatusOK, SequencesToResponse(seqs))
	}
}

func createSequence(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Name == "" {
			httputil.WriteError(w, http.StatusBadRequest, "name is required")
			return
		}

		s := domain.NewSequence(userID, body.Name)
		if err := uc.CreateSequence(r.Context(), s); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to create sequence")
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, SequenceToResponse(s))
	}
}

func getSequence(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid sequence id")
			return
		}
		seq, err := uc.GetSequence(r.Context(), id)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to get sequence")
			return
		}
		if seq == nil {
			httputil.WriteError(w, http.StatusNotFound, "sequence not found")
			return
		}

		steps, err := uc.ListSteps(r.Context(), id)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to list steps")
			return
		}
		if steps == nil {
			steps = []domain.SequenceStep{}
		}

		httputil.WriteJSON(w, http.StatusOK, SequenceDetailResponse{
			Sequence: SequenceToResponse(seq),
			Steps:    StepsToResponse(steps),
		})
	}
}

func updateSequence(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid sequence id")
			return
		}

		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Name == "" {
			httputil.WriteError(w, http.StatusBadRequest, "name is required")
			return
		}

		if err := uc.UpdateSequence(r.Context(), id, body.Name); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to update sequence")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"name": body.Name})
	}
}

func deleteSequence(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid sequence id")
			return
		}
		if err := uc.DeleteSequence(r.Context(), id); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to delete sequence")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

func addStep(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid sequence id")
			return
		}

		var body struct {
			StepOrder  int    `json:"step_order"`
			DelayDays  int    `json:"delay_days"`
			Channel    string `json:"channel"`
			PromptHint string `json:"prompt_hint"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		channel := domain.StepChannel(body.Channel)
		if body.Channel == "" {
			channel = domain.StepChannelEmail
		}

		step := domain.NewSequenceStep(id, body.StepOrder, body.DelayDays, channel, body.PromptHint)
		if err := uc.CreateStep(r.Context(), step); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to create step")
			return
		}
		httputil.WriteJSON(w, http.StatusCreated, StepToResponse(step))
	}
}

func deleteStep(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stepID, err := uuid.Parse(chi.URLParam(r, "stepId"))
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid step id")
			return
		}
		if err := uc.DeleteStep(r.Context(), stepID); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to delete step")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

func previewMessage(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name    string `json:"name"`
			Company string `json:"company"`
			Context string `json:"context"`
			Channel string `json:"channel"`
			Hint    string `json:"hint"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Name == "" {
			httputil.WriteError(w, http.StatusBadRequest, "name is required")
			return
		}
		if body.Channel == "" {
			body.Channel = "email"
		}
		if body.Hint == "" {
			body.Hint = "первое касание"
		}

		var text string
		var err error
		switch domain.StepChannel(body.Channel) {
		case domain.StepChannelTelegram:
			text, err = uc.aiGenerator.GenerateTelegramMessage(r.Context(), body.Name, "", body.Company, body.Context, body.Hint, "", "", "")
		default:
			text, err = uc.aiGenerator.GenerateColdMessage(r.Context(), body.Name, "", body.Company, body.Context, body.Hint, "", "", "")
		}
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "generation failed: "+err.Error())
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"text": text})
	}
}

func launchSequence(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid sequence id")
			return
		}

		var body struct {
			ProspectIDs []uuid.UUID `json:"prospect_ids"`
			SendNow     bool        `json:"send_now"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if len(body.ProspectIDs) == 0 {
			httputil.WriteError(w, http.StatusBadRequest, "prospect_ids is required")
			return
		}

		if err := uc.Launch(r.Context(), id, body.ProspectIDs, body.SendNow); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to launch sequence")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "launched"})
	}
}

func toggleActive(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid sequence id")
			return
		}

		var body struct {
			IsActive bool `json:"is_active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := uc.ToggleActive(r.Context(), id, body.IsActive); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to toggle sequence")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]bool{"is_active": body.IsActive})
	}
}

func getQueue(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		msgs, err := uc.GetQueue(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to get queue")
			return
		}
		if msgs == nil {
			msgs = []domain.OutboundMessage{}
		}
		httputil.WriteJSON(w, http.StatusOK, OutboundMessagesToResponse(msgs))
	}
}

func getSent(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		msgs, err := uc.GetSent(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to get sent messages")
			return
		}
		if msgs == nil {
			msgs = []domain.OutboundMessage{}
		}
		httputil.WriteJSON(w, http.StatusOK, OutboundMessagesToResponse(msgs))
	}
}

func approveMessage(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid message id")
			return
		}
		if err := uc.ApproveMessage(r.Context(), id); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to approve message")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "approved"})
	}
}

func rejectMessage(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid message id")
			return
		}
		if err := uc.RejectMessage(r.Context(), id); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to reject message")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
	}
}

func editMessage(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := httputil.ParseIDParam(r, "id")
		if err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid message id")
			return
		}

		var body struct {
			Body string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httputil.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Body == "" {
			httputil.WriteError(w, http.StatusBadRequest, "body is required")
			return
		}

		if err := uc.EditMessage(r.Context(), id, body.Body); err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to edit message")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}
}

func trackOpen(uc *UseCase) http.HandlerFunc {
	pixel, _ := base64.StdEncoding.DecodeString("R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7")
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		msgID, err := uuid.Parse(idStr)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = uc.MarkOpened(r.Context(), msgID)
		w.Header().Set("Content-Type", "image/gif")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		_, _ = w.Write(pixel)
	}
}

func getStats(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httputil.UserIDFromContext(r.Context())
		if !ok {
			httputil.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		stats, err := uc.GetStats(r.Context(), userID)
		if err != nil {
			httputil.WriteError(w, http.StatusInternalServerError, "failed to get stats")
			return
		}
		httputil.WriteJSON(w, http.StatusOK, StatsToResponse(stats))
	}
}
