#!/usr/bin/env bash
# ミューテーションテストの作業領域のパスを出力します。
# gremlins はモジュール全体を作業領域へ複製するため、モジュールの外に置きます。
set -euo pipefail
echo "${XDG_CACHE_HOME:-$HOME/.cache}/awase-mutation"
