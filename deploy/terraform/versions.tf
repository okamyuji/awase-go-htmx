# awase は feedflow の既存の EC2 へ同居するため、AWS の資産は作りません。
# 管理するのは Cloudflare（DNS、Origin CA 証明書、WAF）と、既存の EC2 への配送だけです。
terraform {
  required_version = ">= 1.6.0"

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.40"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
    null = {
      source  = "hashicorp/null"
      version = "~> 3.2"
    }
  }
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}
