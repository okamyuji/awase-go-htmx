#!/usr/bin/env bash
# gremlins を流し、結果を scripts/mutation-judge.sh で判定します。
# 作業領域は正常終了でも中断でも消し、消せなければ失敗します。
set -euo pipefail
cd "$(dirname "$0")/.."

GREMLINS="${GREMLINS:-go run github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0}"
work="$(scripts/mutation-workdir.sh)"

cleanup() {
  rm -rf "$work"
  if [ -e "$work" ]; then
    echo "作業領域を消せませんでした: $work" >&2
    exit 1
  fi
}
trap cleanup EXIT
# gremlins は中断されると終了コード0で終わるため、シグナルを受けたらその先へ進ませません。
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

mkdir -p "$work"
export TMPDIR="$work"
out="$work/gremlins.json"

# テスト結果のキャッシュが当たると、gremlins が時間切れの基準にする初回の計測が短くなり、
# 変異がすべて時間切れになるため、キャッシュを使わせません。
# shellcheck disable=SC2086
GOFLAGS="${GOFLAGS:-} -count=1" $GREMLINS unleash --timeout-coefficient 10 -E "^tools/" -o "$out" .

# gremlins は中断されたときと結果が0件のときに JSON を書きません。
if [ ! -f "$out" ]; then
  echo "結果の JSON がありません。中断されたか、変異が0件です" >&2
  exit 1
fi
cover="$work/cover.out"
go test -count=1 -coverprofile="$cover" ./... >/dev/null
scripts/mutation-judge.sh "$out" "$cover" "$(go list -m)"
