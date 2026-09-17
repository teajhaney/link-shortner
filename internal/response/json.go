// Package response contains shared HTTP response helpers.
package response

import (
	"encoding/json"
	"net/http"
)

// WriteJSON writes payload as a JSON HTTP response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// WriteError writes a JSON error response in the API's standard shape.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}
