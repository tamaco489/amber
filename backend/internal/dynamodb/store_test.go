package dynamodb

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/tamaco489/amber/backend/internal/deploy"
	"github.com/tamaco489/amber/backend/internal/deploy/deploytest"
)

// fakeAPI は、決めた結果を返すだけの API (呼び出しの中身を残す)
type fakeAPI struct {
	getOut     *ddb.GetItemOutput   // GetItem が返す値
	err        error                // どの操作も、このエラーを返す
	lastPut    *ddb.PutItemInput    // 最後の PutItem の入力
	lastUpdate *ddb.UpdateItemInput // 最後の UpdateItem の入力
}

func (f *fakeAPI) GetItem(context.Context, *ddb.GetItemInput, ...func(*ddb.Options)) (*ddb.GetItemOutput, error) {
	return f.getOut, f.err
}

func (f *fakeAPI) PutItem(_ context.Context, in *ddb.PutItemInput, _ ...func(*ddb.Options)) (*ddb.PutItemOutput, error) {
	f.lastPut = in
	return &ddb.PutItemOutput{}, f.err
}

func (f *fakeAPI) UpdateItem(_ context.Context, in *ddb.UpdateItemInput, _ ...func(*ddb.Options)) (*ddb.UpdateItemOutput, error) {
	f.lastUpdate = in
	return &ddb.UpdateItemOutput{}, f.err
}

func TestStoreClaim(t *testing.T) {
	tests := map[string]struct {
		err     error
		want    ClaimResult
		wantErr bool
	}{
		"印を付けられる": {
			want: ClaimAcquired,
		},
		"済んでいる": {
			err: &types.ConditionalCheckFailedException{Item: map[string]types.AttributeValue{
				"done": &types.AttributeValueMemberBOOL{Value: true},
			}},
			want: ClaimDone,
		},
		"ほかの呼び出しが進めている": {
			err: &types.ConditionalCheckFailedException{Item: map[string]types.AttributeValue{
				"lease_until": &types.AttributeValueMemberN{Value: "1"},
			}},
			want: ClaimBusy,
		},
		"条件以外のエラーはエラーを返す": {
			err:     errors.New("throttled"),
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			db := &fakeAPI{err: tt.err}
			got, err := New(db, "table").Claim(t.Context(), "ev#reply", deploytest.Time)

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

func TestStorePutParent(t *testing.T) {
	tests := map[string]struct {
		rev           int
		err           error
		wantCondition string
		wantErr       error
	}{
		"新しく作るときは、無いことを条件にする": {
			rev:           1,
			wantCondition: "attribute_not_exists(#pk)",
		},
		"書き換えるときは、1 つ前の rev を条件にする": {
			rev:           3,
			wantCondition: "#rev = :prev",
		},
		"先に書き換えられていたら ErrConflict を返す": {
			rev:           3,
			err:           &types.ConditionalCheckFailedException{},
			wantCondition: "#rev = :prev",
			wantErr:       ErrConflict,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			db := &fakeAPI{err: tt.err}
			p := &deploy.Parent{DeploymentID: deploytest.Deployment, Rev: tt.rev}
			err := New(db, "table").PutParent(t.Context(), p, deploytest.Time)

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
	want := &deploy.Parent{
		DeploymentID:   deploytest.Deployment,
		TS:             deploytest.ParentTS,
		Channel:        deploytest.Channel,
		Service:        deploytest.Service,
		Status:         deploy.StatusCompleted,
		Version:        "v3",
		Actor:          "tamaco489",
		RunURL:         deploytest.RunURL,
		TaskDefinition: deploytest.TaskDef,
		CompletedAt:    deploytest.Time,
		Rev:            4,
	}

	item := parentToItem(want, deploytest.Time)
	if got := item[keyAttr].(*types.AttributeValueMemberS).Value; got != "deployment#"+deploytest.Deployment {
		t.Errorf("pk = %q", got)
	}
	exp := item["expires_at"].(*types.AttributeValueMemberN).Value
	if wantExp := deploytest.Time.Add(30 * 24 * time.Hour).Unix(); exp != strconv.FormatInt(wantExp, 10) {
		t.Errorf("expires_at = %s, want %d", exp, wantExp)
	}

	got, err := parentFromItem(deploytest.Deployment, item)
	if err != nil {
		t.Fatal(err)
	}
	if !got.CompletedAt.Equal(want.CompletedAt) {
		t.Errorf("completed_at = %v, want %v", got.CompletedAt, want.CompletedAt)
	}
	got.CompletedAt, want.CompletedAt = time.Time{}, time.Time{}
	if *got != *want {
		t.Errorf("parent = %+v, want %+v", got, want)
	}
}
