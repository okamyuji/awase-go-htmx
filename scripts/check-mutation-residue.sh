#!/usr/bin/env bash
# ミューテーションテストの残骸が残っていれば失敗します。残骸は make mutation-clean で消します。
set -euo pipefail
cd "$(dirname "$0")/.."

found=0
work="$(scripts/mutation-workdir.sh)"
if [ -e "$work" ]; then
  echo "残骸があります: $work" >&2
  found=1
fi
shopt -s nullglob
for p in gremlins.json gremlins-*; do
  [ -e "$p" ] || continue
  echo "残骸があります: $PWD/$p" >&2
  found=1
done
if [ "$found" -ne 0 ]; then
  echo "make mutation-clean で消してください" >&2
  exit 1
fi
