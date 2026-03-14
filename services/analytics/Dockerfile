FROM golang:1.24-alpine AS builder

WORKDIR /app

# Копируем общие модули (нужны replace-директивы в go.mod)
COPY api/    api/
COPY pkg/    pkg/

# Копируем сервис
COPY services/analytics/ services/analytics/

WORKDIR /app/services/analytics

RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o analytics ./cmd/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o migrate    ./cmd/migrate/main.go

FROM scratch

COPY --from=builder /app/services/analytics/analytics /analytics
COPY --from=builder /app/services/analytics/migrate   /migrate

# Запуск основного сервиса:
#   docker run ... /analytics
#
# Запуск миграций (init container):
#   docker run ... /migrate -direction up
ENTRYPOINT ["/analytics"]
