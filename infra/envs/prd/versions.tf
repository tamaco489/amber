terraform {
  required_version = "~> 1.17.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.73"
    }

    # 通知の Lambda の仮のコード (zip) を作るため (modules/notification)
    archive = {
      source  = "hashicorp/archive"
      version = "~> 2.8"
    }
  }
}
