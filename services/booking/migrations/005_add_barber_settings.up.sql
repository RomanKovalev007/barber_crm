CREATE TABLE IF NOT EXISTS barber_settings (
    barber_id             UUID        PRIMARY KEY,
    compact_slots_enabled BOOLEAN     NOT NULL DEFAULT FALSE,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
