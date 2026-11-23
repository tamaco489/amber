package dynamodb

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

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
