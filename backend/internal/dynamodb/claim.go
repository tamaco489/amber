package dynamodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ClaimResult は、手順に印を付けようとした結果
type ClaimResult int

// 手順に印を付けようとした結果
const (
	ClaimAcquired ClaimResult = iota + 1 // 印を付けた。この呼び出しが手順を進める
	ClaimDone                            // すでに済んでいる。手順を飛ばす
	ClaimBusy                            // ほかの呼び出しが進めている
)

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

	ccf, ok := errors.AsType[*types.ConditionalCheckFailedException](err)
	if !ok {
		return 0, fmt.Errorf("claim %s: %w", key, err)
	}
	if _, ok := ccf.Item["done"]; ok {
		return ClaimDone, nil
	}
	return ClaimBusy, nil
}
