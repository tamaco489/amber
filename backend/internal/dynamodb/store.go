package dynamodb

import "time"

// Store は、親の投稿と手順の記録を、DynamoDB のテーブルに残す
type Store struct {
	client API
	table  string
	lease  time.Duration // 手順の印の期限。Lambda のタイムアウトより長く、非同期呼び出しの再試行 (1 分ほどあと) で切れているくらいにする
}

// New は、テーブル名 table の Store を作る
func New(client API, table string) *Store {
	return &Store{client: client, table: table, lease: DefaultLeaseTime}
}
