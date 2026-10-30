output "app_url" {
  description = "公開 URL です。"
  value       = "https://${var.hostname}"
}

output "dns_record" {
  description = "作成した A レコードです。"
  value       = "${cloudflare_record.app.name} A ${var.origin_public_ip} (proxied)"
}

output "passwd_command" {
  description = "共有アカウントを設定するコマンドの例です。パスワードは標準入力から渡します。"
  value       = "ssh -i ${var.ssh_private_key_path} ec2-user@${var.origin_public_ip} 'cd /home/ec2-user/awase && sudo docker compose exec -T awase-app /awase passwd -username <名前>' < password.txt"
}
