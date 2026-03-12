ALTER TABLE bookings RENAME COLUMN service_id TO serv_id;

ALTER TABLE bookings
    DROP COLUMN IF EXISTS client_phone,
    DROP COLUMN IF EXISTS service_name;
