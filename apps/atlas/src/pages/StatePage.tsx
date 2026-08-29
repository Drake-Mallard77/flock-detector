import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";

import StatusBadge from "../components/StatusBadge";
import {
  listDeployments,
  listStateStats,
  recordPath,
  type Deployment,
  type StateStat,
} from "../lib/api";
import { stateName } from "../lib/states";

/**
 * Every published record in one state.
 *
 * This is the page a search engine can rank and a reader can be sent —
 * "ALPR cameras in Tennessee" — which the UUID-addressed record pages never
 * could be.
 */
export default function StatePage() {
  const { code } = useParams<{ code: string }>();
  const state = (code ?? "").toUpperCase();

  const [records, setRecords] = useState<Deployment[] | null>(null);
  const [stat, setStat] = useState<StateStat | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!state) return;
    let cancelled = false;

    listDeployments({ state, limit: 200 })
      .then((d) => {
        if (!cancelled) setRecords(d);
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Could not load records");
        }
      });

    // Totals come from the index rather than being counted from the page's
    // own rows: the list endpoint is paginated, so counting what's on screen
    // would understate any state with more records than one page.
    listStateStats()
      .then((all) => {
        if (!cancelled) setStat(all.find((s) => s.state === state) ?? null);
      })
      .catch(() => {
        /* Totals are supporting detail; the records are the page. */
      });

    return () => {
      cancelled = true;
    };
  }, [state]);

  // Only published records are shown. under_review candidates naming real
  // agencies shouldn't appear on a page framed as what's documented in a
  // place — the same judgement the sitemap and the state totals make.
  const published = (records ?? []).filter((r) =>
    ["confirmed", "contract_found", "osm_documented"].includes(r.status),
  );

  return (
    <div className="page">
      <p className="back-link">
        <Link to="/states">← All states</Link>
      </p>

      <h1>{stateName(state)}</h1>
      <p className="lede">
        {stat
          ? `${stat.deployments.toLocaleString()} published agency record${
              stat.deployments === 1 ? "" : "s"
            } and ${stat.cameras.toLocaleString()} mapped camera location${
              stat.cameras === 1 ? "" : "s"
            }.`
          : "Published agency records and mapped camera locations."}
      </p>

      {error && <div className="notice error">{error}</div>}
      {!records && !error && <p className="state">Loading records…</p>}

      {records && published.length === 0 && (
        <p className="state">
          No published records for {stateName(state)} yet. Camera locations may
          still be mapped here — the map shows those regardless of whether an
          agency record has been documented.{" "}
          <Link to="/">Open the map</Link>.
        </p>
      )}

      {published.length > 0 && (
        <div className="table-scroll">
          <table className="records">
            <thead>
              <tr>
                <th>Location</th>
                <th>Agency</th>
                <th>Documented units</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {published.map((r) => (
                <tr key={r.id}>
                  <td>
                    <Link to={recordPath(r)}>
                      {r.city}, {r.state}
                    </Link>
                  </td>
                  <td>{r.agency_name}</td>
                  <td>
                    {r.documented_units ?? <span className="unknown">?</span>}
                  </td>
                  <td>
                    <StatusBadge status={r.status} />
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
