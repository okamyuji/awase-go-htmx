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

# カバレッジの各ブロックを「ファイル 開始行 終了行」の形にします。モジュールのパスは文字列として比べます。
blocks=$(awk -v p="$module/" 'index($0, p) == 1 {
  rest = substr($0, length(p) + 1)
  split(rest, at, ":")
  split(at[2], pos, "[.,]")
  print at[1], pos[1], pos[3]
}' "$cover")
# ブロックが1つもないと、すべての NOT COVERED が除外されて合格に化けるため止めます。
if [ -z "$blocks" ]; then
  echo "カバレッジに $module のブロックがありません: $cover" >&2
  exit 1
fi
# gremlins の mutants_total は KILLED、LIVED、NOT VIABLE の件数です（report.go の fileReport）。
# 明細がこれと食い違うと、明細だけを見る判定は生き残りを見落とすため止めます。
if [ "$(jq '.mutants_total as $t | [.files[].mutations[] | select(.status == "KILLED" or .status == "LIVED" or .status == "NOT VIABLE")] | length == $t' "$json")" != true ]; then
  echo "結果の明細が mutants_total と一致しません: $json" >&2
  exit 1
fi
# プロセス置換の中の jq の失敗は set -e で止まらないため、先に変数へ取り込みます。
survivors=$(jq -r '
  .files[] | .file_name as $f | .mutations[]
  | select(.status != "KILLED" and .status != "NOT VIABLE")
  | [.status, .type, $f, .line, .column] | @tsv' "$json")

failed=""
while IFS=$'\t' read -r status type file line col; do
  [ -n "$status" ] || continue
  if [ "$status" = "NOT COVERED" ] &&
    ! awk -v f="$file" -v l="$line" '$1 == f && $2 <= l && l <= $3 { found = 1 } END { exit !found }' <<<"$blocks"; then
    echo "計測できない行のため除外: $status $type $file:$line:$col"
    continue
  fi
  failed+="$status $type $file:$line:$col"$'\n'
done <<<"$survivors"

if [ -n "$failed" ]; then
  echo "KILLED にならなかった変異があります" >&2
  printf '%s' "$failed" >&2
  exit 1
fi
echo "すべての変異が KILLED になりました"
