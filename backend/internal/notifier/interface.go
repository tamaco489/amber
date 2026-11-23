package notifier

import (
	"context"
	"time"

	"github.com/tamaco489/amber/backend/internal/deploy"
	"github.com/tamaco489/amber/backend/internal/dynamodb"
	"github.com/tamaco489/amber/backend/internal/slack"
)

// Slack は、Notifier が使う Slack の Web API (テストで差し替える)
type Slack interface {
	// PostMessage は、メッセージを投稿し、その ts を返す
	PostMessage(ctx context.Context, msg slack.Message) (string, error)

	// UpdateMessage は、投稿を書き換える
	UpdateMessage(ctx context.Context, msg slack.Message) error

	// AddReaction は、投稿にスタンプを付ける
	AddReaction(ctx context.Context, channel, ts, name string) error
}

// Store は、親の投稿と、手順の記録を残す場所 (実装は internal/dynamodb。テストで差し替える)
type Store interface {
	// Claim は、手順に印を付ける。印は期限の間だけ有効
	Claim(ctx context.Context, key string, now time.Time) (dynamodb.ClaimResult, error)

	// Done は、手順が済んだ印を付ける
	Done(ctx context.Context, key string, now time.Time) error

	// Release は、手順の印を外す (失敗したとき、再試行ですぐに進め直せるようにする)
	Release(ctx context.Context, key string) error

	// GetParent は、親の投稿を読む。無ければ nil を返す
	GetParent(ctx context.Context, deploymentID string) (*deploy.Parent, error)

	// PutParent は、親の投稿を書く。先に書き換えられていたら dynamodb.ErrConflict を返す
	PutParent(ctx context.Context, p *deploy.Parent, now time.Time) error
}
