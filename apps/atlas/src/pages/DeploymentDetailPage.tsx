import { lazy, Suspense, useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";

// Lazy, because RecordMap pulls in Leaflet. Imported eagerly it landed
// Leaflet in the main bundle — 82KB to 127KB gzipped on every page,
// including the ones that never show a map. That is exactly what the
// lazy MapPage import in App.tsx exists to avoid.
const RecordMap = lazy(() => import("../components/RecordMap"));
import StatusBadge from "../components/StatusBadge";
import {
  ApiError,
  getDeployment,
  getDeploymentBySlug,
  listDeploymentCameras,
  recordPath,
  type Deployment,
  type DeploymentCameras,
} from "../lib/api";

const EVIDENCE_LABELS: Record<string, string> = {
  council_report: "Council report",
  contract: "Contract / agreement",
  invoice: "Invoice",
  news_article: "News article",
  foia_response: "FOIA response",
  user_photo: "User photo",
  osm_import: "OpenStreetMap",
};

export default function DeploymentDetailPage() {
  // Serves both routes: /deployments/:id (the original, permanent form) and
  // /state/:code/:slug (readable, and what the sitemap advertises).
  const { id, code, slug } = useParams<{ id: string; code: string; slug: string }>();
  const navigate = useNavigate();
  const [record, setRecord] = useState<Deployment | null>(null);
  const [cameras, setCameras] = useState<DeploymentCameras | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const load = id
      ? getDeployment(id)
      : code && slug
        ? getDeploymentBySlug(code, slug)
        : null;
    if (!load) return;

    load
      .then((d) => {
        if (cancelled) return;
        setRecord(d);
        // Arrived by UUID: send the reader to the canonical address.
        // replace, not push, so Back returns wherever they came from rather
        // than to the UUID URL that immediately redirects again.
        if (id) navigate(recordPath(d), { replace: true });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(
          err instanceof ApiError && err.status === 404
            ? "No record with that ID."
            : err instanceof Error
              ? err.message
              : "Could not load this record",
        );
      });

    return () => {
      cancelled = true;
    };
  }, [id, code, slug, navigate]);

  // Keyed on the resolved record rather than the URL, because the readable
  // route carries a slug and the cameras endpoint takes an id.
  //
  // Allowed to fail quietly: the record is the page, its map is an
  // enhancement, and a failure here shouldn't replace a readable record
  // with an error.
  useEffect(() => {
    if (!record) return;
    let cancelled = false;

    listDeploymentCameras(record.id)
      .then((c) => {
        if (!cancelled) setCameras(c);
      })
      .catch(() => {
        if (!cancelled) setCameras({ linked: 0, cameras: [] });
      });

    return () => {
      cancelled = true;
    };
  }, [record]);

  if (error) {
    return (
      <div className="page">
        <div className="notice error">{error}</div>
        <p className="back-link">
          <Link to="/deployments">← All deployments</Link>
        </p>
      </div>
    );
  }

  if (!record) {
    return (
      <div className="page">
        <p className="state">Loading record…</p>
      </div>
    );
  }

  return (
    <div className="page">
      <p className="back-link">
        <Link to="/deployments">← All deployments</Link>
      </p>

      <h1>
        {record.city}, {record.state}
      </h1>
      <p className="lede">
        {record.agency_name}
        {record.county ? ` · ${record.county} County` : ""}
      </p>

      {/* Placed above the details rather than below: where the cameras are
          is the first thing most readers want, and burying it under a
          definition list means most never scroll to it. Renders only once
          loaded, so the page doesn't reflow under someone mid-read. */}
      {cameras && (
        <Suspense fallback={null}>
          <RecordMap data={cameras} agency={record.agency_name} />
        </Suspense>
      )}

      <dl className="prose">
        <div style={{ marginBottom: "1rem" }}>
          <dt style={{ fontWeight: 600 }}>Status</dt>
          <dd style={{ margin: "0.2rem 0 0" }}>
            <StatusBadge status={record.status} />
          </dd>
        </div>

        <div style={{ marginBottom: "1rem" }}>
          <dt style={{ fontWeight: 600 }}>Documented units</dt>
          <dd style={{ margin: "0.2rem 0 0" }}>
            {record.documented_units ?? (
              <span className="unknown">
                Not documented in the records reviewed so far
              </span>
            )}
          </dd>
        </div>

        <div style={{ marginBottom: "1rem" }}>
          <dt style={{ fontWeight: 600 }}>Evidence type</dt>
          <dd style={{ margin: "0.2rem 0 0" }}>
            {EVIDENCE_LABELS[record.evidence_type] ?? record.evidence_type}
          </dd>
        </div>

        <div style={{ marginBottom: "1rem" }}>
          <dt style={{ fontWeight: 600 }}>Sources</dt>
          <dd style={{ margin: "0.2rem 0 0" }}>
            {record.source_links.length === 0 ? (
              <span className="unknown">No source links recorded</span>
            ) : (
              <ul style={{ margin: 0, paddingLeft: "1.1rem" }}>
                {record.source_links.map((link) => (
                  <li key={link}>
                    <a href={link} target="_blank" rel="noopener noreferrer">
                      {link}
                    </a>
                  </li>
                ))}
              </ul>
            )}
          </dd>
        </div>

        {record.notes && (
          <div style={{ marginBottom: "1rem" }}>
            <dt style={{ fontWeight: 600 }}>Notes</dt>
            <dd style={{ margin: "0.2rem 0 0" }}>{record.notes}</dd>
          </div>
        )}

        <div>
          <dt style={{ fontWeight: 600 }}>Last reviewed</dt>
          <dd style={{ margin: "0.2rem 0 0" }}>
            {record.last_reviewed_at ? (
              new Date(record.last_reviewed_at).toLocaleString()
            ) : (
              <span className="unknown">Not yet reviewed by a moderator</span>
            )}
          </dd>
        </div>
      </dl>

      {/* Rewritten now that a camera map sits above it. The old wording —
          "not the position of any individual camera" — was true of the
          record alone and became a contradiction next to a map of
          individual cameras. The two really are different claims, made on
          different evidence, and saying so is the point rather than a
          caveat. */}
      <p style={{ color: "var(--text-muted)", fontSize: "0.82rem", marginTop: "2rem" }}>
        This record describes a documented agency deployment and is placed at
        city level. Any camera locations mapped above are separate
        contributions to OpenStreetMap, attributed to this agency by their
        operator tag — they are not a claim by this project about where a
        given camera is, and they are not a complete list of what the agency
        operates.
      </p>
    </div>
  );
}
