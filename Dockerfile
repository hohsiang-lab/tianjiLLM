FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=arm64

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /tianji ./cmd/tianji

FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /tianji /usr/local/bin/tianji
COPY configs/ /app/configs/

WORKDIR /app

EXPOSE 4000

ENTRYPOINT ["tianji"]
CMD ["--config", "proxy_config.yaml"]
