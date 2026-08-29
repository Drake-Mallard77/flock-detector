import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";

import {
  listDeployments,
  recordPath,
  verifyDeployment,
  type Deployment,
  type EvidenceType,
} from "../lib/api";

// Only the evidence types that represent an actual public record. The API
// enforces the same list; offering osm_import here would present a circular
// verification as a valid choice.
const EVIDENCE_OPTIONS: Array<{ value: EvidenceType; label: string }> = [
  { value: "council_report", label: "Council report" },
  { value: "contract", label: "Contract / agreement" },
  { value: "invoice", label: "Invoice" },
  { value: "foia_response", label: "FOIA response" },
  { value: "news_article", label: "News article" },
];

/**
 * Verifying published records against public records.
 *
 * Every published record is currently `osm_documented`: mapped by
 * OpenStreetMap contributors and, in the site's own words, not checked
 * against anything a public body filed. The Review Desk could already set
 * "confirmed" — it just never showed a record you could set it on, because
 * its queue only loads status='under_review'. This is the missing half.
 *
 * Ordered by documented units, largest first. Verifying Detroit's 450
 * cameras moves the atlas further than verifying a three-camera village,
 * and a queue of 1,142 needs to put the consequential ones in front.
 */
export default function VerificationQueue() {
  const [queue, setQueue] = useState<Deployment[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [openId, setOpenId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState<string | null>(null);

  const [evidence, setEvidence] = useState<EvidenceType>("council_report");
  const [link, setLink] = useState("");
  const [units, setUnits] = useState("");

  const load = useCallback(() => {
    setError(null);
    listDeployments({ status: "osm_documented", limit: 50 })
      .then((rows) =>
        setQueue(
          [...rows].sort(
            (a, b) => (b.documented_units ?? 0) - (a.documented_units ?? 0),
          ),
        ),
      )
      .catch((err: unknown) =>
        setError(err instanceof Error ? err.message : "Could not load records"),
      );
  }, []);

  useEffect(load, [load]);

  function open(id: string) {
    setOpenId(id);
    setEvidence("council_report");
    setLink("");
    setUnits("");
    setError(null);
  }

  async function submit(record: Deployment, status: "confirmed" | "contract_found") {
    setBusy(true);
    setError(null);
    try {
      await verifyDeployment(record.id, {
        status,
        evidence_type: evidence,
        source_links: [link],
        documented_units: units.trim() === "" ? undefined : Number(units),
      });
      setDone(`${record.agency_name} is now ${status.replace(/_/g, " ")}.`);
      setOpenId(null);
      load();
    } catch (err: unknown) {
      // Surfaced rather than swallowed: the API refuses circular or
      // unsourced verifications, and its message explains why. That message
      // is the useful part.
      setError(err instanceof Error ? err.message : "Could not save this verification");
    } finally {
      setBusy(false);
    }
  }

  if (!queue && !error) return <p className="state">Loading records…</p>;

  return (
    <section className="verify">
      <h2>Verify against public records</h2>
      <p className="verify-lede">
        Every published record is currently <strong>OSM documented</strong> —
        mapped by OpenStreetMap contributors, not checked against anything a
        public body filed. Attaching the council report, contract, invoice or
        FOIA response behind one promotes it to <strong>Confirmed</strong>.
        Largest deployments first.
      </p>

      {done && <div className="notice success">{done}</div>}
      {error && <div className="notice error">{error}</div>}

      {queue && queue.length === 0 && (
        <p className="state">No records awaiting verification.</p>
      )}

      <ul className="verify-list">
        {(queue ?? []).map((d) => (
          <li key={d.id}>
            <div className="verify-row">
              <div>
                <Link to={recordPath(d)}>{d.agency_name}</Link>
                <span className="verify-meta">
                  {d.city}, {d.state}
                  {d.documented_units != null
                    ? ` · ${d.documented_units.toLocaleString()} documented units`
                    : ""}
                </span>
              </div>
              <button type="button" onClick={() => (openId === d.id ? setOpenId(null) : open(d.id))}>
                {openId === d.id ? "Cancel" : "Add evidence"}
              </button>
            </div>

            {openId === d.id && (
              <div className="verify-form">
                <label>
                  <span>Evidence type</span>
                  <select
                    value={evidence}
                    onChange={(e) => setEvidence(e.target.value as EvidenceType)}
                  >
                    {EVIDENCE_OPTIONS.map((o) => (
                      <option key={o.value} value={o.value}>
                        {o.label}
                      </option>
                    ))}
                  </select>
                </label>

                <label>
                  <span>Link to the document</span>
                  <input
                    type="url"
                    value={link}
                    placeholder="https://…"
                    onChange={(e) => setLink(e.target.value)}
                  />
                </label>

                <label>
                  <span>Documented units (optional)</span>
                  <input
                    type="number"
                    min={0}
                    value={units}
                    placeholder={d.documented_units != null ? String(d.documented_units) : ""}
                    onChange={(e) => setUnits(e.target.value)}
                  />
                </label>

                <div className="verify-actions">
                  <button
                    type="button"
                    disabled={busy || link.trim() === ""}
                    onClick={() => void submit(d, "confirmed")}
                  >
                    {busy ? "Saving…" : "Confirm"}
                  </button>
                  <button
                    type="button"
                    disabled={busy || link.trim() === ""}
                    onClick={() => void submit(d, "contract_found")}
                  >
                    Contract found
                  </button>
                </div>
                {/* Disabling on an empty link states the rule before the
                    server has to. The API still enforces it — this is a
                    convenience, not the guarantee. */}
                <p className="verify-hint">
                  A record can only be confirmed with a link to the document it
                  rests on.
                </p>
              </div>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
