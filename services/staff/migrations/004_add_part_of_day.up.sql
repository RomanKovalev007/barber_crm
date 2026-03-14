ALTER TABLE schedule
    ADD COLUMN part_of_day TEXT NOT NULL DEFAULT 'am' CHECK (part_of_day IN ('am', 'pm'));
