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

# 入力が壊れていると、すべての NOT COVERED が除外されて合格に化けるため、失敗にします。
expect_broken() {
  local name="$1" json="$2" cover="$3"
  if scripts/mutation-judge.sh "$json" "$cover" "$mod" >/dev/null 2>&1; then
    echo "NG: $name（終了コード 0、期待 1）" >&2
    fail=1
  fi
}
judge "NOT COVERED" 11 || true
: >"$dir/empty.out"
expect_broken "空のカバレッジは失敗" "$dir/r.json" "$dir/empty.out"
sed "s#^$mod/#other.test/m/#" "$dir/cover.out" >"$dir/cover.out.other"
expect_broken "モジュールが一致しないカバレッジは失敗" "$dir/r.json" "$dir/cover.out.other"
echo '{"mutants_total": 0}' >"$dir/nofiles.json"
expect_broken "files のない結果は失敗" "$dir/nofiles.json" "$dir/cover.out"

[ "$fail" = 0 ] && echo "mutation-judge の判定はすべて期待どおりです"
exit "$fail"
