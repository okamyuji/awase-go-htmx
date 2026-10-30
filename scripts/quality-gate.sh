#!/usr/bin/env bash
# pre-commit と手動から呼ぶ品質ゲートです。ミューテーションテストは時間がかかるため含めず、make mutation で実行します。
set -euo pipefail
cd "$(dirname "$0")/.."

echo "==> mutation residue"
scripts/check-mutation-residue.sh

echo "==> gofmt"
unformatted="$(gofmt -l .)"
if [ -n "$unformatted" ]; then
  echo "gofmt 未適用: $unformatted" >&2
  exit 1
fi

echo "==> go fix -diff"
# -diff は書き換えずに差分だけを出します。差分があれば非ゼロで終わるため、|| true で受けてから判定します。
fix_diff="$(go fix -diff ./... || true)"
if [ -n "$fix_diff" ]; then
  echo "go fix で近代化できる箇所があります。go fix ./... を実行してください" >&2
  echo "$fix_diff" >&2
  exit 1
fi

echo "==> go vet"
go vet ./...

echo "==> staticcheck"
staticcheck ./...

echo "==> golangci-lint"
golangci-lint run --timeout 5m ./...

echo "==> govulncheck"
govulncheck ./...

echo "==> go test"
go test -count=1 -shuffle=on -race -coverprofile=coverage.out -covermode=atomic ./...

echo "==> coverage"
total="$(go tool cover -func=coverage.out | awk '/^total:/ { sub(/%/, "", $3); print $3 }')"
echo "total ${total}%"
awk -v t="$total" 'BEGIN { exit (t + 0 >= 80) ? 0 : 1 }' || { echo "カバレッジが80%未満です" >&2; exit 1; }

echo "==> CRAP"
scripts/crap.sh coverage.out 15

echo "==> build"
go build -o /dev/null ./cmd/awase

echo "==> doclint"
go run -C tools/doclint . ../../docs

echo "==> gitleaks"
# 作業ツリー全体を見ると、git の管理外の secrets.auto.tfvars や terraform.tfstate を拾うため、
# コミット済みの履歴とステージ済みの差分だけを見ます。
gitleaks git . --redact --no-banner --config .gitleaks.toml
gitleaks git . --pre-commit --staged --redact --no-banner --config .gitleaks.toml

echo "all quality checks passed"
