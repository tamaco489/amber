# ================================================================================
# Slack Bot Token
# ================================================================================

# 通知の Lambda が Slack の API を呼ぶときのボットトークン
#   - 標準のパラメータの SecureString (AWS マネージドキー aws/ssm で暗号化)。置いておくだけでは料金はかからない
#   - Terraform ではダミー値で作り、本物のトークンは aws ssm put-parameter --overwrite で入れる
#   - value ではなく書き込み専用の value_wo を使う。value だと、plan のたびに復号した本物の値を読み、state に残すため
#   - value_wo は value_wo_version を上げたときだけ書き込むため、CLI で入れた値をダミー値へ戻さない
# DOC: https://aws.amazon.com/systems-manager/pricing/
# DOC: https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter
resource "aws_ssm_parameter" "slack_bot_token" {
  name             = "/${local.name_prefix}/notification/slack-bot-token"
  description      = "Slack bot token for the notification Lambda. The real token is set with the AWS CLI, not Terraform."
  type             = "SecureString"
  tier             = "Standard"
  value_wo         = "dummy"
  value_wo_version = 1
}
