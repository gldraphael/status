# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

# gcc + musl-dev are required because pebble uses DataDog/zstd (CGO).
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o status .

# ── Runtime stage ──────────────────────────────────────────────────────────────
FROM alpine:3.21 AS certs

# Gather ca-certificates and tzdata for scratch image.
RUN apk add --no-cache ca-certificates tzdata

# ── Final stage (rootless scratch) ─────────────────────────────────────────────
FROM scratch

COPY --from=certs /etc/ssl/certs /etc/ssl/certs
COPY --from=certs /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=certs /etc/passwd /etc/passwd
COPY --from=certs /lib/ld-musl-x86_64.so.1 /lib/
COPY --from=certs /lib/libc.musl-x86_64.so.1 /lib/

COPY --from=builder /app/status /app/status

# Rootless: run as nonroot user (uid 65534).
USER 65534

WORKDIR /tmp

EXPOSE 8080

ENTRYPOINT ["/app/status"]
