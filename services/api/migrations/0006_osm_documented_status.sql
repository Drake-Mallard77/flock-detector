-- A distinct published state for records derived from OpenStreetMap.
--
-- Without it there was no honest way to publish these. 'confirmed' means a
-- moderator checked the record against a council report, contract, or FOIA
-- response — and the ~1,150 OSM-derived candidates have had no such check.
-- Publishing them as 'confirmed' on a site titled "Public Records Atlas"
-- would claim exactly the verification the project exists to provide.
--
-- 'osm_documented' says what is actually true: OpenStreetMap contributors
-- mapped these cameras and attributed them to this operator. That is real,
-- citable information; it just isn't a public record.
ALTER TABLE deployments
    DROP CONSTRAINT IF EXISTS deployments_status_check;

ALTER TABLE deployments
    ADD CONSTRAINT deployments_status_check
    CHECK (status IN (
        'confirmed',
        'contract_found',
        'osm_documented',
        'under_review',
        'disputed',
        'removed'
    ));
