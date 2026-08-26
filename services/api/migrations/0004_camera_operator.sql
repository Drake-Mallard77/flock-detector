-- Who operates the camera, per OpenStreetMap's `operator` tag. Distinct
-- from `manufacturer`: manufacturer is who built it, operator is who runs
-- it, and conflating them would attribute a police deployment to a vendor.
--
-- Sparse by nature — only ~14% of ALPR nodes carry the tag — so this is a
-- lead for deriving agency-level deployment candidates, never a complete
-- picture of who operates what.
ALTER TABLE camera_sightings
    ADD COLUMN operator TEXT;

CREATE INDEX camera_sightings_operator_idx
    ON camera_sightings (operator)
    WHERE operator IS NOT NULL;
