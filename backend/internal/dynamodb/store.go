// Package dynamodb は、デプロイの通知の、親の投稿と手順の記録を DynamoDB のテーブルに残す
//
// # アイテム
//
// テーブルのパーティションキーは、文字列の pk だけ (ソートキーはない)。expires_at は TTL の属性 (Unix 秒) で、30 日後に消える。
//
// 親の投稿 (pk = "deployment#<デプロイ ID>")
//
//	ts               S  親の投稿の ts
//	channel          S  投稿したチャンネルの ID
//	service          S  サービスの名前
//	status           S  deploying / rolling_back / completed / failed
//	version          S  バージョン (Git のタグ。開始が届くまでは無い)
//	actor            S  起動した人 (同上)
//	run_url          S  GitHub Actions の実行の URL (同上)
//	task_definition  S  タスク定義の ARN (同上)
//	completed_at     S  完了した時刻 (RFC 3339)
//	rev              N  書き換えるたびに 1 ずつ増やす (楽観ロック。同時に届いたイベントが、互いの書き換えを消さないようにする)
//	expires_at       N  TTL
//
// 手順の記録 (pk = "step#<手順のキー>")
//
//	lease_until  N     処理中の印 (Unix 秒)。この時刻までは、ほかの呼び出しは同じ手順を進めない
//	done         BOOL  済んだ印。あれば、その手順は飛ばす
//	expires_at   N     TTL
//
// 手順のキーは次のとおり。
//
//	parent#<デプロイ ID>       親の投稿 (デプロイごとに 1 つ。どのイベントが作ったかによらない)
//	<イベント ID>#reply        スレッドへの返信
//	<イベント ID>#parent       親の書き換え
//	<イベント ID>#reaction     スタンプ
package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

// アイテムの置き方は、パッケージのコメントに書く
const (
	keyAttr          = "pk"
	parentKeyPrefix  = "deployment#"
	stepKeyPrefix    = "step#"
	itemTTL          = 30 * 24 * time.Hour
	DefaultLeaseTime = 60 * time.Second
)

// ClaimResult は、手順に印を付けようとした結果
type ClaimResult int

// 手順に印を付けようとした結果
const (
	ClaimAcquired ClaimResult = iota + 1 // 印を付けた。この呼び出しが手順を進める
	ClaimDone                            // すでに済んでいる。手順を飛ばす
	ClaimBusy                            // ほかの呼び出しが進めている
)

// ErrConflict は、親の投稿を書き換えようとしたときに、ほかの呼び出しが先に書き換えていたこと
var ErrConflict = errors.New("parent was updated concurrently")

// API は、Store が使う DynamoDB の操作 (テストで差し替える)
type API interface {
	// GetItem は、親の投稿を読む
	GetItem(ctx context.Context, in *ddb.GetItemInput, optFns ...func(*ddb.Options)) (*ddb.GetItemOutput, error)

	// PutItem は、親の投稿を書く
	PutItem(ctx context.Context, in *ddb.PutItemInput, optFns ...func(*ddb.Options)) (*ddb.PutItemOutput, error)

	// UpdateItem は、手順の印を付ける・外す
	UpdateItem(ctx context.Context, in *ddb.UpdateItemInput, optFns ...func(*ddb.Options)) (*ddb.UpdateItemOutput, error)
}

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

// 式の中の属性の名前 (予約語とぶつからないよう、すべて #名前 で参照する)
//
// DOC: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/ReservedWords.html
var stepAttrNames = map[string]string{
	"#lease": "lease_until",
	"#done":  "done",
	"#exp":   "expires_at",
}

func stepKey(key string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{keyAttr: &types.AttributeValueMemberS{Value: stepKeyPrefix + key}}
}

func parentKey(deploymentID string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{keyAttr: &types.AttributeValueMemberS{Value: parentKeyPrefix + deploymentID}}
}

func unix(t time.Time) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatInt(t.Unix(), 10)}
}

// Claim は、done が無く、lease_until の期限が切れているときだけ、印を付ける
//   - 条件に合わなかったときは、そのときのアイテムを見て、済んでいるのか処理中なのかを返す
//
// DOC: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Expressions.ConditionExpressions.html
func (s *Store) Claim(ctx context.Context, key string, now time.Time) (ClaimResult, error) {
	_, err := s.client.UpdateItem(ctx, &ddb.UpdateItemInput{
		TableName:                aws.String(s.table),
		Key:                      stepKey(key),
		UpdateExpression:         aws.String("SET #lease = :until, #exp = :exp"),
		ConditionExpression:      aws.String("attribute_not_exists(#done) AND (attribute_not_exists(#lease) OR #lease < :now)"),
		ExpressionAttributeNames: stepAttrNames,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":until": unix(now.Add(s.lease)),
			":exp":   unix(now.Add(itemTTL)),
			":now":   unix(now),
		},
		ReturnValuesOnConditionCheckFailure: types.ReturnValuesOnConditionCheckFailureAllOld,
	})
	if err == nil {
		return ClaimAcquired, nil
	}

	var ccf *types.ConditionalCheckFailedException
	if !errors.As(err, &ccf) {
		return 0, fmt.Errorf("claim %s: %w", key, err)
	}
	if _, ok := ccf.Item["done"]; ok {
		return ClaimDone, nil
	}
	return ClaimBusy, nil
}

// Done は、done を付け、lease_until を外す
func (s *Store) Done(ctx context.Context, key string, now time.Time) error {
	_, err := s.client.UpdateItem(ctx, &ddb.UpdateItemInput{
		TableName:                aws.String(s.table),
		Key:                      stepKey(key),
		UpdateExpression:         aws.String("SET #done = :done, #exp = :exp REMOVE #lease"),
		ExpressionAttributeNames: stepAttrNames,
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":done": &types.AttributeValueMemberBOOL{Value: true},
			":exp":  unix(now.Add(itemTTL)),
		},
	})
	if err != nil {
		return fmt.Errorf("mark %s done: %w", key, err)
	}
	return nil
}

// Release は、lease_until を外す
func (s *Store) Release(ctx context.Context, key string) error {
	_, err := s.client.UpdateItem(ctx, &ddb.UpdateItemInput{
		TableName:                aws.String(s.table),
		Key:                      stepKey(key),
		UpdateExpression:         aws.String("REMOVE #lease"),
		ExpressionAttributeNames: map[string]string{"#lease": "lease_until"},
	})
	if err != nil {
		return fmt.Errorf("release %s: %w", key, err)
	}
	return nil
}

// GetParent は、強い整合性で親の投稿を読む (書いた直後に、ほかの呼び出しから読めるようにする)。無ければ nil を返す
func (s *Store) GetParent(ctx context.Context, deploymentID string) (*deploy.Parent, error) {
	out, err := s.client.GetItem(ctx, &ddb.GetItemInput{
		TableName:      aws.String(s.table),
		Key:            parentKey(deploymentID),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("get parent %s: %w", deploymentID, err)
	}
	if len(out.Item) == 0 {
		return nil, nil
	}
	return parentFromItem(deploymentID, out.Item)
}

// PutParent は、rev を条件に親の投稿を書く
//   - p.Rev が 1 以下なら新しく作る (すでにあれば ErrConflict)
//   - それ以外は、今の rev が p.Rev - 1 のときだけ書く (違えば ErrConflict)
func (s *Store) PutParent(ctx context.Context, p *deploy.Parent, now time.Time) error {
	in := putParentInput(s.table, p, now)

	_, err := s.client.PutItem(ctx, in)
	if err == nil {
		return nil
	}

	var ccf *types.ConditionalCheckFailedException
	if errors.As(err, &ccf) {
		return ErrConflict
	}
	return fmt.Errorf("put parent %s: %w", p.DeploymentID, err)
}

// putParentInput は、親の投稿を書く PutItem の入力 (rev による条件付き)
func putParentInput(table string, p *deploy.Parent, now time.Time) *ddb.PutItemInput {
	in := &ddb.PutItemInput{
		TableName: aws.String(table),
		Item:      parentToItem(p, now),
	}
	if p.Rev <= 1 {
		in.ConditionExpression = aws.String("attribute_not_exists(#pk)")
		in.ExpressionAttributeNames = map[string]string{"#pk": keyAttr}
		return in
	}

	in.ConditionExpression = aws.String("#rev = :prev")
	in.ExpressionAttributeNames = map[string]string{"#rev": "rev"}
	in.ExpressionAttributeValues = map[string]types.AttributeValue{
		":prev": &types.AttributeValueMemberN{Value: strconv.Itoa(p.Rev - 1)},
	}
	return in
}

func parentToItem(p *deploy.Parent, now time.Time) map[string]types.AttributeValue {
	item := parentKey(p.DeploymentID)
	str := func(name, v string) {
		if v == "" {
			return
		}
		item[name] = &types.AttributeValueMemberS{Value: v}
	}
	str("ts", p.TS)
	str("channel", p.Channel)
	str("service", p.Service)
	str("status", string(p.Status))
	str("version", p.Version)
	str("actor", p.Actor)
	str("run_url", p.RunURL)
	str("task_definition", p.TaskDefinition)
	if !p.CompletedAt.IsZero() {
		str("completed_at", p.CompletedAt.UTC().Format(time.RFC3339))
	}
	item["rev"] = &types.AttributeValueMemberN{Value: strconv.Itoa(p.Rev)}
	item["expires_at"] = unix(now.Add(itemTTL))
	return item
}

func parentFromItem(deploymentID string, item map[string]types.AttributeValue) (*deploy.Parent, error) {
	str := func(name string) string {
		v, ok := item[name].(*types.AttributeValueMemberS)
		if !ok {
			return ""
		}
		return v.Value
	}

	p := &deploy.Parent{
		DeploymentID:   deploymentID,
		TS:             str("ts"),
		Channel:        str("channel"),
		Service:        str("service"),
		Status:         deploy.Status(str("status")),
		Version:        str("version"),
		Actor:          str("actor"),
		RunURL:         str("run_url"),
		TaskDefinition: str("task_definition"),
	}

	completedAt, err := parseTime(str("completed_at"))
	if err != nil {
		return nil, fmt.Errorf("parent %s: completed_at: %w", deploymentID, err)
	}
	p.CompletedAt = completedAt

	rev, err := parseRev(item["rev"])
	if err != nil {
		return nil, fmt.Errorf("parent %s: rev: %w", deploymentID, err)
	}
	p.Rev = rev
	return p, nil
}

// parseTime は、RFC 3339 の時刻を読む。空ならゼロ値
func parseTime(v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, v)
}

// parseRev は、rev を読む。無ければ 0
func parseRev(v types.AttributeValue) (int, error) {
	n, ok := v.(*types.AttributeValueMemberN)
	if !ok {
		return 0, nil
	}
	return strconv.Atoi(n.Value)
}
