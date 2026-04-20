# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /app/bin/car-payment-gateway \
    ./cmd/api/...

# ── Runtime stage ─────────────────────────────────────────────────────────────
# Distroless produces a minimal image with no shell, reducing attack surface.
FROM gcr.io/distroless/static-debian12

COPY --from=builder /app/bin/car-payment-gateway /car-payment-gateway

EXPOSE 8080

ENTRYPOINT ["/car-payment-gateway"]
