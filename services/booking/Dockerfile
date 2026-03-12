FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o booking   ./cmd/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o migrate ./cmd/migrate/main.go

FROM scratch

COPY --from=builder /app/booking   /booking
COPY --from=builder /app/migrate /migrate

ENTRYPOINT ["/booking"]


