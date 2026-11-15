# ================================================================================
# OIDC Provider
# ================================================================================

# GitHub Actions の OIDC プロバイダー (GitHub が発行するトークンを、AWS が信頼するための登録)
#   - 同じ URL のプロバイダーは、1 つの AWS アカウントに 1 つしか作れない
#   - この AWS アカウントでは tamaco-blog がすでに作って使っているため、ここでは作らずに参照だけする
data "aws_iam_openid_connect_provider" "github" {
  url = "https://token.actions.githubusercontent.com"
}
