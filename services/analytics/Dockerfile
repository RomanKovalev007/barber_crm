FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o analytics ./cmd/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o migrate    ./cmd/migrate/main.go

FROM scratch

COPY --from=builder /app/analytics /analytics
COPY --from=builder /app/migrate   /migrate

# Запуск основного сервиса:
#   docker run ... /analytics
#
# Запуск миграций (init container):
#   docker run ... /migrate -direction up
ENTRYPOINT ["/analytics"]
