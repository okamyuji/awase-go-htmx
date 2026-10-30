#!/usr/bin/env bash
# awase の国別遮断ルールを、ゾーンのカスタムルールのルールセットへ追加または削除します。
# このルールセットには AI Crawl Control が作るルールが同居するため、terraform でルールセットごと持たず、
# ルール単位の API で awase のルールだけを扱います。同じ説明のルールがあれば追加せずに式をそろえます。
# 使い方: cf-country-rule.sh add|remove <ゾーン名> <ホスト名>
set -euo pipefail

ACTION="${1:?usage: cf-country-rule.sh add|remove <zone> <hostname>}"
ZONE="${2:?zone}"
HOST="${3:?hostname}"
DESC="awase: 日本以外からの要求を遮断する"
EXPR="(http.host eq \"$HOST\" and not ip.src.country in {\"JP\"})"
API="${CF_API:-https://api.cloudflare.com/client/v4}"

# トークンは環境変数か、terraform の secrets.auto.tfvars から読みます。画面には出しません。
if [ -z "${CF_API_TOKEN:-}" ]; then
  tfvars="$(cd "$(dirname "$0")/../terraform" && pwd)/secrets.auto.tfvars"
  CF_API_TOKEN=$(sed -n 's/^[[:space:]]*cloudflare_api_token[[:space:]]*=[[:space:]]*"\(.*\)".*/\1/p' "$tfvars")
fi
[ -n "$CF_API_TOKEN" ] || { echo "Cloudflare の API トークンがありません" >&2; exit 1; }

api() {
  local method="$1" path="$2" body="${3:-}"
  local out
  out=$(curl -sS -X "$method" -H "Authorization: Bearer $CF_API_TOKEN" -H "Content-Type: application/json" ${body:+--data "$body"} "$API$path")
  if [ "$(jq -r '.success' <<<"$out")" != "true" ]; then
    echo "Cloudflare API の失敗: $method $path: $(jq -c '.errors' <<<"$out")" >&2
    return 1
  fi
  printf '%s' "$out"
}

zone_id=$(api GET "/zones?name=$ZONE" | jq -r '.result[0].id')
entry_file=$(mktemp)
trap 'rm -f "$entry_file"' EXIT
status=$(curl -sS -o "$entry_file" -w '%{http_code}' -H "Authorization: Bearer $CF_API_TOKEN" "$API/zones/$zone_id/rulesets/phases/http_request_firewall_custom/entrypoint")
entry=$(cat "$entry_file")
# 404 以外の失敗を「ルールセットなし」と扱うと、PUT が同居する AI Crawl Control のルールを消すため止めます。
case "$status" in
  200) ;;
  404) entry='{"success":false}' ;;
  *) echo "Cloudflare API の失敗: GET entrypoint: HTTP $status" >&2; exit 1 ;;
esac
ruleset_id=$(jq -r 'if .success then .result.id else empty end' <<<"$entry")
rule_id=$(jq -r --arg d "$DESC" 'if .success then (.result.rules[]? | select(.description == $d) | .id) else empty end' <<<"$entry")
rule=$(jq -nc --arg e "$EXPR" --arg d "$DESC" '{action: "block", expression: $e, description: $d, enabled: true}')

case "$ACTION" in
  add)
    if [ -z "$ruleset_id" ]; then
      api PUT "/zones/$zone_id/rulesets/phases/http_request_firewall_custom/entrypoint" "$(jq -nc --argjson r "$rule" '{rules: [$r]}')" >/dev/null
      echo "カスタムルールのルールセットを作り、awase のルールを追加しました"
    elif [ -z "$rule_id" ]; then
      api POST "/zones/$zone_id/rulesets/$ruleset_id/rules" "$rule" >/dev/null
      echo "awase のルールを追加しました"
    else
      api PATCH "/zones/$zone_id/rulesets/$ruleset_id/rules/$rule_id" "$rule" >/dev/null
      echo "awase のルールの式をそろえました"
    fi
    ;;
  remove)
    if [ -n "$rule_id" ]; then
      api DELETE "/zones/$zone_id/rulesets/$ruleset_id/rules/$rule_id" >/dev/null
      echo "awase のルールを削除しました"
    else
      echo "awase のルールはありません"
    fi
    ;;
  *)
    echo "usage: cf-country-rule.sh add|remove <zone> <hostname>" >&2
    exit 2
    ;;
esac
