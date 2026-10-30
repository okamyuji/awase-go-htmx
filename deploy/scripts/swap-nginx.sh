#!/usr/bin/env bash
# awase の nginx 設定と証明書を差し替えます。nginx -t が失敗したら3つのファイルを元に戻し、reload しません。
# 無効な設定を共有の conf.d に残すと、次に nginx が再起動したときに feedflow と boki3 も止まるためです。
# パスと nginx の呼び出しは環境変数で変えられ、ローカルの nginx コンテナでも同じ手順を試せます。
set -euo pipefail

NEW="${1:?usage: swap-nginx.sh <新しいファイルのディレクトリ>}"
CONF_DIR="${CONF_DIR:-/etc/feedflow/conf.d}"
TLS_DIR="${TLS_DIR:-/etc/feedflow/tls}"
NGINX="${NGINX:-sudo docker compose --project-directory /home/ec2-user/feedflow -f /home/ec2-user/feedflow/compose.yml -f /home/ec2-user/feedflow/compose.override.yml exec -T nginx nginx}"
SUDO="${SUDO-sudo}"

files=("$CONF_DIR/zz-awase.conf" "$TLS_DIR/awase.crt" "$TLS_DIR/awase.key")
# 鍵の控えを置くので、退避先も root の権限で作ります。
BAK="$($SUDO mktemp -d)"
trap '$SUDO rm -rf "$BAK"' EXIT

for f in "${files[@]}"; do
  if $SUDO test -f "$f"; then $SUDO cp -p "$f" "$BAK/"; fi
done

restore() {
  for f in "${files[@]}"; do
    b="$BAK/$(basename "$f")"
    if $SUDO test -f "$b"; then $SUDO cp -p "$b" "$f"; else $SUDO rm -f "$f"; fi
  done
}

$SUDO install -m 644 "$NEW/awase.crt" "$TLS_DIR/awase.crt"
$SUDO install -m 600 "$NEW/awase.key" "$TLS_DIR/awase.key"
$SUDO install -m 644 "$NEW/zz-awase.conf" "$CONF_DIR/zz-awase.conf"

# shellcheck disable=SC2086
if ! $NGINX -t; then
  restore
  echo "nginx -t に失敗したため awase の設定と証明書を元に戻しました" >&2
  exit 1
fi
# shellcheck disable=SC2086
$NGINX -s reload
