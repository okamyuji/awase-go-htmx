#!/usr/bin/env bash
# gremlins の結果を判定します。KILLED と NOT VIABLE 以外の変異が1件でもあれば失敗します。
# ただし、NOT COVERED のうち、カバレッジのどのブロックにも入らない行の変異は除外します。
# Go のカバレッジはパッケージ水準の const と var を計測しないため、gremlins はそれらの変異を実行できず、
# テストを足しても KILLED にならないからです。関数の中の NOT COVERED は、これまでどおり失敗です。
# 使い方: mutation-judge.sh <gremlins の JSON> <go test -coverprofile の出力> <モジュールのパス>
set -euo pipefail

json="${1:?usage: mutation-judge.sh <gremlins.json> <cover.out> <module>}"
cover="${2:?cover.out}"
module="${3:?module}"

jq -r '"変異 \(.mutants_total) 件 / KILLED \(.mutants_killed) 件 / LIVED \(.mutants_lived) 件 / NOT COVERED \(.mutants_not_covered) 件 / NOT VIABLE \(.mutants_not_viable) 件"' "$json"

# カバレッジの各ブロックを「ファイル 開始行 終了行」の形にします。
blocks=$(sed -E -n "s#^${module}/([^:]+):([0-9]+)\.[0-9]+,([0-9]+)\.[0-9]+ .*#\1 \2 \3#p" "$cover")

failed=""
while IFS=$'\t' read -r status type file line col; do
  [ -n "$status" ] || continue
  if [ "$status" = "NOT COVERED" ] &&
    ! awk -v f="$file" -v l="$line" '$1 == f && $2 <= l && l <= $3 { found = 1 } END { exit !found }' <<<"$blocks"; then
    echo "計測できない行のため除外: $status $type $file:$line:$col"
    continue
  fi
  failed+="$status $type $file:$line:$col"$'\n'
done < <(jq -r '
  .files[] | .file_name as $f | .mutations[]
  | select(.status != "KILLED" and .status != "NOT VIABLE")
  | [.status, .type, $f, .line, .column] | @tsv' "$json")

if [ -n "$failed" ]; then
  echo "KILLED にならなかった変異があります" >&2
  printf '%s' "$failed" >&2
  exit 1
fi
echo "すべての変異が KILLED になりました"
