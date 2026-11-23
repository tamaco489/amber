package notifier

import (
	"context"
	"errors"
	"time"
)

// Status は、親の投稿が表す、デプロイの今の状態
type Status string

// デプロイの状態
const (
	StatusDeploying   Status = "deploying"
	StatusRollingBack Status = "rolling_back"
	StatusCompleted   Status = "completed"
	StatusFailed      Status = "failed"
)

// rank は、状態の進み具合
//   - 親の状態は、進む方向にだけ変える。遅れて届いたイベントで、完了や失敗が「デプロイ中」に戻らないようにする
//   - 失敗は完了より強くする (ロールバックが完了しても、失敗したことを残すため)
func (s Status) rank() int {
	switch s {
	case StatusDeploying:
		return 0
	case StatusRollingBack:
		return 1
	case StatusCompleted:
		return 2
	case StatusFailed:
		return 3
	}
	return -1
}

// Parent は、親の投稿と、それを組み立て直すのに要る値
//   - chat.update は本文をまるごと置き換えるため、書き換えるたびに、ここから組み立て直す
type Parent struct {
	DeploymentID   string
	TS             string
	Channel        string
	Service        string
	Status         Status
	Version        string
	Actor          string
	RunURL         string
	TaskDefinition string
	CompletedAt    time.Time
	Rev            int
}

// ClaimResult は、手順に印を付けようとした結果
type ClaimResult int

// 手順に印を付けようとした結果
const (
	// ClaimAcquired: 印を付けた。この呼び出しが手順を進める
	ClaimAcquired ClaimResult = iota + 1
	// ClaimDone: すでに済んでいる。手順を飛ばす
	ClaimDone
	// ClaimBusy: ほかの呼び出しが進めている
	ClaimBusy
)

// ErrConflict は、親の投稿を書き換えようとしたときに、ほかの呼び出しが先に書き換えていたこと
var ErrConflict = errors.New("parent was updated concurrently")

// Store は、親の投稿と、手順の記録を残す場所 (DynamoDB)
type Store interface {
	// Claim は、手順に印を付ける。印は lease の間だけ有効で、期限が切れたら、ほかの呼び出しが付け直せる
	Claim(ctx context.Context, key string, now time.Time) (ClaimResult, error)
	// Done は、手順が済んだ印を付ける
	Done(ctx context.Context, key string, now time.Time) error
	// Release は、手順の印を外す (Slack の呼び出しに失敗したとき、再試行ですぐに進め直せるようにする)
	Release(ctx context.Context, key string) error

	// GetParent は、親の投稿を読む。無ければ false を返す
	GetParent(ctx context.Context, deploymentID string) (Parent, bool, error)
	// PutParent は、親の投稿を書く。p.Rev が 1 なら新しく作り、それ以外は rev が p.Rev - 1 のときだけ書く (違えば ErrConflict)
	PutParent(ctx context.Context, p Parent, now time.Time) error
}
