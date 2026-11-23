# Go のコードの書き方

`backend/` の Go のコードに適用する。lint (`backend/.golangci.yaml`) で拾えない書き方を決める。

## パッケージの分け方

- 外部のサービスに関わるものは、サービスごとの専用のパッケージに切り出す (例: `internal/slack`、`internal/dynamodb`)
  - 外部の API のクライアントと、その API に渡す本文の組み立ては、同じパッケージに置く
  - 処理の流れを持つパッケージ (例: `internal/notifier`) には、外部のサービスの SDK を直接書かない
- どこからも使う値の型 (ドメインの型) は、外部のサービスに依存しないパッケージに置く (例: `internal/deploy`)
- AWS の SDK と同じ名前のパッケージを読み込むときは、SDK の側に別名を付ける (例: `ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"`)
- テストで複数のパッケージから使う値と関数は、`<パッケージ>test` のパッケージに置く (例: `internal/deploy/deploytest`)

## 分岐

- `if` / `else` は使わない。条件を反転して、早期リターンにする

  ```go
  // NG
  if p.Rev <= 1 {
      in.ConditionExpression = aws.String("attribute_not_exists(#pk)")
  } else {
      in.ConditionExpression = aws.String("#rev = :prev")
  }

  // OK
  if p.Rev <= 1 {
      in.ConditionExpression = aws.String("attribute_not_exists(#pk)")
      return in
  }
  in.ConditionExpression = aws.String("#rev = :prev")
  return in
  ```

- `switch` の `case` の中には処理を長く書かない。関数かメソッドに切り出し、`case` ではそれを呼ぶだけにする
- 値を対応させるだけの `switch` は、`map` にする

## 戻り値

- 構造体はポインタで返す。エラーのときは、ゼロ値の構造体ではなく `nil` を返す

  ```go
  // NG
  return deploy.Parent{}, fmt.Errorf("parent %s: completed_at: %w", deploymentID, err)

  // OK
  return nil, fmt.Errorf("parent %s: completed_at: %w", deploymentID, err)
  ```

- 「無い」ことは `(値, bool, error)` の `bool` ではなく、`nil` で表す

## コメント

- 構造体のフィールドのコメントは、フィールドの行の末尾に書く (上の行には書かない)

  ```go
  type Notifier struct {
      Slack   Slack  // Slack の Web API
      Channel string // 投稿するチャンネルの ID
  }
  ```

- インターフェースのメソッドのコメントは、メソッドの上の行に書き、メソッドどうしの間を 1 行空ける

  ```go
  type Store interface {
      // Claim は、手順に印を付ける。印は期限の間だけ有効
      Claim(ctx context.Context, key string, now time.Time) (dynamodb.ClaimResult, error)

      // Done は、手順が済んだ印を付ける
      Done(ctx context.Context, key string, now time.Time) error
  }
  ```

## テスト

- テーブル駆動のテストは、`map[string]struct{...}` にし、テストケースの名前をキーにする
- テストケースの名前は日本語で書く

  ```go
  tests := map[string]struct {
      rev     int
      wantErr error
  }{
      "先に書き換えられていたら ErrConflict を返す": {
          rev:     3,
          wantErr: ErrConflict,
      },
  }

  for name, tt := range tests {
      t.Run(name, func(t *testing.T) {
          // ...
      })
  }
  ```
