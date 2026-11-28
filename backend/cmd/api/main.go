// api は、ECS のデプロイ戦略を確かめるための検証用アプリ
//   - /health: ALB のヘルスチェック用
//   - /: バージョン番号を返す (どちらの版に振り分けられたかを確かめるため)
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

	if err := run(); err != nil {
		slog.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	srv := &http.Server{
		Addr:              addr,
		Handler:           withAccessLog(newMux(version), version),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server started", "addr", addr, "version", version)
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
