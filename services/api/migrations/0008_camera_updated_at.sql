-- Freshness needs a timestamp that moves when a re-import touches a row.
--
-- created_at doesn't: a weekly refresh mostly UPDATEs existing rows, so
-- created_at stays at the original import date and the data would look
-- ancient even while being refreshed correctly. Without this, the
-- /health/data check can't distinguish "refreshing fine" from "stopped
-- running weeks ago" — which is the entire point of the check.
ALTER TABLE camera_sightings
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Backfill from created_at so existing rows carry a truthful value rather
-- than all claiming to have been updated at migration time.
UPDATE camera_sightings SET updated_at = created_at;

-- Supports max(updated_at) on the freshness check without a full scan.
CREATE INDEX camera_sightings_updated_at_idx ON camera_sightings (updated_at DESC);
