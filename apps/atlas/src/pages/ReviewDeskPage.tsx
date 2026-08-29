import { useCallback, useEffect, useState } from "react";

import DuplicateResolver from "../components/DuplicateResolver";
import VerificationQueue from "../components/VerificationQueue";
import { Link } from "react-router-dom";

import GoogleSignIn from "../components/GoogleSignIn";
import StatusBadge from "../components/StatusBadge";
import { useAuth } from "../lib/auth";
import {
  bulkReviewDeployments,
  listDeployments,
  reviewDeployment,
  type Deployment,
  type DeploymentStatus,
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

// Shown on each card so a reviewer can see at a glance whether a record is
// about a police force or a supermarket — the two warrant very different
// scrutiny, and the distinction is easy to miss in a long queue.
const OPERATOR_TYPE_LABELS: Record<string, string> = {
  law_enforcement: "Law enforcement",
  government: "Government",
  education: "Education",
  private: "Private",
  unknown: "Unclassified",
};

const DECISIONS: Array<{ status: DeploymentStatus; label: string }> = [
  { status: "confirmed", label: "Confirm" },
  { status: "contract_found", label: "Contract found" },
  // For OSM-derived candidates: publishes without claiming the
  // public-records verification that "Confirm" asserts.
  { status: "osm_documented", label: "Mark OSM-documented" },
  { status: "disputed", label: "Dispute" },
  { status: "removed", label: "Remove" },
];

export default function ReviewDeskPage() {
  const { me, loading, signIn, signOut, isModerator } = useAuth();
  const [queue, setQueue] = useState<Deployment[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [bulkBusy, setBulkBusy] = useState(false);

  const load = useCallback(() => {
    listDeployments({ status: "under_review", limit: 200 })
      .then((rows) => {
        setQueue(rows);
        // Drop selections for records that are no longer in the queue,
        // otherwise a later bulk action could target something already
        // decided.
        setSelected((prev) => {
          const live = new Set(rows.map((r) => r.id));
          return new Set([...prev].filter((id) => live.has(id)));
        });
      })
      .catch((err: unknown) =>
        setError(err instanceof Error ? err.message : "Could not load the review queue"),
      );
  }, []);

  useEffect(() => {
    if (isModerator) load();
  }, [isModerator, load]);

  async function decide(id: string, status: DeploymentStatus) {
    setBusyId(id);
    setError(null);
    try {
      await reviewDeployment(id, status);
      // Refetch rather than mutating locally, so the list reflects what the
      // server actually recorded.
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not save that decision");
    } finally {
      setBusyId(null);
    }
  }

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function bulkDecide(status: DeploymentStatus) {
    if (selected.size === 0) return;
    setBulkBusy(true);
    setError(null);
    try {
      const res = await bulkReviewDeployments([...selected], status);
      // Report what the server actually changed rather than what was asked
      // for — ids that no longer exist are skipped silently server-side.
      if (res.updated !== selected.size) {
        setError(
          `${res.updated} of ${selected.size} records were updated. The rest may have been reviewed already.`,
        );
      }
      setSelected(new Set());
      load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not apply that bulk decision");
    } finally {
      setBulkBusy(false);
    }
  }

  async function handleCredential(credential: string) {
    setError(null);
    try {
      await signIn(credential);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Sign-in failed");
    }
  }

  if (loading) {
    return (
      <div className="page">
        <p className="state">Checking your session…</p>
      </div>
    );
  }

  if (!me) {
    return (
      <div className="page">
        <h1>Review desk</h1>
        <p className="lede">
          Moderators sign in here to review submitted records. Signing in doesn't grant
          moderator access on its own — access is assigned separately.
        </p>
        {error && <div className="notice error">{error}</div>}
        <GoogleSignIn onCredential={handleCredential} />
      </div>
    );
  }

  if (!isModerator) {
    return (
      <div className="page">
        <h1>Review desk</h1>
        <div className="notice">
          You're signed in as <strong>{me.email}</strong>, but this account doesn't have
          moderator access. If that's unexpected, ask an administrator to grant it.
        </div>
        <button className="primary" onClick={signOut}>
          Sign out
        </button>
      </div>
    );
  }

  return (
    <div className="page">
      <div className="review-header">
        <div>
          <h1>Review desk</h1>
          <p className="lede">
            Submissions awaiting review. Nothing here is public until you act on it.
          </p>
        </div>
        <div className="review-identity">
          <span>{me.email}</span>
          <button onClick={signOut}>Sign out</button>
        </div>
      </div>

      {error && <div className="notice error">{error}</div>}

      {queue === null && !error && <p className="state">Loading queue…</p>}

      {queue && queue.length === 0 && (
        <p className="state">
          Nothing is waiting for review. <Link to="/deployments">Browse published records</Link>.
        </p>
      )}

      {/* Above the review queue: a duplicate is a defect in what is already
          published, while the queue is about what to publish next. */}
      <DuplicateResolver />

      {/* Below duplicates, above the candidate queue: a duplicate is a
          defect in what is published, verification is the standing work,
          and the candidate queue is usually empty. */}
      <VerificationQueue />

      {queue && queue.length > 0 && (
        <div className="review-bulk">
          <label>
            <input
              type="checkbox"
              checked={selected.size > 0 && selected.size === queue.length}
              // Indeterminate when only some are selected, so "select all"
              // never looks like it covers more than it does.
              ref={(el) => {
                if (el) el.indeterminate = selected.size > 0 && selected.size < queue.length;
              }}
              onChange={(e) =>
                setSelected(e.target.checked ? new Set(queue.map((d) => d.id)) : new Set())
              }
            />
            <span>
              {selected.size > 0 ? `${selected.size} selected` : `Select all ${queue.length}`}
            </span>
          </label>

          <div className="review-bulk-actions">
            {DECISIONS.map((dec) => (
              <button
                key={dec.status}
                disabled={selected.size === 0 || bulkBusy}
                onClick={() => void bulkDecide(dec.status)}
              >
                {bulkBusy ? "Working…" : `${dec.label} selected`}
              </button>
            ))}
          </div>
        </div>
      )}

      {queue?.map((d) => (
        <article key={d.id} className="review-card">
          <header>
            <label className="review-select">
              <input
                type="checkbox"
                checked={selected.has(d.id)}
                onChange={() => toggle(d.id)}
                aria-label={`Select ${d.agency_name}`}
              />
            </label>
            <h2>
              {d.city}, {d.state}
            </h2>
            {d.operator_type && (
              <span className={`operator-type operator-type-${d.operator_type}`}>
                {OPERATOR_TYPE_LABELS[d.operator_type] ?? d.operator_type}
              </span>
            )}
            <StatusBadge status={d.status} />
          </header>

          <dl className="review-fields">
            <div>
              <dt>Agency</dt>
              <dd>{d.agency_name}</dd>
            </div>
            <div>
              <dt>Documented units</dt>
              <dd>{d.documented_units ?? <span className="unknown">Not stated</span>}</dd>
            </div>
            <div>
              <dt>Evidence</dt>
              <dd>{EVIDENCE_LABELS[d.evidence_type] ?? d.evidence_type}</dd>
            </div>
            <div>
              <dt>Submitted</dt>
              <dd>{new Date(d.created_at).toLocaleString()}</dd>
            </div>
          </dl>

          <div className="review-sources">
            <strong>Sources</strong>
            {d.source_links.length === 0 ? (
              <p className="unknown">
                No sources provided — treat with suspicion; this can't be verified as-is.
              </p>
            ) : (
              <ul>
                {d.source_links.map((link) => (
                  <li key={link}>
                    <a href={link} target="_blank" rel="noopener noreferrer">
                      {link}
                    </a>
                  </li>
                ))}
              </ul>
            )}
          </div>

          {d.notes && (
            <p className="review-notes">
              <strong>Notes: </strong>
              {d.notes}
            </p>
          )}

          <div className="review-actions">
            {DECISIONS.map((dec) => (
              <button
                key={dec.status}
                disabled={busyId === d.id}
                onClick={() => decide(d.id, dec.status)}
              >
                {dec.label}
              </button>
            ))}
          </div>
        </article>
      ))}
    </div>
  );
}
