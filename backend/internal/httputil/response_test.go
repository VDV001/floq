package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	t.Run("sets content type and status", func(t *testing.T) {
		w := httptest.NewRecorder()
		WriteJSON(w, http.StatusCreated, map[string]string{"ok": "true"})

		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if w.Code != http.StatusCreated {
			t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
		}

		var body map[string]string
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["ok"] != "true" {
			t.Errorf("body[ok] = %q, want true", body["ok"])
		}
	})

	t.Run("encodes structs", func(t *testing.T) {
		type payload struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		w := httptest.NewRecorder()
		WriteJSON(w, http.StatusOK, payload{Name: "Alice", Age: 30})

		var body map[string]any
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["name"] != "Alice" {
			t.Errorf("name = %v, want Alice", body["name"])
		}
	})
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusBadRequest, "invalid input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "invalid input" {
		t.Errorf("error = %q, want %q", body["error"], "invalid input")
	}
}

func TestWriteErrorDetail(t *testing.T) {
	w := httptest.NewRecorder()
	WriteErrorDetail(w, http.StatusBadRequest, "ИИ не подключён", "ai_not_configured", "Откройте Настройки → ИИ")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "ИИ не подключён" {
		t.Errorf("error = %q", body["error"])
	}
	if body["code"] != "ai_not_configured" {
		t.Errorf("code = %q", body["code"])
	}
	if body["remedy"] != "Откройте Настройки → ИИ" {
		t.Errorf("remedy = %q", body["remedy"])
	}
}
