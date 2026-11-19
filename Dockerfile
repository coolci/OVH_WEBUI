# syntax=docker/dockerfile:1
# OVH_WEBUI — 单镜像多阶段构建（嵌入前端 //go:embed + 纯 Go SQLite）
# 构建: docker build -t ovh-webui .

ARG GO_VERSION=1.26

FROM node:22-alpine AS web-build
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm ci --ignore-scripts
COPY web/index.html web/vite.config.ts web/tsconfig*.json web/tailwind.config.ts web/postcss.config.js web/components.json ./
COPY web/public ./public
COPY web/src ./src
ENV NODE_ENV=production
RUN npm run build

FROM golang:${GO_VERSION}-bookworm AS go-build
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=web-build /web/dist ./web
RUN VER=$(tr -d '[:space:]' < VERSION) && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -tags ui -trimpath \
      -ldflags="-s -w -X github.com/ovh-webui/server/internal/handlers.Version=${VER}" \
      -o /out/ovh-webui .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S ovh && adduser -S -G ovh -h /app ovh

WORKDIR /app
COPY --from=go-build /out/ovh-webui /app/ovh-webui
COPY server/docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/ovh-webui /app/docker-entrypoint.sh \
    && mkdir -p /data/cache /data/logs \
    && chown -R ovh:ovh /app /data

ENV PORT=19998 \
    DATA_DIR=/data \
    GIN_MODE=release \
    ENABLE_API_KEY_AUTH=true \
    TZ=Asia/Shanghai \
    OVH_ENV_FILE=/data/.env

VOLUME ["/data"]
EXPOSE 19998
USER ovh

HEALTHCHECK --interval=15s --timeout=5s --start-period=20s --retries=5 \
  CMD wget -qO- "http://127.0.0.1:${PORT}/health" >/dev/null || exit 1

ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["/app/ovh-webui"]
