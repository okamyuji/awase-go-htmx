variable "origin_public_ip" {
  description = "同居先 EC2 の Elastic IP です。feedflow の deploy/terraform で terraform output -raw elastic_ip を実行して取得します。"
  type        = string
}

variable "ssh_private_key_path" {
  description = "同居先 EC2 へ SSH するための秘密鍵のパスです。feedflow の terraform が作った鍵を使います。"
  type        = string
  default     = "../../../feedflow-go-htmx/deploy/terraform/feedflow_ssh_key.pem"
}

variable "cloudflare_api_token" {
  description = "Cloudflare の API トークンです。DNS、ゾーンの読み取り、SSL と証明書、ゾーンの WAF の編集権限が要ります。"
  type        = string
  sensitive   = true
}

variable "zone_name" {
  description = "Cloudflare で管理しているゾーン名です。"
  type        = string
  default     = "okamyuji.work"
}

variable "hostname" {
  description = "awase を公開するホスト名です。"
  type        = string
  default     = "awase.okamyuji.work"
}
