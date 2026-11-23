package dynamodb

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

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
			p := &deploy.Parent{DeploymentID: testDeployment, Rev: tt.rev}
			err := New(db, "table").PutParent(t.Context(), p, testTime)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got := aws.ToString(db.lastPut.ConditionExpression); got != tt.wantCondition {
				t.Errorf("condition = %q, want %q", got, tt.wantCondition)
			}
		})
	}
}
