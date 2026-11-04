# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM node:26.13.0-alpine3.24 AS web-build
WORKDIR /src/web
COPY apps/web/package.json apps/web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY apps/web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine3.24 AS backend-build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src/backend
COPY apps/backend/go.mod apps/backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY apps/backend/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.24.3 AS runtime
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 bikerskz \
    && adduser -S -D -H -u 10001 -G bikerskz bikerskz \
    && mkdir -p /var/lib/bikerskz/media \
    && chown -R 10001:10001 /var/lib/bikerskz
WORKDIR /app
COPY --from=backend-build --chown=10001:10001 /out/api /app/api
COPY --from=web-build --chown=10001:10001 /src/web/dist /app/web
ARG VCS_REF=unknown
LABEL org.opencontainers.image.source="https://github.com/unstppbl/bikers" \
    org.opencontainers.image.revision="$VCS_REF"
ENV PORT=8080 \
    WEB_DIST_DIR=/app/web \
    MEDIA_DIR=/var/lib/bikerskz/media \
    TZ=Asia/Almaty
USER 10001:10001
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD ["/app/api", "healthcheck"]
ENTRYPOINT ["/app/api"]
CMD ["serve"]
