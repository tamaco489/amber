package main

import (
	"encoding/json"
	"net/http"
)

// writeJSON は、ステータスと JSON の本文を返す
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
