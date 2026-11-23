# Go のコードの書き方

`backend/` の Go のコードに適用する。lint で拾えない書き方を決める。

## 検査

- `just backend::check` で、golangci-lint (`backend/.golangci.yaml`)、gopls の解析 (`backend/scripts/gopls-check.sh`)、テストを流す。CI (`ci-backend`) も同じものを流す
- gopls の解析は、golangci-lint に入っていないもの (`writestring` など) を拾うために流す。エディターに出る警告は、CI でも失敗になる

## パッケージの分け方

- 外部のサービスに関わるものは、サービスごとの専用のパッケージに切り出す (例: `internal/slack`、`internal/dynamodb`)
  - 外部の API のクライアントと、その API に渡す本文の組み立ては、同じパッケージに置く
  - 処理の流れを持つパッケージ (例: `internal/notifier`) には、外部のサービスの SDK を直接書かない
- どこからも使う値の型 (ドメインの型) は、外部のサービスに依存しないパッケージに置く (例: `internal/deploy`)
- AWS の SDK と同じ名前のパッケージを読み込むときは、SDK の側に別名を付ける (例: `ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"`)

## ファイルの分け方

- 1 つのファイルには、1 つの役割だけを置く。型と、その型を扱う関数は同じファイルに置く
  - 例: `internal/deploy` は `kind.go` (通知の種類)、`event.go` (読み取りの入口)、`started.go` / `ecs.go` (出どころごとの読み取り)、`status.go`、`parent.go`
- 外部のサービスを呼ぶ型 (例: `dynamodb.Store`) は、メソッドごとに 1 ファイルにする。ファイル名はメソッド名を snake_case にしたもの (`Claim` は `claim.go`、`GetParent` は `get_parent.go`)
  - 型の定義とコンストラクターは `store.go` のように、型の名前のファイルに置く
  - 複数のメソッドで使うもの (キーの組み立て、属性の名前、値の変換) は、役割の分かる名前のファイルにまとめる (例: `attribute.go`、`parent_item.go`)
- パッケージの説明 (`// Package ...`) は `doc.go` に書く
- インターフェースの定義は、パッケージごとに `interface.go` にまとめる
- テストのファイルも、テストする対象のファイルに合わせて分ける (`claim.go` なら `claim_test.go`)。テストで使う差し替えは `fake_test.go` に置く
- import は、標準ライブラリ、外部のパッケージ、このリポジトリのパッケージの 3 つのまとまりに分ける (`golangci-lint fmt` が `goimports` の `local-prefixes` でそろえる)

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
- エラーを返すときは、ほかの戻り値に値を入れない (`return nil, err`)。テストの差し替え (`fake_test.go`) も同じにする

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

- **単体テスト**は、それぞれのパッケージに置き、そのパッケージで定義した関数とメソッドだけを確かめる
  - 入力は、そのテストの中で組み立てる。ほかのパッケージの型 (例: `deploy.Event`) は、値をそのまま書いて渡す。ほかのパッケージの処理 (イベントの読み取りなど) を通して作らない
  - 処理の流れを持つパッケージ (例: `notifier`) の単体テストでは、流れ (呼ぶ回数、順番、再試行での重複) だけを確かめる。ほかのパッケージが作る中身 (Slack の本文の見出しや色) は確かめない
  - テストで使う値、組み立ての関数、差し替えは、そのパッケージの `_test.go` (`fixture_test.go`、`helper_test.go`、`fake_test.go`) に置く
- **結合テスト**は、複数のパッケージをまたいで通しで確かめるテストで、`internal/integration` に置く
  - 例: EventBridge の生のイベントを `notifier.Handle` に渡し、Slack に届く本文までを確かめる
  - テスト用のパッケージ (`xxxtest` など) は作らない。結合テストで使うものは、`internal/integration` の `_test.go` に置く

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
