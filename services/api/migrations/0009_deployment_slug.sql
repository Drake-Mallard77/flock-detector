-- Readable URLs for agency records.
--
-- Records were addressed only by UUID: /deployments/8a29341e-1c58-...
-- That URL matches nothing anyone would ever type or link, which defeats
-- the point of publishing them. /state/tn/nashville-metropolitan-police-
-- department is the same record at an address a person and a search engine
-- can both make sense of.

ALTER TABLE deployments ADD COLUMN slug TEXT;

-- Base slug is city + agency, which reads naturally and disambiguates the
-- many "<County> Sheriff's Office" names that repeat across a state.
--
-- Collisions get a numeric suffix by creation order rather than being
-- rejected. Nine records collide today, all of them genuine duplicates
-- (punctuation variants of the same agency), and a migration is the wrong
-- place to decide which of a pair to discard — that is a moderator's call.
-- A suffixed slug keeps every record reachable until someone makes it.
WITH base AS (
    SELECT id,
           state,
           created_at,
           trim(BOTH '-' FROM regexp_replace(
               lower(coalesce(city, '') || '-' || agency_name),
               '[^a-z0-9]+', '-', 'g'
           )) AS s
    FROM deployments
),
ranked AS (
    SELECT id,
           -- Guards against a record whose name is entirely punctuation
           -- producing an empty slug, which would collide with every other
           -- such record and make an unreachable URL.
           CASE WHEN s = '' THEN 'record' ELSE s END AS s,
           row_number() OVER (
               PARTITION BY state, CASE WHEN s = '' THEN 'record' ELSE s END
               ORDER BY created_at, id
           ) AS rn
    FROM base
)
UPDATE deployments d
SET slug = CASE WHEN r.rn = 1 THEN r.s ELSE r.s || '-' || r.rn END
FROM ranked r
WHERE d.id = r.id;

ALTER TABLE deployments ALTER COLUMN slug SET NOT NULL;

-- Unique per state, not globally: "springfield-police-department" exists in
-- more than one state and both deserve the readable form. The state is
-- already in the path.
CREATE UNIQUE INDEX deployments_state_slug_uq ON deployments (state, slug);

-- Slug generation lives in the database because two places insert
-- deployments — the API's submission endpoint and the importer's derive job
-- — and a rule duplicated in both drifts. Anything that inserts a record
-- gets the same slug it would have got anywhere else.
--
-- The suffix loop is what makes the column safely NOT NULL under
-- concurrency: it asks for the next free suffix rather than assuming the
-- base is available. The unique index remains the real guarantee; this just
-- means callers rarely hit it.
CREATE FUNCTION next_deployment_slug(p_state TEXT, p_city TEXT, p_agency TEXT)
RETURNS TEXT AS $$
DECLARE
    base TEXT;
    candidate TEXT;
    n INT := 1;
BEGIN
    base := trim(BOTH '-' FROM regexp_replace(
        lower(coalesce(p_city, '') || '-' || coalesce(p_agency, '')),
        '[^a-z0-9]+', '-', 'g'
    ));
    IF base = '' THEN
        base := 'record';
    END IF;

    candidate := base;
    WHILE EXISTS (
        SELECT 1 FROM deployments WHERE state = p_state AND slug = candidate
    ) LOOP
        n := n + 1;
        candidate := base || '-' || n;
    END LOOP;

    RETURN candidate;
END;
$$ LANGUAGE plpgsql;

-- Filled by trigger rather than by each caller.
--
-- Two code paths insert deployments today and tests insert directly, so
-- "remember to call next_deployment_slug()" is a rule that will be forgotten
-- — and with the column NOT NULL, forgetting it is a failed insert rather
-- than a missing slug. The trigger makes the column self-maintaining: any
-- INSERT that omits a slug gets a correct one.
--
-- An explicitly supplied slug is respected, so a moderator can rename a
-- record's URL later without the database overwriting the choice.
CREATE FUNCTION set_deployment_slug() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.slug IS NULL OR NEW.slug = '' THEN
        NEW.slug := next_deployment_slug(NEW.state, NEW.city, NEW.agency_name);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER deployments_set_slug
    BEFORE INSERT ON deployments
    FOR EACH ROW EXECUTE FUNCTION set_deployment_slug();
