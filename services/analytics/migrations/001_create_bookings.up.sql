CREATE TABLE IF NOT EXISTS bookings
(
    booking_id   String,
    barber_id    String,
    client_phone String,
    client_name  String,
    service_id   String,
    service_name String,
    price        Int32,
    start_time   DateTime,
    end_time     DateTime,
    status       Enum8('pending' = 1, 'completed' = 2, 'cancelled' = 3, 'no_show' = 4),
    occurred_at  DateTime
)
ENGINE = ReplacingMergeTree(occurred_at)
PARTITION BY toYYYYMM(start_time)
ORDER BY booking_id
