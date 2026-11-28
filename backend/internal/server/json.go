package server

import (
	"encoding/json"
	"net/http"
)

// WriteJSON は、ステータスと JSON の本文を返す
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
