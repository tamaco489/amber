package health

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/tamaco489/amber/backend/internal/failmode"
)

// maxSleep は、/ の sleep で待たせられる上限
//   - cmd/api の shutdownTimeout より短くし、終了処理の確認で待ち切れるようにする
const maxSleep = 15 * time.Second

// Check は、ALB のヘルスチェックに ok を返す
func Check(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

// FailingCheck は、ALB のヘルスチェックに 500 を返し続ける
//   - 版の名前が -fail-health で終わるときに、Check の代わりに使う (internal/failmode)
//   - プロセスは動くがヘルスチェックに通らない版を作り、デプロイのサーキットブレーカーが元の版に戻すかを確かめるため
//   - アプリの不具合ではなく、わざと返したと分かるように、返すたびにアクセスログとは別に ERROR のログを 1 行出す
func FailingCheck(version string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger.Error("failing the health check on purpose",
			"mode", failmode.Health,
			"method", r.Method,
			"path", r.URL.Path,
			"version", version,
		)
		http.Error(w, "fail-health: this version fails the health check on purpose", http.StatusInternalServerError)
	}
}

// Version は、バージョン番号を返す
//   - ?sleep=5s のように指定すると、その時間だけ待ってから返す (SIGTERM を受けたときに、処理中のリクエストを終えてから止まるかを確かめるため)
func Version(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s := r.URL.Query().Get("sleep"); s != "" {
			d, err := time.ParseDuration(s)
			if err != nil || d < 0 || d > maxSleep {
				http.Error(w, "sleep must be a duration between 0s and "+maxSleep.String(), http.StatusBadRequest)
				return
			}
			select {
			case <-time.After(d):
			case <-r.Context().Done():
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"version": version})
	}
}
