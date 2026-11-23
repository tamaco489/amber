package dynamodb

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

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
