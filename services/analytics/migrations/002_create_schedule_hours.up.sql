CREATE TABLE IF NOT EXISTS schedule_hours
(
    schedule_id String,
    barber_id   String,
    date        Date,
    start_time  String,
    end_time    String,
    is_deleted  UInt8,
    occurred_at DateTime
)
ENGINE = ReplacingMergeTree(occurred_at)
PARTITION BY toYYYYMM(date)
ORDER BY schedule_id
