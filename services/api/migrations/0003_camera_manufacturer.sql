-- The importer originally captured only manufacturer="Flock Safety" nodes.
-- Measured against OSM, that missed ~10% of documented ALPR cameras (DC: 86
-- Flock out of 95 total, the rest Motorola Solutions, Genetec, Leonardo).
-- Storing the manufacturer lets the atlas cover ALPR surveillance generally
-- while still letting readers filter to Flock specifically.

ALTER TABLE camera_sightings
    ADD COLUMN manufacturer TEXT;

-- Existing rows all came from the Flock-only import.
UPDATE camera_sightings
SET manufacturer = 'Flock Safety'
WHERE source = 'osm_import' AND manufacturer IS NULL;

CREATE INDEX camera_sightings_manufacturer_idx ON camera_sightings (manufacturer);
