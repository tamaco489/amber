# syntax=docker/dockerfile:1

# ================================================================================
# Build
# ================================================================================
# ビルドは実行する環境のアーキテクチャで行い、Go のクロスコンパイルで TARGETARCH 向けのバイナリを作る
#   - QEMU で arm64 をエミュレートせずに済むため、amd64 の CI でも速い
FROM --platform=$BUILDPLATFORM golang:1.27.4 AS build

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api

# ================================================================================
# Runtime
# ================================================================================
# distroless の static (シェルもパッケージマネージャーも入っていない)
#   - nonroot のタグは、UID 65532 の非 root ユーザーで動く
#   - アプリはファイルを書かないため、ルートファイルシステムが読み取り専用でも動く
FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=build /out/api /api

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
