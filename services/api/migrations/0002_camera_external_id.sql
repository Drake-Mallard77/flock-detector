-- Supports the OSM/DeFlock importer (services/importer): external_id lets
-- re-running the import stay idempotent (upsert on conflict instead of
-- creating duplicate rows every run), and state lets the importer record
-- what it already knows from querying Overpass per-state, without having
-- to reverse-geocode every node just to answer "which state is this in".

ALTER TABLE camera_sightings
    ADD COLUMN external_id TEXT,
    ADD COLUMN state TEXT;

-- Partial unique index: external_id only needs to be unique when present
-- (user submissions never set it), and scoped implicitly to source since
-- in practice only osm_import rows populate it.
CREATE UNIQUE INDEX camera_sightings_external_id_uq
    ON camera_sightings (external_id)
    WHERE external_id IS NOT NULL;
