import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import StatusBadge from "../components/StatusBadge";
import { listDeployments, type Deployment, type DeploymentStatus } from "../lib/api";

const TABS: Array<{ value: DeploymentStatus | "all"; label: string }> = [
  { value: "all", label: "All records" },
  { value: "confirmed", label: "Confirmed" },
  { value: "contract_found", label: "Contract found" },
  { value: "under_review", label: "Under review" },
];

const EVIDENCE_LABELS: Record<string, string> = {
  council_report: "Council report",
  contract: "Contract / agreement",
  invoice: "Invoice",
  news_article: "News article",
  foia_response: "FOIA response",
  user_photo: "User photo",
  osm_import: "OpenStreetMap",
};

export default function DeploymentsPage() {
  const [status, setStatus] = useState<DeploymentStatus | "all">("all");
  const [search, setSearch] = useState("");
  const [rows, setRows] = useState<Deployment[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Search is sent to the API rather than applied to the fetched page.
  // Filtering client-side only ever searched the rows already loaded (one
  // page, 50 by default), so any match past that was invisible and the
  // search looked broken. Debounced so typing doesn't fire a request per
  // keystroke.
  const [debouncedSearch, setDebouncedSearch] = useState("");
  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedSearch(search.trim()), 300);
    return () => window.clearTimeout(timer);
  }, [search]);

  useEffect(() => {
    let cancelled = false;
    setRows(null);
    setError(null);

    listDeployments({
      ...(status === "all" ? {} : { status }),
      ...(debouncedSearch ? { q: debouncedSearch } : {}),
    })
      .then((data) => {
        if (!cancelled) setRows(data);
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Could not load records");
        }
      });

    return () => {
      cancelled = true;
    };
  }, [status, debouncedSearch]);

  const visible = rows;

  return (
    <div className="page">
      <h1>Deployments</h1>
      <p className="lede">
        Search public records and reviewed reports. Every published record shows what we know —
        and what we don't.
      </p>

      <div className="filters">
        {TABS.map((tab) => (
          <button
            key={tab.value}
            aria-pressed={status === tab.value}
            onClick={() => setStatus(tab.value)}
          >
            {tab.label}
          </button>
        ))}
        <input
          type="search"
          placeholder="Filter by agency, city, or state…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          aria-label="Filter records"
          style={{ marginLeft: "auto", minWidth: "16rem" }}
        />
      </div>

      {error && <div className="notice error">{error}</div>}

      {!error && rows === null && <p className="state">Loading records…</p>}

      {visible && visible.length === 0 && (
        <div className="state">
          {debouncedSearch || status !== "all" ? (
            <p>
              No records match that search.{" "}
              <Link to="/submit">Submit a sighting</Link> if you have documentation.
            </p>
          ) : (
            <>
              <p style={{ marginTop: 0 }}>
                <strong>No deployment records have been published yet.</strong>
              </p>
              <p>
                Deployment records are agency-level and must be backed by public records — a
                council report, contract, or FOIA response — so they're added and reviewed by
                hand rather than imported.
              </p>
              <p>
                The <Link to="/">map</Link> already shows tens of thousands of individual
                camera locations from OpenStreetMap. Those are a separate layer: a camera pin
                is not a claim about which agency operates it. See{" "}
                <Link to="/methodology">Methodology</Link> for why the two are kept apart, or{" "}
                <Link to="/submit">submit a record</Link> if you have documentation.
              </p>
            </>
          )}
        </div>
      )}

      {visible && visible.length > 0 && (
        <div className="table-scroll">
          <table className="records">
            <thead>
              <tr>
                <th>Location</th>
                <th>Agency</th>
                <th>Documented units</th>
                <th>Evidence</th>
                <th>Status</th>
                <th>Last reviewed</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((d) => (
                <tr key={d.id}>
                  <td>
                    <Link to={`/deployments/${d.id}`}>
                      {d.city}, {d.state}
                    </Link>
                  </td>
                  <td>{d.agency_name}</td>
                  <td>
                    {d.documented_units ?? (
                      <span className="unknown" title="Not documented in available records">
                        ?
                      </span>
                    )}
                  </td>
                  <td>{EVIDENCE_LABELS[d.evidence_type] ?? d.evidence_type}</td>
                  <td>
                    <StatusBadge status={d.status} />
                  </td>
                  <td>
                    {d.last_reviewed_at ? (
                      new Date(d.last_reviewed_at).toLocaleDateString()
                    ) : (
                      <span className="unknown">Not yet reviewed</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
