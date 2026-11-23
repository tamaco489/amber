package dynamodb

import (
	"context"

	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// API は、Store が使う DynamoDB の操作 (テストで差し替える)
type API interface {
	// GetItem は、親の投稿を読む
	GetItem(ctx context.Context, in *ddb.GetItemInput, optFns ...func(*ddb.Options)) (*ddb.GetItemOutput, error)

	// PutItem は、親の投稿を書く
	PutItem(ctx context.Context, in *ddb.PutItemInput, optFns ...func(*ddb.Options)) (*ddb.PutItemOutput, error)

	// UpdateItem は、手順の印を付ける・外す
	UpdateItem(ctx context.Context, in *ddb.UpdateItemInput, optFns ...func(*ddb.Options)) (*ddb.UpdateItemOutput, error)
}
