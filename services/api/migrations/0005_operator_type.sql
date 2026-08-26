-- Coarse classification of who runs a deployment, so the atlas can say
-- plainly what kind of claim each record makes. "Chicago Police Department"
-- and "The Home Depot" are both real ALPR deployments, but presenting them
-- identically in a public-records atlas blurs what the site asserts.
--
-- Derived by keyword matching (services/importer/internal/importer/
-- operator.go), which is fallible — hence 'unknown' rather than a forced
-- guess, and hence every derived record still requires human review.
ALTER TABLE deployments
    ADD COLUMN operator_type TEXT
        CHECK (operator_type IN ('law_enforcement', 'government', 'education', 'private', 'unknown'));

CREATE INDEX deployments_operator_type_idx
    ON deployments (operator_type)
    WHERE operator_type IS NOT NULL;
