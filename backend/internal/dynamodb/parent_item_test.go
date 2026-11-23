package dynamodb

import (
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/tamaco489/amber/backend/internal/deploy"
)

func TestParentItemRoundTrip(t *testing.T) {
	want := &deploy.Parent{
		DeploymentID:   testDeployment,
		TS:             testParentTS,
		Channel:        testChannel,
		Service:        testService,
		Status:         deploy.StatusCompleted,
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
	if *got != *want {
		t.Errorf("parent = %+v, want %+v", got, want)
	}
}
