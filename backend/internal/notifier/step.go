package notifier

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/tamaco489/amber/backend/internal/dynamodb"
)

// ErrBusy は、同じ手順をほかの呼び出しが進めていること
//   - エラーとして返し、Lambda の再試行で、済んだかを確かめ直す
var ErrBusy = errors.New("step is in progress in another invocation")

// step は、手順に印を付けてから fn を実行し、済んだ印を付ける
//   - 済んでいれば fn を実行しない (同じイベントの再配送や、途中まで成功したあとの再試行で、同じ投稿をしない)
//   - fn が失敗したら印を外し、次の再試行ですぐに進め直せるようにする
func (n *Notifier) step(ctx context.Context, key string, fn func() error) error {
	res, err := n.Store.Claim(ctx, key, n.now())
	if err != nil {
		return err
	}
	if res == dynamodb.ClaimDone {
		return nil
	}
	if res == dynamodb.ClaimBusy {
		return fmt.Errorf("%s: %w", key, ErrBusy)
	}

	if err := fn(); err != nil {
		n.release(ctx, key)
		return fmt.Errorf("%s: %w", key, err)
	}
	return n.Store.Done(ctx, key, n.now())
}

// release は、手順の印を外す。外せなくても、印の期限が切れれば進め直せるため、ログに残すだけにする
func (n *Notifier) release(ctx context.Context, key string) {
	if err := n.Store.Release(ctx, key); err != nil {
		slog.WarnContext(ctx, "failed to release step", "key", key, "error", err)
	}
}
