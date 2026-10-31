#!/usr/bin/env bash
# mutation-judge.sh の判定を、作った gremlins の結果とカバレッジで確かめます。
set -euo pipefail
cd "$(dirname "$0")/.."

dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
mod=example.test/m
# a.go の 10〜12 行は関数の中（計測される行）、3 行はパッケージ水準の定数（計測されない行）です。
printf 'mode: set\n%s/a.go:10.2,12.3 2 1\n' "$mod" >"$dir/cover.out"

judge() {
  local status="$1" line="$2"
  jq -n --arg s "$status" --argjson l "$line" '{
    mutants_total: 1, mutants_killed: 0, mutants_lived: 0, mutants_not_covered: 0, mutants_not_viable: 0,
    files: [{file_name: "a.go", mutations: [{status: $s, type: "ARITHMETIC_BASE", line: $l, column: 5}]}]
  }' >"$dir/r.json"
  scripts/mutation-judge.sh "$dir/r.json" "$dir/cover.out" "$mod" >/dev/null 2>&1
}

fail=0
expect() {
  local want="$1" name="$2"
  shift 2
  local got=0
  judge "$@" || got=1
  if [ "$got" != "$want" ]; then
    echo "NG: $name（終了コード $got、期待 $want）" >&2
    fail=1
  fi
}

expect 0 "KILLED は合格" KILLED 11
expect 0 "計測されない行の NOT COVERED は除外" "NOT COVERED" 3
expect 1 "計測される行の NOT COVERED は失敗" "NOT COVERED" 11
expect 1 "LIVED は失敗" LIVED 11
expect 1 "計測されない行でも LIVED は失敗" LIVED 3

[ "$fail" = 0 ] && echo "mutation-judge の判定はすべて期待どおりです"
exit "$fail"
