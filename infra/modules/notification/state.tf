# ================================================================================
# State Table
# ================================================================================

# 通知の Lambda が、デプロイごとの状態を残すテーブル
#   - パーティションキーはデプロイ ID。親の投稿の ts と、処理したイベント・済んだ手順の印を残す (項目の形は Lambda の本体で決める)
#   - 古い項目は、TTL (expires_at、エポック秒) で消す。TTL での削除に料金はかからない
#   - オンデマンドのため、置いておくだけでは料金はかからない (リクエストと保存量の課金)
#   - ポイントインタイムリカバリは有料で、消えても次のデプロイの通知が新しいスレッドになるだけのため、使わない
# DOC: https://aws.amazon.com/dynamodb/pricing/on-demand/
resource "aws_dynamodb_table" "notifier_state" {
  name         = "${local.name_prefix}-notification"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "deployment_id"

  attribute {
    name = "deployment_id"
    type = "S"
  }

  ttl {
    attribute_name = "expires_at"
    enabled        = true
  }
}
