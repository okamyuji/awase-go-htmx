#!/usr/bin/env bash
# awase を nginx から外します。設定を消して nginx -t と reload を済ませてから証明書を消します。
# 証明書だけが消えて設定が残ると、nginx は証明書を読めずに起動に失敗し、feedflow と boki3 も止まるためです。
# 何度実行しても成功するように書いています。
set -euo pipefail

CONF_DIR="${CONF_DIR:-/etc/feedflow/conf.d}"
TLS_DIR="${TLS_DIR:-/etc/feedflow/tls}"
NGINX="${NGINX:-sudo docker compose --project-directory /home/ec2-user/feedflow -f /home/ec2-user/feedflow/compose.yml -f /home/ec2-user/feedflow/compose.override.yml exec -T nginx nginx}"
SUDO="${SUDO-sudo}"

conf="$CONF_DIR/zz-awase.conf"
# 鍵の控えを置くので、退避先も root の権限で作ります。
BAK="$($SUDO mktemp -d)"
trap '$SUDO rm -rf "$BAK"' EXIT

if $SUDO test -f "$conf"; then
  $SUDO cp -p "$conf" "$BAK/"
  $SUDO rm -f "$conf"
fi
# shellcheck disable=SC2086
if ! $NGINX -t; then
  if $SUDO test -f "$BAK/zz-awase.conf"; then $SUDO cp -p "$BAK/zz-awase.conf" "$conf"; fi
  echo "nginx -t に失敗したため awase の設定を元に戻しました" >&2
  exit 1
fi
# shellcheck disable=SC2086
$NGINX -s reload
$SUDO rm -f "$TLS_DIR/awase.crt" "$TLS_DIR/awase.key"
