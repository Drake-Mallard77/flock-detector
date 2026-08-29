-- An audit trail for record decisions.
--
-- deployments carries reviewed_by and last_reviewed_at, but only ever the
-- most recent values: confirming a record, then disputing it, then
-- confirming it again leaves no trace of the first two. On a project whose
-- claim is that every published record is traceable, "who decided this, on
-- what evidence, and when" should not be answerable only for the latest
-- decision.
--
-- Camera additions need no equivalent. The importer only ever inserts or
-- updates, so camera_sightings.created_at already records when each camera
-- first appeared and nothing overwrites it.
CREATE TABLE deployment_events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,

    -- What happened: reviewed, verified, or merged.
    kind TEXT NOT NULL CHECK (kind IN ('reviewed', 'verified', 'merged')),

    -- Status before and after. from_status is nullable because the first
    -- event for a record predates any tracked transition.
    from_status TEXT,
    to_status   TEXT NOT NULL,

    -- The evidence a verification rested on, captured at the time. Copied
    -- rather than referenced: a later edit to the record should not silently
    -- rewrite what a past decision was based on.
    evidence_type TEXT,
    source_links  TEXT[] NOT NULL DEFAULT '{}',

    -- Free text for a merge target or a moderator's reasoning.
    note TEXT,

    actor_id   UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The two ways this gets read: one record's history, and the site-wide
-- recent-activity feed.
CREATE INDEX deployment_events_deployment_idx
    ON deployment_events (deployment_id, created_at DESC);
CREATE INDEX deployment_events_recent_idx
    ON deployment_events (created_at DESC);
