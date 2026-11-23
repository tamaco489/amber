package dynamodb

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

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
