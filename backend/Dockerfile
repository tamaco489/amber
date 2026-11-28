# syntax=docker/dockerfile:1

# ベースイメージは、タグとダイジェスト (マルチアーキテクチャのインデックス) で固定する
#   - タグを付け替えられても、別のイメージを読み込まないため
#   - 上げるのは Dependabot (docker) の PR。golang を上げたら .tool-versions と go.mod の版もそろえる

# ================================================================================
# Build
# ================================================================================
# ビルドは実行する環境のアーキテクチャで行い、Go のクロスコンパイルで TARGETARCH 向けのバイナリを作る
#   - QEMU で arm64 をエミュレートせずに済むため、amd64 の CI でも速い
FROM --platform=$BUILDPLATFORM golang:1.27.4@sha256:5f1b98fa02fe396ebce2a0227a0e08309e11792915b9dd2e1f71053d7e7e6666 AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd

# アプリが読み込む internal のパッケージだけを入れる
#   - 通知の Lambda だけが使うパッケージ (AWS の SDK に依存する) を変えても、イメージの層を作り直さないため
#   - アプリが読み込むパッケージを足したら、ここにも足す (足りなければビルドが失敗する)
COPY internal/server ./internal/server
COPY internal/health ./internal/health
COPY internal/checkout ./internal/checkout

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api

# ================================================================================
# Runtime
# ================================================================================
# distroless の static (シェルもパッケージマネージャーも入っていない)
#   - nonroot のタグは、UID 65532 の非 root ユーザーで動く
#   - アプリはファイルを書かないため、ルートファイルシステムが読み取り専用でも動く
FROM gcr.io/distroless/static-debian13:nonroot@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5

COPY --from=build /out/api /api

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
