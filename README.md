# Barber CRM

CRM-система для частных барберов: расписание, запись клиентов, аналитика.

Репозиторий с frontend: https://github.com/K4rens/Barber-CRM/tree/main

Поднятый удаленно сервис: http://132.243.16.248/

## Архитектура

```
                        ┌─────────────┐
          HTTP :8080    │             │
Client ─────────────── │ api-gateway │
                        │             │
                        └──────┬──────┘
                               │ gRPC
              ┌────────────────┼────────────────┐
              │                │                │               │
              ▼                ▼                ▼               ▼
         ┌─────────┐    ┌─────────┐    ┌───────────┐    ┌──────────┐
         │  staff  │    │ booking │    │ analytics │    │  client  │
         │  :50051 │    │  :50052 │    │   :50053  │    │  :50054  │
         └────┬────┘    └────┬────┘    └─────┬─────┘    └────┬─────┘
              │              │               │                │
         ┌────▼────┐    ┌────▼────┐    ┌─────▼─────┐    ┌────▼─────┐
         │Postgres │    │Postgres │    │ClickHouse │    │ Postgres │
         │staff_db │    │booking_d│    │ analytics │    │ client_db│
         └─────────┘    └─────────┘    └───────────┘    └──────────┘
              │              │
         ┌────▼────┐    ┌────▼────┐
         │  Redis  │    │  Redis  │
         └─────────┘    └─────────┘

         Kafka ──► analytics (consumer)
               ──► client    (consumer)
         staff ──► Kafka (producer: события бронирований)
         booking ─► Kafka (producer: события бронирований)

         Prometheus ◄── все сервисы (/metrics)
         Grafana    ◄── Prometheus
```

## Стек

| Слой | Технологии |
|---|---|
| Язык | Go 1.24 |
| Transport | gRPC, HTTP (chi v5) |
| Авторизация | JWT (golang-jwt/jwt/v5) |
| БД | PostgreSQL 16, ClickHouse 24.3 |
| Кэш | Redis 7 |
| Очереди | Kafka (KRaft, без ZooKeeper) |
| Метрики | Prometheus, Grafana |
| Конфиг | cleanenv (env-переменные) |
| Миграции | собственный мигратор в каждом сервисе |

## Структура репозитория

```
.
├── api/                        # Protobuf-определения + сгенерированный Go-код
│   └── proto/
│       ├── staff/v1/
│       ├── booking/v1/
│       ├── analytics/v1/
│       └── client/v1/
├── pkg/                        # Общие пакеты
│   ├── auth/                   # JWT: генерация, валидация, claims
│   ├── config/                 # Структуры конфигурации (cleanenv)
│   ├── logger/                 # slog-обёртка
│   ├── postgres/               # pgxpool
│   └── redis/                  # go-redis клиент
├── services/
│   ├── api-gateway/            # HTTP :8080 → gRPC прокси
│   ├── staff/                  # gRPC :50051, авторизация барберов, расписание
│   ├── booking/                # gRPC :50052, бронирования, слоты
│   ├── analytics/              # gRPC :50053, статистика из ClickHouse
│   └── client/                 # gRPC :50054, профили клиентов
├── infra/
│   ├── postgres/init.sql       # Инициализация баз данных
│   ├── prometheus/             # prometheus.yml
│   └── grafana/                # Provisioning: datasources, dashboards
└── docker-compose.yml
```

## Быстрый старт

```bash
docker-compose up -d
```

Поднимаются все сервисы, инфраструктура и запускаются миграции. После старта:

| Сервис | Адрес |
|---|---|
| API Gateway | http://localhost:8080 |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3000 (admin/admin) |
| ClickHouse HTTP | http://localhost:8123 |
| PostgreSQL | localhost:5433 |
| Kafka | localhost:9092 |

## API

Базовый путь: `/api/v1`

### Публичные (без авторизации)

```
GET  /barbers
GET  /barbers/{barber_id}/services
GET  /barbers/{barber_id}/free-slots
POST /bookings
```

### Авторизация

```
POST /auth/login
POST /auth/refresh
POST /auth/logout          [JWT]
```

### Staff (требуют JWT)

```
GET    /staff/barber

GET    /staff/services
POST   /staff/services
PUT    /staff/services/{service_id}
DELETE /staff/services/{service_id}

GET    /staff/schedule
PUT    /staff/schedule/{date}
DELETE /staff/schedule/{date}
GET    /staff/slots

POST   /staff/bookings
GET    /staff/bookings/{booking_id}
PUT    /staff/bookings/{booking_id}
PATCH  /staff/bookings/{booking_id}
DELETE /staff/bookings/{booking_id}

GET    /staff/clients
GET    /staff/clients/{client_id}
PUT    /staff/clients/{client_id}

GET    /staff/analytics
```

### Служебные

```
GET /health
GET /metrics
```

## Конфигурация

Все параметры передаются через переменные окружения.

### api-gateway

| Переменная | По умолчанию | Описание |
|---|---|---|
| `HTTP_PORT` | `:8080` | HTTP-порт |
| `JWT_SECRET` | `jwt-secret` | Секрет для валидации JWT |
| `STAFF_ADDR` | `localhost:50051` | Адрес staff-сервиса |
| `BOOKING_ADDR` | `localhost:50052` | Адрес booking-сервиса |
| `ANALYTICS_ADDR` | `localhost:50053` | Адрес analytics-сервиса |
| `CLIENT_ADDR` | `localhost:50054` | Адрес client-сервиса |

### staff

| Переменная | По умолчанию | Описание |
|---|---|---|
| `GRPC_PORT` | `:50051` | gRPC-порт |
| `JWT_SECRET` | `jwt-secret` | Секрет для выдачи JWT |
| `METRICS_PORT` | `:9091` | Порт Prometheus /metrics |
| `POSTGRES_HOST` | `db` | — |
| `POSTGRES_PORT` | `5432` | — |
| `POSTGRES_USER` | `postgres` | — |
| `POSTGRES_PASSWORD` | `postgres` | — |
| `POSTGRES_DB` | — | Имя базы данных |
| `REDIS_HOST` | `localhost` | — |
| `REDIS_PORT` | `6379` | — |
| `REDIS_TTL_MINUTE` | `43200` | TTL кэша в минутах (30 дней) |
| `KAFKA_BROKERS` | `localhost:9092` | — |

### booking

| Переменная | По умолчанию | Описание |
|---|---|---|
| `GRPC_PORT` | `:50051` | gRPC-порт |
| `JWT_SECRET` | `jwt-secret` | — |
| `STAFF_GRPC_ADDR` | `staff:50051` | Адрес staff-сервиса |
| `METRICS_PORT` | `:9092` | Порт Prometheus /metrics |
| `POSTGRES_*` | — | Аналогично staff |
| `REDIS_*` | — | Аналогично staff |
| `KAFKA_BROKERS` | `localhost:9092` | — |

### analytics

| Переменная | По умолчанию | Описание |
|---|---|---|
| `GRPC_PORT` | `:50052` | gRPC-порт |
| `METRICS_PORT` | `:9093` | Порт Prometheus /metrics |
| `CLICKHOUSE_HOST` | `localhost` | — |
| `CLICKHOUSE_PORT` | `9000` | Нативный протокол |
| `CLICKHOUSE_DATABASE` | `analytics` | — |
| `CLICKHOUSE_USERNAME` | `default` | — |
| `CLICKHOUSE_PASSWORD` | — | — |
| `KAFKA_BROKERS` | `localhost:9092` | — |

### client

| Переменная | По умолчанию | Описание |
|---|---|---|
| `GRPC_PORT` | `:50053` | gRPC-порт |
| `METRICS_PORT` | `:9094` | Порт Prometheus /metrics |
| `POSTGRES_*` | — | Аналогично staff |
| `KAFKA_BROKERS` | `localhost:9092` | — |

## Метрики и наблюдаемость

Каждый сервис отдаёт метрики на своём порту:

| Сервис | Endpoint | Метрики |
|---|---|---|
| api-gateway | `localhost:8080/metrics` | `http_requests_total`, `http_request_duration_seconds` |
| staff | `localhost:9091/metrics` | `grpc_server_*` (go-grpc-prometheus) |
| booking | `localhost:9092/metrics` | `grpc_server_*` |
| analytics | `localhost:9093/metrics` | `grpc_server_*` |
| client | `localhost:9094/metrics` | `grpc_server_*` |

Prometheus собирает метрики автоматически. Grafana доступна по `http://localhost:3000`, datasource Prometheus уже добавлен через provisioning.

Рекомендуемые дашборды для импорта (Dashboards → Import → ID):
- `10818` — gRPC метрики
- `13407` — Go runtime
