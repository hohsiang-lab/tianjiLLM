FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH
ARG CODEX_CLIENT_VERSION

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN version="${CODEX_CLIENT_VERSION:-$(tr -d '[:space:]' < .codex-client-version)}" && \
    echo "${version}" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' || \
      { echo "invalid CODEX_CLIENT_VERSION: ${version}" >&2; exit 1; } && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH:-$(go env GOARCH)} \
      go build \
        -ldflags "-X github.com/praxisllmlab/tianjiLLM/internal/config.DefaultOpenAICodexBackendClientVersion=${version}" \
        -o /tianji \
        ./cmd/tianji

FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /tianji /usr/local/bin/tianji
COPY configs/ /app/configs/

WORKDIR /app

EXPOSE 4000

ENTRYPOINT ["tianji"]
CMD ["--config", "proxy_config.yaml"]
