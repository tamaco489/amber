package main

import (
	"github.com/tamaco489/amber/backend/internal/checkout"
	"github.com/tamaco489/amber/backend/internal/failmode"
	"github.com/tamaco489/amber/backend/internal/health"
	"github.com/tamaco489/amber/backend/internal/server"
)

// newHandlers は、ルートに並べるハンドラーを、版の名前と動きから組み立てる
//   - -fail-health の版は、/health だけを 500 を返すものに差し替える。ほかのパスは、ふつうに返す
func newHandlers(version string, mode failmode.Mode) *server.Handlers {
	h := &server.Handlers{
		Health:   health.Check,
		Version:  health.Version(version),
		Checkout: checkout.Handle(version),
	}
	if mode == failmode.Health {
		h.Health = health.FailingCheck
	}
	return h
}
