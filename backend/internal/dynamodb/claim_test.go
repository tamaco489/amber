package dynamodb

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

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
			err: conditionFailed(map[string]types.AttributeValue{
				"done": &types.AttributeValueMemberBOOL{Value: true},
			}),
			want: ClaimDone,
		},
		"ほかの呼び出しが進めている": {
			err: conditionFailed(map[string]types.AttributeValue{
				"lease_until": &types.AttributeValueMemberN{Value: "1"},
			}),
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
			got, err := New(db, "table").Claim(t.Context(), "ev#reply", testTime)

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("result = %v, want %v", got, tt.want)
			}
			if key := keyOf(db.lastUpdate.Key); key != "step#ev#reply" {
				t.Errorf("key = %q, want step#ev#reply", key)
			}
		})
	}
}
