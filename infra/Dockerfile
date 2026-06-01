# Watchclaw control-plane — isolated container image.
# Multi-stage: build the Go binary, ship a minimal non-root Alpine image.
# Dashboard + Cytoscape + topology are go:embed-ed, so the binary is self-contained.

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY control-plane/go.mod control-plane/go.sum ./
RUN go mod download
COPY control-plane/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/watchclaw-cp .

FROM alpine:3.20
RUN adduser -D -u 10001 watchclaw && mkdir -p /data && chown watchclaw /data
COPY --from=build /out/watchclaw-cp /usr/local/bin/watchclaw-cp
# Optional live-progress file (mounted read-only via compose); harmless if absent.
USER watchclaw
EXPOSE 8787
ENV WATCHCLAW_ADDR=:8787 \
    WATCHCLAW_DB=/data/watchclaw.db \
    WATCHCLAW_PROGRESS=/progress/LIVE-PROGRESS.md
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/watchclaw-cp"]
