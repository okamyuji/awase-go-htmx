.PHONY: build test run gate mutation mutation-clean precommit-install

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o bin/awase ./cmd/awase

test:
	go test -race -shuffle=on -count=1 ./...

run:
	AWASE_DB_PATH=$${AWASE_DB_PATH:-data/awase.db} AWASE_COOKIE_SECURE=false go run ./cmd/awase

gate:
	./scripts/quality-gate.sh

# 変異を全件 KILLED にできているかを測ります。時間がかかるため品質ゲートには入れていません。
mutation:
	./scripts/mutation.sh

mutation-clean:
	rm -rf "$$(./scripts/mutation-workdir.sh)" gremlins.json gremlins-*

precommit-install:
	pre-commit install
