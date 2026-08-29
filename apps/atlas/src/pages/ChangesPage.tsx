import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { getChanges, type Changes } from "../lib/api";
import { stateName } from "../lib/states";

const WINDOWS = [7, 30, 90] as const;

/**
 * What has changed in the atlas recently.
 *
 * Two different things, deliberately kept apart. Cameras arriving is
 * OpenStreetMap contributors mapping more of the world, and says nothing
 * about whether anyone has checked those cameras against a public record.
 * Decisions are this project doing its own work. Merging them into one
 * "activity" number would let volume stand in for verification.
 */
export default function ChangesPage() {
  const [days, setDays] = useState<number>(30);
  const [data, setData] = useState<Changes | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(
    (window: number) => {
      setError(null);
      setData(null);
      getChanges(window)
        .then(setData)
        .catch((err: unknown) =>
          setError(err instanceof Error ? err.message : "Could not load recent changes"),
        );
    },
    [],
  );

  useEffect(() => load(days), [days, load]);

  return (
    <div className="page">
      <h1>What's changed</h1>
      <p className="lede">
        Cameras newly mapped, and decisions taken on records.
      </p>

      <div className="filters">
        {WINDOWS.map((w) => (
          <button
            key={w}
            type="button"
            aria-pressed={days === w}
            onClick={() => setDays(w)}
          >
            Last {w} days
          </button>
        ))}
      </div>

      {error && (
        <div className="notice error">
          {error}{" "}
          {/* A dead error message makes a transient failure look permanent. */}
          <button type="button" className="retry" onClick={() => load(days)}>
            Try again
          </button>
        </div>
      )}
      {!data && !error && <p className="state">Loading…</p>}

      {data && (
        <>
          <h2>Cameras newly mapped</h2>
          {data.cameras_added === 0 ? (
            <p className="state">
              No new camera locations in this window. The import runs weekly;
              a quiet week is normal.
            </p>
          ) : (
            <>
              <p>
                <strong>{data.cameras_added.toLocaleString()}</strong> camera
                location{data.cameras_added === 1 ? "" : "s"} first appeared in
                OpenStreetMap in this window.
              </p>
              <ul className="change-list">
                {data.by_state.map((s) => (
                  <li key={s.state}>
                    <Link to={`/state/${s.state.toLowerCase()}`}>
                      {stateName(s.state)}
                    </Link>
                    <span>+{s.cameras.toLocaleString()}</span>
                  </li>
                ))}
              </ul>
            </>
          )}

          <h2>Decisions on records</h2>
          {data.decisions.length === 0 ? (
            <p className="state">
              No records were reviewed, verified, or merged in this window.
            </p>
          ) : (
            <ul className="change-events">
              {data.decisions.map((e, i) => (
                <li key={`${e.created_at}-${i}`}>
                  <div>
                    {e.agency_name && e.state && e.slug ? (
                      <Link to={`/state/${e.state.toLowerCase()}/${e.slug}`}>
                        {e.agency_name}
                      </Link>
                    ) : (
                      <span>A record</span>
                    )}
                    <span className="change-meta">
                      {e.from_status
                        ? `${e.from_status.replace(/_/g, " ")} → ${e.to_status.replace(/_/g, " ")}`
                        : e.to_status.replace(/_/g, " ")}
                      {e.evidence_type ? ` · ${e.evidence_type.replace(/_/g, " ")}` : ""}
                      {" · "}
                      {new Date(e.created_at).toLocaleDateString()}
                    </span>
                    {e.note && <span className="change-meta">{e.note}</span>}
                  </div>
                  {/* The document a decision rested on, linked from the
                      decision itself — the point of recording it. */}
                  {e.source_links.length > 0 && (
                    <a href={e.source_links[0]} target="_blank" rel="noopener noreferrer">
                      Source
                    </a>
                  )}
                </li>
              ))}
            </ul>
          )}

          <p className="change-note">
            New cameras are OpenStreetMap contributors mapping more of the
            world; they are not a sign that anything has been checked against
            a public record. See <Link to="/coverage">Coverage</Link> for how
            much of this atlas has been verified.
          </p>
        </>
      )}
    </div>
  );
}
