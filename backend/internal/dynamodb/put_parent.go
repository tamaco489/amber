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

// ErrConflict は、親の投稿を書き換えようとしたときに、ほかの呼び出しが先に書き換えていたこと
var ErrConflict = errors.New("parent was updated concurrently")

// PutParent は、rev を条件に親の投稿を書く
//   - p.Rev が 1 以下なら新しく作る (すでにあれば ErrConflict)
//   - それ以外は、今の rev が p.Rev - 1 のときだけ書く (違えば ErrConflict)
func (s *Store) PutParent(ctx context.Context, p *deploy.Parent, now time.Time) error {
	in := putParentInput(s.table, p, now)

	_, err := s.client.PutItem(ctx, in)
	if err == nil {
		return nil
	}

	if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
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
