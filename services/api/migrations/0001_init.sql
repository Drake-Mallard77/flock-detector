-- Initial schema: users, deployments (agency/contract-level records), and
-- camera_sightings (opt-in precise-pin layer). See docs/ARCHITECTURE.md.

CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    role          TEXT NOT NULL DEFAULT 'submitter'
                  CHECK (role IN ('submitter', 'moderator', 'admin')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE deployments (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agency_name        TEXT NOT NULL,
    city               TEXT NOT NULL,
    state              TEXT NOT NULL,
    county             TEXT,
    location           GEOGRAPHY(Point, 4326),
    documented_units   INTEGER,
    evidence_type       TEXT NOT NULL
                        CHECK (evidence_type IN (
                            'council_report', 'contract', 'invoice',
                            'news_article', 'foia_response', 'user_photo',
                            'osm_import'
                        )),
    source_links       TEXT[] NOT NULL DEFAULT '{}',
    status             TEXT NOT NULL DEFAULT 'under_review'
                        CHECK (status IN (
                            'confirmed', 'contract_found', 'under_review',
                            'disputed', 'removed'
                        )),
    notes              TEXT,
    created_by         UUID REFERENCES users(id),
    reviewed_by        UUID REFERENCES users(id),
    last_reviewed_at   TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX deployments_location_gix ON deployments USING GIST (location);
CREATE INDEX deployments_state_idx ON deployments (state);
CREATE INDEX deployments_status_idx ON deployments (status);

CREATE TABLE camera_sightings (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id  UUID REFERENCES deployments(id) ON DELETE SET NULL,
    location       GEOGRAPHY(Point, 4326) NOT NULL,
    direction      SMALLINT CHECK (direction BETWEEN 0 AND 359),
    camera_type    TEXT,
    photo_url      TEXT,
    source         TEXT NOT NULL DEFAULT 'user_submission'
                   CHECK (source IN ('user_submission', 'osm_import')),
    status         TEXT NOT NULL DEFAULT 'under_review'
                   CHECK (status IN ('confirmed', 'under_review', 'removed')),
    created_by     UUID REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX camera_sightings_location_gix ON camera_sightings USING GIST (location);
CREATE INDEX camera_sightings_deployment_idx ON camera_sightings (deployment_id);
