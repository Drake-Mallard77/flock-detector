import pg from "pg";
const { Pool } = pg;
export const pool = new Pool({ connectionString: process.env.DATABASE_URL, max: 10, idleTimeoutMillis: 30000, connectionTimeoutMillis: 10000 });

const migrations = [
`CREATE TABLE IF NOT EXISTS deployments (
 id BIGSERIAL PRIMARY KEY, city TEXT NOT NULL, state CHAR(2) NOT NULL, agency TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'Under review' CHECK (status IN ('Confirmed','Contract found','Under review')),
 cameras INTEGER, evidence TEXT NOT NULL, source_url TEXT NOT NULL, latitude DOUBLE PRECISION,
 longitude DOUBLE PRECISION, map_x DOUBLE PRECISION NOT NULL, map_y DOUBLE PRECISION NOT NULL,
 lifecycle TEXT NOT NULL DEFAULT 'active', reviewed_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
`CREATE INDEX IF NOT EXISTS deployments_state_idx ON deployments(state)`,
`CREATE INDEX IF NOT EXISTS deployments_geo_idx ON deployments(latitude, longitude)`,
`CREATE TABLE IF NOT EXISTS submissions (
 id BIGSERIAL PRIMARY KEY, city TEXT NOT NULL, state CHAR(2) NOT NULL, location TEXT NOT NULL,
 evidence_url TEXT NOT NULL, notes TEXT NOT NULL, latitude DOUBLE PRECISION, longitude DOUBLE PRECISION,
 status TEXT NOT NULL DEFAULT 'pending', reviewer_note TEXT, reviewed_by TEXT, reviewed_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
`CREATE INDEX IF NOT EXISTS submissions_status_idx ON submissions(status)`,
`CREATE TABLE IF NOT EXISTS corrections (
 id BIGSERIAL PRIMARY KEY, deployment_id BIGINT NOT NULL REFERENCES deployments(id), correction_type TEXT NOT NULL,
 details TEXT NOT NULL, evidence_url TEXT, status TEXT NOT NULL DEFAULT 'pending', reviewer_note TEXT,
 reviewed_by TEXT, reviewed_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
`CREATE INDEX IF NOT EXISTS corrections_status_idx ON corrections(status)`,
`CREATE TABLE IF NOT EXISTS ingest_jobs (
 id BIGSERIAL PRIMARY KEY, source_url TEXT NOT NULL, source_type TEXT NOT NULL, agency TEXT,
 status TEXT NOT NULL DEFAULT 'queued', records_found INTEGER NOT NULL DEFAULT 0,
 imported_count INTEGER NOT NULL DEFAULT 0, notes TEXT, created_by TEXT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`
];

const seeds = [
 ["Indio","CA","Indio Police Department","Confirmed",40,"Council staff report and agreement","https://indio.civicweb.net/document/73480/Flock%20Agreement.pdf",33.7206,-116.2156,15,65,"2026-08-18"],
 ["Denver","CO","City and County of Denver","Under review",null,"Public-records request and responsive documents","https://www.muckrock.com/foi/united-states-of-america-10/request-for-information-regarding-flock-safety-camera-installations-city-and-county-of-denver-195574/",39.7392,-104.9903,36,48,"2026-08-14"],
 ["Pawtucket","RI","Pawtucket Police Department","Contract found",null,"Published procurement reporting","https://www.wired.com/story/the-cop-who-took-on-flock/",41.8787,-71.3826,91,31,"2026-08-18"],
 ["Evanston","IL","Evanston Police Department","Under review",19,"Public notices document deactivation/removal","https://www.ilsos.gov/news/2025/august/250825d1.pdf",42.0451,-87.6877,64,38,"2026-08-12"],
 ["Longmont","CO","Longmont Public Safety","Under review",null,"Contract status reported; physical status uncertain","https://www.theguardian.com/us-news/2026/aug/20/flock-cameras-surveillance",40.1672,-105.1019,36,45,"2026-08-20"]
];

export async function migrate() {
  const client = await pool.connect();
  try {
    await client.query("BEGIN");
    for (const sql of migrations) await client.query(sql);
    const { rows } = await client.query("SELECT COUNT(*)::int AS count FROM deployments");
    if (!rows[0].count) for (const row of seeds) await client.query("INSERT INTO deployments (city,state,agency,status,cameras,evidence,source_url,latitude,longitude,map_x,map_y,reviewed_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)", row);
    await client.query("COMMIT");
  } catch (error) { await client.query("ROLLBACK"); throw error; } finally { client.release(); }
}
