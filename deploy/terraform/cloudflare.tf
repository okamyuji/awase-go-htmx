# Cloudflare 側の資産です。ゾーンの SSL モードは feedflow の terraform が管理しているため、ここでは変えません。

data "cloudflare_zone" "this" {
  name = var.zone_name
}

# nginx で実際の接続元 IP を復元するために使います。
data "cloudflare_ip_ranges" "cloudflare" {}

resource "tls_private_key" "origin" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_cert_request" "origin" {
  private_key_pem = tls_private_key.origin.private_key_pem

  subject {
    common_name = var.hostname
  }

  dns_names = [var.hostname]
}

# Cloudflare とオリジンの間だけで有効な証明書です。SSL モード Full(strict) で Cloudflare が検証します。
resource "cloudflare_origin_ca_certificate" "origin" {
  csr                = tls_cert_request.origin.cert_request_pem
  hostnames          = [var.hostname]
  request_type       = "origin-rsa"
  requested_validity = 5475
}

# プロキシを有効にして、HTTPS を Cloudflare で終端し、オリジンの IP を隠します。
resource "cloudflare_record" "app" {
  zone_id = data.cloudflare_zone.this.id
  name    = var.hostname
  type    = "A"
  content = var.origin_public_ip
  proxied = true
  ttl     = 1
  comment = "awase origin (feedflow EC2 に同居)"
}
