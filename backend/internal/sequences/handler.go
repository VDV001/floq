package sequences

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func getUserID(r *http.Request) uuid.UUID {
	uid, _ := r.Context().Value("user_id").(uuid.UUID)
	return uid
}

func RegisterRoutes(router chi.Router, uc *UseCase) {
	router.Get("/api/sequences", listSequences(uc))
	router.Post("/api/sequences", createSequence(uc))
	router.Get("/api/sequences/{id}", getSequence(uc))
	router.Put("/api/sequences/{id}", updateSequence(uc))
	router.Delete("/api/sequences/{id}", deleteSequence(uc))
	router.Post("/api/sequences/{id}/steps", addStep(uc))
	router.Post("/api/sequences/{id}/launch", launchSequence(uc))
	router.Patch("/api/sequences/{id}/toggle", toggleActive(uc))

	router.Get("/api/outbound/queue", getQueue(uc))
	router.Post("/api/outbound/{id}/approve", approveMessage(uc))
	router.Post("/api/outbound/{id}/reject", rejectMessage(uc))
	router.Post("/api/outbound/{id}/edit", editMessage(uc))
	router.Get("/api/outbound/stats", getStats(uc))
}

func parseIDParam(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "id"))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func listSequences(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		seqs, err := uc.ListSequences(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list sequences")
			return
		}
		if seqs == nil {
			seqs = []Sequence{}
		}
		writeJSON(w, http.StatusOK, seqs)
	}
}

func createSequence(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}

		s := &Sequence{
			ID:        uuid.New(),
			UserID:    getUserID(r),
			Name:      body.Name,
			IsActive:  false,
			CreatedAt: time.Now().UTC(),
		}
		if err := uc.CreateSequence(r.Context(), s); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create sequence")
			return
		}
		writeJSON(w, http.StatusCreated, s)
	}
}

func getSequence(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid sequence id")
			return
		}
		seq, err := uc.GetSequence(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get sequence")
			return
		}
		if seq == nil {
			writeError(w, http.StatusNotFound, "sequence not found")
			return
		}

		steps, err := uc.ListSteps(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list steps")
			return
		}
		if steps == nil {
			steps = []SequenceStep{}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"sequence": seq,
			"steps":    steps,
		})
	}
}

func updateSequence(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid sequence id")
			return
		}

		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}

		s := &Sequence{ID: id, Name: body.Name}
		if err := uc.UpdateSequence(r.Context(), s); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update sequence")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"name": body.Name})
	}
}

func deleteSequence(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid sequence id")
			return
		}
		if err := uc.DeleteSequence(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete sequence")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

func addStep(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid sequence id")
			return
		}

		var body struct {
			StepOrder  int    `json:"step_order"`
			DelayDays  int    `json:"delay_days"`
			PromptHint string `json:"prompt_hint"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		step := &SequenceStep{
			ID:         uuid.New(),
			SequenceID: id,
			StepOrder:  body.StepOrder,
			DelayDays:  body.DelayDays,
			PromptHint: body.PromptHint,
			CreatedAt:  time.Now().UTC(),
		}
		if err := uc.CreateStep(r.Context(), step); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create step")
			return
		}
		writeJSON(w, http.StatusCreated, step)
	}
}

func launchSequence(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid sequence id")
			return
		}

		var body struct {
			ProspectIDs []uuid.UUID `json:"prospect_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if len(body.ProspectIDs) == 0 {
			writeError(w, http.StatusBadRequest, "prospect_ids is required")
			return
		}

		if err := uc.Launch(r.Context(), id, body.ProspectIDs); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to launch sequence")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "launched"})
	}
}

func toggleActive(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid sequence id")
			return
		}

		var body struct {
			IsActive bool `json:"is_active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := uc.ToggleActive(r.Context(), id, body.IsActive); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to toggle sequence")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"is_active": body.IsActive})
	}
}

func getQueue(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		msgs, err := uc.GetQueue(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get queue")
			return
		}
		if msgs == nil {
			msgs = []OutboundMessage{}
		}
		writeJSON(w, http.StatusOK, msgs)
	}
}

func approveMessage(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid message id")
			return
		}
		if err := uc.ApproveMessage(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to approve message")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
	}
}

func rejectMessage(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid message id")
			return
		}
		if err := uc.RejectMessage(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to reject message")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
	}
}

func editMessage(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseIDParam(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid message id")
			return
		}

		var body struct {
			Body string `json:"body"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if body.Body == "" {
			writeError(w, http.StatusBadRequest, "body is required")
			return
		}

		if err := uc.EditMessage(r.Context(), id, body.Body); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to edit message")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}
}

func getStats(uc *UseCase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)
		stats, err := uc.GetStats(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get stats")
			return
		}
		writeJSON(w, http.StatusOK, stats)
	}
}
