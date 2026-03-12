CREATE UNIQUE INDEX IF NOT EXISTS bookings_barber_time_start_active_idx
    ON bookings (barber_id, time_start)
    WHERE status NOT IN ('cancelled', 'no_show');
