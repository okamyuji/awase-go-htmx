#!/usr/bin/env bash
# 全関数の CRAP = 複雑度^2 * (1 - カバレッジ)^3 + 複雑度 を計算し、しきい値以上があれば失敗します。
# 使い方: scripts/crap.sh <coverprofile> [しきい値]
set -euo pipefail

PROFILE="${1:?usage: crap.sh <coverprofile> [threshold]}"
THRESHOLD="${2:-15}"
GOCYCLO="${GOCYCLO:-go run github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0}"
cd "$(dirname "$0")/.."

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

go tool cover -func="$PROFILE" > "$TMP/funccov"
# shellcheck disable=SC2086
# tools/ は独立したモジュールなので対象から外します。
find . -path ./tools -prune -o -name '*.go' ! -name '*_test.go' -print0 | xargs -0 $GOCYCLO > "$TMP/cyclo"

# gocyclo は "(*T).Name"、cover は "Name" と書くので、名前ではなく「ファイル:宣言行」で突き合わせます。
awk -v threshold="$THRESHOLD" '
  NR == FNR {
    split($1, p, ":")
    sub(/^.*awase-go-htmx\//, "", p[1])
    cov = $3; sub(/%/, "", cov)
    covmap[p[1] ":" p[2]] = cov
    next
  }
  {
    split($4, loc, ":")
    sub(/^\.\//, "", loc[1])
    key = loc[1] ":" loc[2]
    cov = (key in covmap) ? covmap[key] : 0
    c = $1; v = cov / 100
    crap = c * c * (1 - v) ^ 3 + c
    if (crap >= threshold) {
      printf "%s:%s comp=%d cov=%.1f%% crap=%.2f OVER\n", loc[1], $3, c, cov, crap
      fail = 1
    }
    if (crap > max) max = crap
  }
  END {
    printf "最大 CRAP=%.2f (しきい値 %s 未満が合格)\n", max, threshold
    exit fail
  }
' "$TMP/funccov" "$TMP/cyclo"
