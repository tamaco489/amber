provider "aws" {
  region = var.region

  default_tags {
    tags = {
      project     = "amber"
      environment = var.env
      managed_by  = "terraform"
    }
  }
}
