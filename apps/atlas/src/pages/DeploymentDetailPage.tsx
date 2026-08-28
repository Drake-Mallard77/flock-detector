import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import StatusBadge from "../components/StatusBadge";
import { ApiError, getDeployment, type Deployment } from "../lib/api";

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
  const { id } = useParams<{ id: string }>();
  const [record, setRecord] = useState<Deployment | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;

    getDeployment(id)
      .then((d) => {
        if (!cancelled) setRecord(d);
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
  }, [id]);

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

      <p style={{ color: "var(--text-muted)", fontSize: "0.82rem", marginTop: "2rem" }}>
        Locations are shown at city level. This record describes a documented agency
        deployment, not the position of any individual camera.
      </p>
    </div>
  );
}
