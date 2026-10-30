# イメージをローカルで arm64 向けにビルドして転送し、EC2 では取り込みと起動だけを行います。
# 同居先の t4g は1GB のメモリを feedflow と boki3 と共有しており、EC2 で Go をビルドすると同居アプリを巻き込むためです。
#
# 前提（feedflow の terraform が構築済みであること）:
#   - compose プロジェクト feedflow が /home/ec2-user/feedflow で動き、ネットワーク feedflow_internal と nginx を持つ
#   - /etc/feedflow/conf.d と /etc/feedflow/tls が nginx にマウント済み
#   - データ用の EBS が /mnt/feedflow-data にマウント済み

locals {
  repo_root  = abspath("${path.module}/../..")
  tmp_dir    = abspath("${path.module}/.terraform-tmp")
  image_path = "${local.tmp_dir}/awase-image.tar.gz"

  build_files = sort(concat(
    tolist(fileset(local.repo_root, "cmd/**")),
    tolist(fileset(local.repo_root, "internal/**")),
    tolist(fileset(local.repo_root, "web/**")),
    ["go.mod", "go.sum", "Dockerfile", "compose.yml"],
  ))
  build_hash = sha256(join("", [for f in local.build_files : filesha256("${local.repo_root}/${f}")]))

  cloudflare_cidrs = concat(
    data.cloudflare_ip_ranges.cloudflare.ipv4_cidr_blocks,
    data.cloudflare_ip_ranges.cloudflare.ipv6_cidr_blocks,
  )
  nginx_conf = templatefile("${path.module}/templates/awase.cloudflare.conf.tftpl", {
    hostname           = var.hostname
    real_ip_from_block = join("\n", [for c in local.cloudflare_cidrs : "    set_real_ip_from ${c};"])
  })

  swap_script     = "${local.repo_root}/deploy/scripts/swap-nginx.sh"
  teardown_script = "${local.repo_root}/deploy/scripts/teardown-nginx.sh"
}

resource "null_resource" "image" {
  triggers = {
    build_hash = local.build_hash
  }

  provisioner "local-exec" {
    working_dir = local.repo_root
    command     = "mkdir -p ${local.tmp_dir} && docker buildx build --platform linux/arm64 --load -t awase:dev . && docker save awase:dev | gzip > ${local.image_path}"
  }
}

# 撤去の手順だけを持つリソースです。配送のリソースは再配備のたびに作り直されるため、
# そこに destroy 時のプロビジョナを置くと再配備のたびに awase が外れます。
# 作成時は何もしないので、配送が失敗しても汚染された状態にならず、destroy で必ず撤去が走ります。
resource "null_resource" "teardown" {
  triggers = {
    host            = var.origin_public_ip
    key_path        = var.ssh_private_key_path
    teardown_script = local.teardown_script
  }

  connection {
    type        = "ssh"
    host        = self.triggers.host
    user        = "ec2-user"
    private_key = file(self.triggers.key_path)
  }

  provisioner "file" {
    when        = destroy
    source      = self.triggers.teardown_script
    destination = "/home/ec2-user/awase-teardown.sh"
  }

  provisioner "remote-exec" {
    when = destroy
    inline = [
      "set -eux",
      "bash /home/ec2-user/awase-teardown.sh",
      "if [ -f /home/ec2-user/awase/compose.yml ]; then cd /home/ec2-user/awase && sudo docker compose down; fi",
      # データの /mnt/feedflow-data/awase は消さずに残します。
      "rm -rf /home/ec2-user/awase /home/ec2-user/awase-teardown.sh",
    ]
  }
}

resource "null_resource" "deploy" {
  # 接続先と鍵のパスも持たせ、それらが変わって撤去が走ったときに配送も続けて実行されるようにします。
  triggers = {
    build_hash  = local.build_hash
    cert_id     = cloudflare_origin_ca_certificate.origin.id
    nginx_conf  = sha256(local.nginx_conf)
    swap_script = filesha256(local.swap_script)
    host        = var.origin_public_ip
    key_path    = var.ssh_private_key_path
  }

  depends_on = [null_resource.image, null_resource.teardown]

  connection {
    type        = "ssh"
    host        = var.origin_public_ip
    user        = "ec2-user"
    private_key = file(var.ssh_private_key_path)
  }

  provisioner "remote-exec" {
    inline = ["mkdir -p /home/ec2-user/awase-new"]
  }

  provisioner "file" {
    source      = local.image_path
    destination = "/home/ec2-user/awase-image.tar.gz"
  }

  provisioner "file" {
    source      = "${local.repo_root}/compose.yml"
    destination = "/home/ec2-user/awase-compose.yml"
  }

  provisioner "file" {
    source      = local.swap_script
    destination = "/home/ec2-user/awase-swap.sh"
  }

  provisioner "file" {
    content     = local.nginx_conf
    destination = "/home/ec2-user/awase-new/zz-awase.conf"
  }

  provisioner "file" {
    content     = cloudflare_origin_ca_certificate.origin.certificate
    destination = "/home/ec2-user/awase-new/awase.crt"
  }

  provisioner "file" {
    content     = tls_private_key.origin.private_key_pem
    destination = "/home/ec2-user/awase-new/awase.key"
  }

  provisioner "remote-exec" {
    inline = [
      "set -eux",
      # file プロビジョナは既定の umask で書くので、鍵の権限を最初に絞ります。
      "chmod 600 /home/ec2-user/awase-new/awase.key",
      "sudo docker network inspect feedflow_internal >/dev/null",
      "cd /home/ec2-user/feedflow && sudo docker compose -f compose.yml -f compose.override.yml ps --services --status running | grep -qx nginx",
      # 実行イメージは distroless の nonroot（uid 65532）なので、データの置き場の所有者を合わせます。
      "sudo mkdir -p /mnt/feedflow-data/awase",
      "sudo chown 65532:65532 /mnt/feedflow-data/awase",
      "sudo docker load -i /home/ec2-user/awase-image.tar.gz",
      "rm -f /home/ec2-user/awase-image.tar.gz",
      "mkdir -p /home/ec2-user/awase",
      "mv /home/ec2-user/awase-compose.yml /home/ec2-user/awase/compose.yml",
      "cd /home/ec2-user/awase && sudo docker compose up -d --no-build",
      "bash /home/ec2-user/awase-swap.sh /home/ec2-user/awase-new",
      "rm -rf /home/ec2-user/awase-new /home/ec2-user/awase-swap.sh",
      # awase-app はポートを公開しないので、コンテナの IP へホストから接続して確かめます。
      "cd /home/ec2-user/awase && CID=$(sudo docker compose ps -q awase-app) && IP=$(sudo docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' \"$CID\") && curl -fsS -m 10 --retry 5 --retry-delay 2 --retry-connrefused -o /dev/null \"http://$IP:8080/healthz\"",
    ]
  }
}
