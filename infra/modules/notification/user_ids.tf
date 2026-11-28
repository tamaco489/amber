# ================================================================================
# Slack User IDs
# ================================================================================

# 通知の Lambda が、デプロイを起動した人にメンションするための対応表
#   - 値は JSON で、GitHub のユーザー名から Slack のユーザー ID を引く ({"<GitHub のユーザー名>": "<Slack のユーザー ID>"})
#   - 公開リポジトリのため、Slack のユーザー ID をコードと tfvars に書かない。Terraform では空の対応表で作り、本物の値は aws ssm put-parameter --overwrite で入れる
#   - ボットトークンと同じく、標準のパラメータの SecureString (AWS マネージドキー aws/ssm で暗号化) にし、書き込み専用の value_wo で書く
#     (state に値を残さず、CLI で入れた値を空の対応表へ戻さない)
# DOC: https://aws.amazon.com/systems-manager/pricing/
# DOC: https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/ssm_parameter
resource "aws_ssm_parameter" "slack_user_ids" {
  name             = "/${local.name_prefix}/notification/slack-user-ids"
  description      = "JSON map from GitHub usernames to Slack user IDs, to mention the person who started a deployment. The real map is set with the AWS CLI, not Terraform."
  type             = "SecureString"
  tier             = "Standard"
  value_wo         = "{}"
  value_wo_version = 1
}
