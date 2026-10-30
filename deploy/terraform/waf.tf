# ゾーンの WAF のルールセットです。ルールセットは段階ごとにゾーンで1つしか作れないため、
# awase の terraform がゾーンで共有する資産として持ちます。feedflow や boki3 がルールを足す場合もここへ加えます。
# このため awase の terraform で destroy を実行すると、他のアプリのルールも消えます。

resource "cloudflare_ruleset" "custom" {
  zone_id     = data.cloudflare_zone.this.id
  name        = "zone custom rules"
  description = "ゾーンのカスタムルール"
  kind        = "zone"
  phase       = "http_request_firewall_custom"

  rules {
    action      = "block"
    expression  = "(http.host eq \"${var.hostname}\" and not ip.src.country in {\"JP\"})"
    description = "awase: 日本以外からの要求を遮断する"
    enabled     = true
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
