// api は、ECS のデプロイ戦略を確かめるための検証用アプリ
//   - /health、/: どの版が動いているかを確かめる (internal/health)
//   - /v1/carts/{cart_id}/checkout: 注文を確定する (internal/checkout)
//   - ルーティングとアクセスログは internal/server。ここでは起動と終了、ハンドラーの組み立て (handlers.go) だけを持つ
//   - 版の名前が -fail-exit / -fail-health / -fail-500 で終わるときは、わざと失敗する (internal/failmode)
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tamaco489/amber/backend/internal/failmode"
	"github.com/tamaco489/amber/backend/internal/server"
)

// version は、ビルド時に -ldflags "-X main.version=..." で埋め込む
var version = "dev"

const (
	addr = ":8080"

	// SIGTERM を受けてから、処理中のリクエストを待つ上限
	//   - ECS の stopTimeout (既定 30 秒) より短くし、SIGKILL より先に終えられるようにする
	shutdownTimeout = 20 * time.Second
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// -fail-exit の版は、HTTP サーバーを立てる前に終わる (起動の途中で落ちるアプリの真似)
	//   - タスクが RUNNING のまま続かないため、デプロイのサーキットブレーカーが失敗と数える
	mode := failmode.FromVersion(version)
	if mode == failmode.Exit {
		slog.Error("exiting on purpose before starting the server", "version", version, "mode", mode)
		os.Exit(1)
	}

	if err := run(mode); err != nil {
		slog.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(mode failmode.Mode) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	mux := server.NewMux(newHandlers(version, mode, slog.Default()))
	srv := &http.Server{
		Addr:              addr,
		Handler:           server.WithAccessLog(mux, version),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server started", "addr", addr, "version", version, "mode", mode)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	// 新しい接続の受け付けをやめ、処理中のリクエストが終わるのを待つ
	slog.Info("shutting down", "timeout", shutdownTimeout.String())
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	slog.Info("server stopped")
	return nil
}
