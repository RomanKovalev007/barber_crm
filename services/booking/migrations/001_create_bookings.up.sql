CREATE TABLE IF NOT EXISTS bookings (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    client_name TEXT        NOT NULL,
    barber_id   UUID        NOT NULL,
    serv_id     UUID        NOT NULL,
    date        DATE        NOT NULL,
    time_start  TIMESTAMPTZ NOT NULL,
    time_end    TIMESTAMPTZ NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bookings_barber_date ON bookings(barber_id, date);
CREATE INDEX idx_bookings_client ON bookings(client_name);
