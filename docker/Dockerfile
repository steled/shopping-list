# syntax=docker/dockerfile:1

# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

ARG APP_VERSION=dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${APP_VERSION}" \
    -o /shopping-list .

# ── Final stage ───────────────────────────────────────────────────────────────
FROM alpine:3.23

ARG APP_VERSION=dev

LABEL org.opencontainers.image.title="Shopping List" \
      org.opencontainers.image.description="Lightweight shopping list web application" \
      org.opencontainers.image.vendor="DGOPS" \
      org.opencontainers.image.source="https://datagroup.ghe.com/DGOPS/cloud-ops.k8s.shopping-list" \
      org.opencontainers.image.version="${APP_VERSION}"

RUN addgroup -S -g 1001 appgroup \
 && adduser  -S -u 1001 -G appgroup appuser

RUN mkdir -p /data && chown appuser:appgroup /data

COPY --from=builder /shopping-list /usr/local/bin/shopping-list

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8080/healthz || exit 1

CMD ["/usr/local/bin/shopping-list"]
