CREATE TABLE IF NOT EXISTS schedule
(
    id         UUID NOT NULL DEFAULT gen_random_uuid(),
    barber_id  UUID NOT NULL REFERENCES barbers(id) ON DELETE CASCADE,
    date       DATE NOT NULL,
    start_time TIME,
    end_time   TIME,
    PRIMARY KEY (id),
    UNIQUE (barber_id, date)
)
