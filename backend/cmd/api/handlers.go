package main

import (
	"log/slog"
	"net/http"

	"github.com/tamaco489/amber/backend/internal/checkout"
	"github.com/tamaco489/amber/backend/internal/failmode"
	"github.com/tamaco489/amber/backend/internal/health"
	"github.com/tamaco489/amber/backend/internal/server"
)

// newHandlers は、ルートに並べるハンドラーを、版の名前と動きから組み立てる
//   - -fail-health の版は、/health だけを 500 を返すものに差し替える。ほかのパスは、ふつうに返す
//   - -fail-500 の版は、/ と注文の確定を 500 を返すものに差し替える。/health は、ふつうに 200 を返す
func newHandlers(version string, mode failmode.Mode, logger *slog.Logger) *server.Handlers {
	h := &server.Handlers{
		Health:   health.Check,
		Version:  health.Version(version),
		Checkout: checkout.Handle(version),
	}
	switch mode {
	case failmode.Health:
		h.Health = health.FailingCheck(version, logger)
	case failmode.Fail500:
		h.Version = fail500(version, logger)
		h.Checkout = fail500(version, logger)
	}
	return h
}

// fail500 は、どのリクエストにも 500 を返し続ける
//   - 版の名前が -fail-500 で終わるときに、/ と注文の確定の代わりに使う (internal/failmode)
//   - 起動もヘルスチェックも通るが、実際のリクエストで 5xx を返す版を作り、ターゲットの 5xx のアラームで元の版に戻すかを確かめるため
//   - アクセスログのミドルウェア (internal/server) の内側に置かれるため、アクセスログにも status 500 が出る
//   - アプリの不具合ではなく、わざと返したと分かるように、返すたびにアクセスログとは別に ERROR のログを 1 行出す
func fail500(version string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Error("returning 500 on purpose",
			"mode", failmode.Fail500,
			"method", r.Method,
			"path", r.URL.Path,
			"version", version,
		)
		http.Error(w, "fail-500: this version returns 500 on purpose", http.StatusInternalServerError)
	}
}
