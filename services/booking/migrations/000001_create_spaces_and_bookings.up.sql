CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE spaces (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    capacity INT NOT NULL CHECK (capacity > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE bookings (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    space_id BIGINT NOT NULL REFERENCES spaces (id),
    user_id BIGINT NOT NULL,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'cancelled')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (end_time > start_time),
    -- Last line of defence against double booking: two active bookings of the
    -- same space can never have overlapping time ranges, whatever the app does.
    CONSTRAINT bookings_no_overlap EXCLUDE USING gist (
        space_id WITH =,
        tstzrange(start_time, end_time) WITH &&
    ) WHERE (status = 'active')
);

CREATE INDEX bookings_user_id_idx ON bookings (user_id, start_time DESC);
