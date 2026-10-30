# awase のビルドと実行のイメージです。テンプレート、静的ファイル、スキーマはバイナリに埋め込みます。
FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY web ./web

ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-arm64} go build -trimpath -ldflags="-s -w" -o /out/awase ./cmd/awase

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/awase /awase
EXPOSE 8080
ENTRYPOINT ["/awase"]
