# Karots Suite for a VPS, behind Nginx Proxy Manager (see compose.yml).
# Put the shop's Linux, server-mode binary from the Shop Builder next to this file
# as ./karots-suite, then: docker compose up -d --build
# No ports are exposed: Nginx Proxy Manager reaches it over a shared Docker network.
FROM alpine:3.20

# Root certificates (SSL to a database, the AI assistant) and time zones (the shop's dates).
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -H -u 10001 karots \
 && mkdir -p /data/backups && chown -R karots /data

# The Builder makes it executable; COPY keeps that (no BuildKit needed).
COPY karots-suite /app/karots-suite
WORKDIR /app
USER karots
ENV TZ=Asia/Colombo

HEALTHCHECK --interval=30s --timeout=5s --start-period=60s \
  CMD wget -qO- "http://127.0.0.1:${SERVER_PORT:-3000}/health" >/dev/null || exit 1

ENTRYPOINT ["/app/karots-suite"]
