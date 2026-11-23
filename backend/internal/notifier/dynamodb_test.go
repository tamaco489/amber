package notifier

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// fakeDynamoDB は、決めた結果を返すだけの DynamoDBAPI (呼び出しの中身を残す)
type fakeDynamoDB struct {
	getOut     *dynamodb.GetItemOutput
	err        error
	lastPut    *dynamodb.PutItemInput
	lastUpdate *dynamodb.UpdateItemInput
}

func (f *fakeDynamoDB) GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return f.getOut, f.err
}

func (f *fakeDynamoDB) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	f.lastPut = in
	return &dynamodb.PutItemOutput{}, f.err
}

func (f *fakeDynamoDB) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	f.lastUpdate = in
	return &dynamodb.UpdateItemOutput{}, f.err
}

func TestDynamoDBStoreClaim(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    ClaimResult
		wantErr bool
	}{
		{name: "acquired", want: ClaimAcquired},
		{
			name: "done",
			err: &types.ConditionalCheckFailedException{Item: map[string]types.AttributeValue{
				"done": &types.AttributeValueMemberBOOL{Value: true},
			}},
			want: ClaimDone,
		},
		{
			name: "busy",
			err: &types.ConditionalCheckFailedException{Item: map[string]types.AttributeValue{
				"lease_until": &types.AttributeValueMemberN{Value: "1"},
			}},
			want: ClaimBusy,
		},
		{name: "other error", err: errors.New("throttled"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &fakeDynamoDB{err: tt.err}
			got, err := NewDynamoDBStore(db, "table").Claim(t.Context(), "ev#reply", testTime)

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("result = %v, want %v", got, tt.want)
			}
			key := db.lastUpdate.Key[keyAttr].(*types.AttributeValueMemberS).Value
			if key != "step#ev#reply" {
				t.Errorf("key = %q, want step#ev#reply", key)
			}
		})
	}
}

func TestDynamoDBStorePutParent(t *testing.T) {
	tests := []struct {
		name          string
		rev           int
		err           error
		wantCondition string
		wantErr       error
	}{
		{name: "create", rev: 1, wantCondition: "attribute_not_exists(#pk)"},
		{name: "update", rev: 3, wantCondition: "#rev = :prev"},
		{name: "conflict", rev: 3, err: &types.ConditionalCheckFailedException{}, wantCondition: "#rev = :prev", wantErr: ErrConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &fakeDynamoDB{err: tt.err}
			err := NewDynamoDBStore(db, "table").PutParent(t.Context(), Parent{DeploymentID: testDeployment, Rev: tt.rev}, testTime)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got := aws.ToString(db.lastPut.ConditionExpression); got != tt.wantCondition {
				t.Errorf("condition = %q, want %q", got, tt.wantCondition)
			}
		})
	}
}

func TestParentItemRoundTrip(t *testing.T) {
	want := Parent{
		DeploymentID:   testDeployment,
		TS:             "1700000000.000001",
		Channel:        testChannel,
		Service:        testService,
		Status:         StatusCompleted,
		Version:        "v3",
		Actor:          "tamaco489",
		RunURL:         testRunURL,
		TaskDefinition: testTaskDef,
		CompletedAt:    testTime,
		Rev:            4,
	}

	item := parentToItem(want, testTime)
	if got := item[keyAttr].(*types.AttributeValueMemberS).Value; got != "deployment#"+testDeployment {
		t.Errorf("pk = %q", got)
	}
	exp := item["expires_at"].(*types.AttributeValueMemberN).Value
	if wantExp := testTime.Add(30 * 24 * time.Hour).Unix(); exp != strconv.FormatInt(wantExp, 10) {
		t.Errorf("expires_at = %s, want %d", exp, wantExp)
	}

	got, err := parentFromItem(testDeployment, item)
	if err != nil {
		t.Fatal(err)
	}
	if !got.CompletedAt.Equal(want.CompletedAt) {
		t.Errorf("completed_at = %v, want %v", got.CompletedAt, want.CompletedAt)
	}
	got.CompletedAt, want.CompletedAt = time.Time{}, time.Time{}
	if got != want {
		t.Errorf("parent = %+v, want %+v", got, want)
	}
}
