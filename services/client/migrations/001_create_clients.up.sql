CREATE TABLE IF NOT EXISTS clients
(
    id           UUID        NOT NULL DEFAULT gen_random_uuid(),
    barber_id    TEXT        NOT NULL,
    phone        TEXT        NOT NULL,
    name         TEXT        NOT NULL,
    notes        TEXT        NOT NULL DEFAULT '',
    visits_count INT         NOT NULL DEFAULT 0,
    last_visit   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id),
    UNIQUE (barber_id, phone)
);

CREATE TABLE IF NOT EXISTS client_processed_events
(
    booking_id   TEXT        NOT NULL PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
