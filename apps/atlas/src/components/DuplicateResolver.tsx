import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";

import {
  listDuplicates,
  mergeDeployment,
  type DuplicateGroup,
} from "../lib/api";

/**
 * Resolving records the atlas holds twice for the same agency.
 *
 * These exist because the importer's identity check was case-insensitive
 * but not punctuation-insensitive, so "Sheriff's Office" and "Sheriff’s
 * Office" were treated as different agencies and a new candidate was
 * proposed every week. That check is fixed; this clears what it left
 * behind.
 *
 * Merging is a moderator action rather than an automatic cleanup because
 * choosing which record survives decides which agency name and which
 * evidence the atlas stands behind. Nothing is deleted — the folded-in
 * record is retired and keeps a note saying where it went.
 */
export default function DuplicateResolver() {
  const [groups, setGroups] = useState<DuplicateGroup[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [merged, setMerged] = useState<string | null>(null);

  const load = useCallback(() => {
    listDuplicates()
      .then(setGroups)
      .catch((err: unknown) =>
        setError(err instanceof Error ? err.message : "Could not load duplicates"),
      );
  }, []);

  useEffect(load, [load]);

  /**
   * Folds every other record in the group into `survivor`.
   *
   * Sequential, not concurrent. A group can hold three or more records, and
   * firing the merges in parallel would have them racing to move the same
   * cameras and each triggering its own reload. One at a time, then a
   * single refresh.
   */
  async function keep(group: DuplicateGroup, survivorId: string, label: string) {
    const others = group.records.filter((r) => r.id !== survivorId);
    setBusy(survivorId);
    setError(null);

    let moved = 0;
    try {
      for (const other of others) {
        const res = await mergeDeployment(survivorId, other.id);
        moved += res.cameras_moved;
      }
      setMerged(
        `Kept ${label}` +
          (moved > 0
            ? ` — ${moved.toLocaleString()} camera${moved === 1 ? "" : "s"} moved across.`
            : "."),
      );
      // Reload rather than pruning locally: merging can resolve a group of
      // three down to a pair, and guessing at that client-side would drift
      // from what the server actually did.
      load();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Could not merge these records");
      // Refresh anyway: some of the group may have merged before the
      // failure, and leaving the stale list on screen would invite acting
      // on records that no longer exist as shown.
      load();
    } finally {
      setBusy(null);
    }
  }

  if (error && !groups) {
    return <div className="notice error">{error}</div>;
  }
  if (!groups) return <p className="state">Checking for duplicates…</p>;

  if (groups.length === 0) {
    return (
      <p className="state">
        No duplicate records. Every agency appears once per state.
      </p>
    );
  }

  return (
    <section className="duplicates">
      <h2>
        Possible duplicates <span className="duplicates-count">{groups.length}</span>
      </h2>
      <p className="duplicates-lede">
        These records normalise to the same agency name within a state.
        Choose the one to keep — its cameras stay, the other's move across,
        and the folded-in record is retired with a note pointing here.
      </p>

      {merged && <div className="notice success">{merged}</div>}
      {error && <div className="notice error">{error}</div>}

      {groups.map((group) => (
        <div className="duplicate-group" key={`${group.state}-${group.records[0].id}`}>
          <h3>
            {group.records[0].agency_name} · {group.state}
          </h3>
          <ul>
            {group.records.map((r) => (
              <li key={r.id}>
                <div className="duplicate-record">
                  <Link to={`/state/${r.state.toLowerCase()}/${r.slug}`}>
                    {r.agency_name}
                  </Link>
                  <span className="duplicate-meta">
                    {r.city} · {r.status.replace(/_/g, " ")} ·{" "}
                    {r.linked_cameras.toLocaleString()} linked camera
                    {r.linked_cameras === 1 ? "" : "s"}
                    {r.documented_units != null
                      ? ` · ${r.documented_units.toLocaleString()} documented`
                      : ""}
                  </span>
                </div>
                <button
                  type="button"
                  disabled={busy !== null}
                  onClick={() => void keep(group, r.id, r.agency_name)}
                >
                  {busy === r.id ? "Merging…" : "Keep this one"}
                </button>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </section>
  );
}
