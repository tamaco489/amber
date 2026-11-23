package notifier

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// アイテムの置き方は doc.go に書く
const (
	keyAttr          = "pk"
	parentKeyPrefix  = "deployment#"
	stepKeyPrefix    = "step#"
	itemTTL          = 30 * 24 * time.Hour
	defaultLeaseTime = 60 * time.Second
)

// DynamoDBAPI は、DynamoDBStore が使う DynamoDB の操作 (テストで差し替える)
type DynamoDBAPI interface {
	GetItem(ctx context.Context, in *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, in *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	UpdateItem(ctx context.Context, in *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
}

// DynamoDBStore は、Store を DynamoDB のテーブルで実装する
type DynamoDBStore struct {
	client DynamoDBAPI
	table  string

	// 手順の印の期限
	//   - Lambda のタイムアウトより長くする。短いと、処理中の呼び出しがあるのに、再試行が同じ手順を進めてしまう
	//   - Lambda の非同期呼び出しの再試行 (1 分ほどあとから) で、期限が切れているくらいにする
	lease time.Duration
}

// NewDynamoDBStore は、テーブル名 table の DynamoDBStore を作る
func NewDynamoDBStore(client DynamoDBAPI, table string) *DynamoDBStore {
	return &DynamoDBStore{client: client, table: table, lease: defaultLeaseTime}
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
func (s *DynamoDBStore) Claim(ctx context.Context, key string, now time.Time) (ClaimResult, error) {
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
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
func (s *DynamoDBStore) Done(ctx context.Context, key string, now time.Time) error {
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
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
func (s *DynamoDBStore) Release(ctx context.Context, key string) error {
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
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

// GetParent は、強い整合性で親の投稿を読む (書いた直後に、ほかの呼び出しから読めるようにする)
func (s *DynamoDBStore) GetParent(ctx context.Context, deploymentID string) (Parent, bool, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.table),
		Key:            parentKey(deploymentID),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return Parent{}, false, fmt.Errorf("get parent %s: %w", deploymentID, err)
	}
	if len(out.Item) == 0 {
		return Parent{}, false, nil
	}
	p, err := parentFromItem(deploymentID, out.Item)
	if err != nil {
		return Parent{}, false, err
	}
	return p, true, nil
}

// PutParent は、rev を条件に親の投稿を書く
func (s *DynamoDBStore) PutParent(ctx context.Context, p Parent, now time.Time) error {
	in := &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item:      parentToItem(p, now),
	}
	if p.Rev <= 1 {
		in.ConditionExpression = aws.String("attribute_not_exists(#pk)")
		in.ExpressionAttributeNames = map[string]string{"#pk": keyAttr}
	} else {
		in.ConditionExpression = aws.String("#rev = :prev")
		in.ExpressionAttributeNames = map[string]string{"#rev": "rev"}
		in.ExpressionAttributeValues = map[string]types.AttributeValue{
			":prev": &types.AttributeValueMemberN{Value: strconv.Itoa(p.Rev - 1)},
		}
	}

	_, err := s.client.PutItem(ctx, in)
	var ccf *types.ConditionalCheckFailedException
	if errors.As(err, &ccf) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("put parent %s: %w", p.DeploymentID, err)
	}
	return nil
}

func parentToItem(p Parent, now time.Time) map[string]types.AttributeValue {
	item := parentKey(p.DeploymentID)
	str := func(name, v string) {
		if v != "" {
			item[name] = &types.AttributeValueMemberS{Value: v}
		}
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

func parentFromItem(deploymentID string, item map[string]types.AttributeValue) (Parent, error) {
	str := func(name string) string {
		if v, ok := item[name].(*types.AttributeValueMemberS); ok {
			return v.Value
		}
		return ""
	}

	p := Parent{
		DeploymentID:   deploymentID,
		TS:             str("ts"),
		Channel:        str("channel"),
		Service:        str("service"),
		Status:         Status(str("status")),
		Version:        str("version"),
		Actor:          str("actor"),
		RunURL:         str("run_url"),
		TaskDefinition: str("task_definition"),
	}
	if v := str("completed_at"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return Parent{}, fmt.Errorf("parent %s: completed_at: %w", deploymentID, err)
		}
		p.CompletedAt = t
	}
	if v, ok := item["rev"].(*types.AttributeValueMemberN); ok {
		rev, err := strconv.Atoi(v.Value)
		if err != nil {
			return Parent{}, fmt.Errorf("parent %s: rev: %w", deploymentID, err)
		}
		p.Rev = rev
	}
	return p, nil
}
