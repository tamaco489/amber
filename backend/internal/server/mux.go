package server

import (
	"net/http"
)

// Handlers は、ルートに並べるハンドラー
type Handlers struct {
	Health   http.HandlerFunc // GET /health (ALB のヘルスチェック)
	Version  http.HandlerFunc // GET / (バージョン番号を返す)
	Checkout http.HandlerFunc // POST /v1/carts/{cart_id}/checkout (注文の確定)
}

// NewMux は、ハンドラーをパスに並べる
func NewMux(h *Handlers) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /{$}", h.Version)
	mux.HandleFunc("POST /v1/carts/{cart_id}/checkout", h.Checkout)
	return mux
}
