CREATE TABLE IF NOT EXISTS services
(
    id        UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    barber_id UUID    NOT NULL REFERENCES barbers(id) ON DELETE CASCADE,
    name      TEXT    NOT NULL,
    price     INTEGER NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE
)
