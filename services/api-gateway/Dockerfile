FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o api-gateway ./cmd/main.go

FROM scratch

COPY --from=builder /app/api-gateway /api-gateway

# Запуск:
#   docker run ... /api-gateway
ENTRYPOINT ["/api-gateway"]
