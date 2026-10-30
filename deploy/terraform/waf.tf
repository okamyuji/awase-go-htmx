# ゾーンの WAF のうち、レート制限の段階のルールセットは awase の terraform が持ちます。
# ルールセットは段階ごとにゾーンで1つしか作れないため、feedflow や boki3 がレート制限を足す場合もここへ加えます。
# このため awase の terraform で destroy を実行すると、他のアプリのレート制限も消えます。
#
# カスタムルールの段階には AI Crawl Control が作るルールが同居するため、ルールセットごとは持ちません。
# awase の国別遮断のルールだけを、ルール単位の API で追加と削除をします。

resource "null_resource" "country_rule" {
  triggers = {
    zone     = var.zone_name
    hostname = var.hostname
    script   = abspath("${path.module}/../scripts/cf-country-rule.sh")
  }

  provisioner "local-exec" {
    command = "${self.triggers.script} add ${self.triggers.zone} ${self.triggers.hostname}"
    environment = {
      CF_API_TOKEN = var.cloudflare_api_token
    }
  }

  # destroy では変数を参照できないため、スクリプトが secrets.auto.tfvars からトークンを読みます。
  provisioner "local-exec" {
    when    = destroy
    command = "${self.triggers.script} remove ${self.triggers.zone} ${self.triggers.hostname}"
  }
}

# Free プランのレート制限は1ルールまでで、式に使える項目はパスと Verified Bot だけです。
# そのためホスト名で絞れず、feedflow と boki3 の /login にも同じ条件がかかります。
resource "cloudflare_ruleset" "ratelimit" {
  zone_id     = data.cloudflare_zone.this.id
  name        = "zone rate limiting"
  description = "ゾーンのレート制限"
  kind        = "zone"
  phase       = "http_ratelimit"

  rules {
    action      = "block"
    expression  = "(http.request.uri.path eq \"/login\")"
    description = "/login への連続した要求を IP ごとに遮断する"
    enabled     = true

    ratelimit {
      characteristics     = ["cf.colo.id", "ip.src"]
      period              = 10
      requests_per_period = 10
      mitigation_timeout  = 10
    }
  }
}
