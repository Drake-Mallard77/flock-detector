-- Shared rate-limit state.
--
-- The limiter was an in-memory map, which was correct when this targeted a
-- single VM. On Cloud Run with max_instance_count = 3 each instance kept its
-- own counters, so the effective limit was three times the intended one and
-- drifted with autoscaling. That matters because this is a data-integrity
-- control — the threat is someone flooding the queue with fabricated
-- submissions to dilute or discredit the dataset — not a performance knob.
--
-- Postgres rather than Redis: the database already exists, submissions are
-- rare enough that one extra round trip on a write path is irrelevant, and
-- adding a cache tier for this would be more moving parts than the problem
-- justifies.
CREATE TABLE rate_limits (
    key          TEXT PRIMARY KEY,
    window_start TIMESTAMPTZ NOT NULL,
    count        INTEGER     NOT NULL DEFAULT 0
);

-- Lets the periodic sweep find expired rows without scanning the table.
CREATE INDEX rate_limits_window_start_idx ON rate_limits (window_start);
