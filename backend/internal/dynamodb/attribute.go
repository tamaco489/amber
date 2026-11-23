package dynamodb

import (
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// アイテムの置き方は、パッケージのコメントに書く
const (
	keyAttr          = "pk"
	parentKeyPrefix  = "deployment#"
	stepKeyPrefix    = "step#"
	itemTTL          = 30 * 24 * time.Hour
	DefaultLeaseTime = 60 * time.Second
)

// 式の中の属性の名前 (予約語とぶつからないよう、すべて #名前 で参照する)
//
// DOC: https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/ReservedWords.html
var stepAttrNames = map[string]string{
	"#lease": "lease_until",
	"#done":  "done",
	"#exp":   "expires_at",
}

// stepKey は、手順の記録のキー
func stepKey(key string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{keyAttr: &types.AttributeValueMemberS{Value: stepKeyPrefix + key}}
}

// parentKey は、親の投稿のキー
func parentKey(deploymentID string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{keyAttr: &types.AttributeValueMemberS{Value: parentKeyPrefix + deploymentID}}
}

// unix は、時刻を Unix 秒の数値の属性にする
func unix(t time.Time) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: strconv.FormatInt(t.Unix(), 10)}
}
