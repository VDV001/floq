package httputil

import (
	"encoding/json"
	"net/http"
)

// WriteJSON encodes v as JSON and writes it to w with the given HTTP status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// WriteError writes a JSON error response: {"error": msg}.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

// WriteErrorDetail writes a JSON error response carrying a machine-readable
// code and a human "what to do" remedy alongside the message:
// {"error": msg, "code": code, "remedy": remedy}. The frontend surfaces the
// remedy to the user and uses the code to offer a direct fix (e.g. a link to
// Settings). Empty code/remedy are omitted.
func WriteErrorDetail(w http.ResponseWriter, status int, msg, code, remedy string) {
	body := map[string]string{"error": msg}
	if code != "" {
		body["code"] = code
	}
	if remedy != "" {
		body["remedy"] = remedy
	}
	WriteJSON(w, status, body)
}
