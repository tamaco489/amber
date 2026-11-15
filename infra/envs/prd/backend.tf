# state を置くバケットは、その state 自身では管理できないため Terraform の対象外とし、AWS CLI で事前に作成している
# 認証情報は backend に書かず、実行する環境ごとに外から渡す
#   - 手元: 環境変数 AWS_PROFILE
#   - CI: GitHub Actions の OIDC で得た一時クレデンシャル
terraform {
  backend "s3" {
    bucket       = "prd-amber-tfstate"
    key          = "envs/prd/terraform.tfstate"
    region       = "ap-northeast-1"
    encrypt      = true
    use_lockfile = true
  }
}
