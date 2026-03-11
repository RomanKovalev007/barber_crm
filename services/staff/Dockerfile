FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o staff   ./cmd/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o migrate ./cmd/migrate/main.go

FROM scratch

COPY --from=builder /app/staff   /staff
COPY --from=builder /app/migrate /migrate

ENTRYPOINT ["/staff"]

# Запуск основного сервиса:
#   docker run ... /staff
#
# Запуск миграций (init container):
#   docker run ... /migrate -direction up
