import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { listStateStats, type StateStat } from "../lib/api";
import { stateName } from "../lib/states";

/**
 * The location index.
 *
 * Until now the only way into 1,150 agency records was knowing a name and
 * typing it into a search box, which fails the reader this project is for:
 * someone who wants to know what's deployed near them and has no agency
 * name to search for. A place is the thing people actually have in mind.
 */
export default function StatesPage() {
  const [stats, setStats] = useState<StateStat[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    listStateStats()
      .then((s) => {
        if (!cancelled) setStats(s);
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Could not load state totals");
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const totalRecords = stats?.reduce((sum, s) => sum + s.deployments, 0) ?? 0;
  const totalCameras = stats?.reduce((sum, s) => sum + s.cameras, 0) ?? 0;

  return (
    <div className="page">
      <h1>Browse by state</h1>
      <p className="lede">
        {stats
          ? `${totalRecords.toLocaleString()} published agency records and ${totalCameras.toLocaleString()} mapped camera locations across ${stats.length} states and territories.`
          : "Published agency records and mapped camera locations, by state."}
      </p>

      {error && <div className="notice error">{error}</div>}
      {!stats && !error && <p className="state">Loading…</p>}

      {stats && (
        <ul className="state-grid">
          {stats.map((s) => (
            <li key={s.state}>
              <Link to={`/state/${s.state.toLowerCase()}`}>
                <span className="state-grid-name">{stateName(s.state)}</span>
                <span className="state-grid-counts">
                  {s.deployments.toLocaleString()} record
                  {s.deployments === 1 ? "" : "s"} ·{" "}
                  {s.cameras.toLocaleString()} camera
                  {s.cameras === 1 ? "" : "s"}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}

      {/* The two numbers count different things and will not add up to each
          other, which is worth saying once here rather than leaving readers
          to infer a relationship that isn't there. */}
      <p className="state-grid-note">
        Records are agency-level findings this project publishes. Camera
        locations are individual points contributed to OpenStreetMap; most
        carry no operator tag, so they can't be attributed to an agency and
        are counted here but not under any record.
      </p>
    </div>
  );
}
