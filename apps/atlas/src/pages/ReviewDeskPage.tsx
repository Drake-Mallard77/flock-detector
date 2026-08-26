import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";

import GoogleSignIn from "../components/GoogleSignIn";
import StatusBadge from "../components/StatusBadge";
import { useAuth } from "../lib/auth";
import {
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

const DECISIONS: Array<{ status: DeploymentStatus; label: string }> = [
  { status: "confirmed", label: "Confirm" },
  { status: "contract_found", label: "Contract found" },
  { status: "disputed", label: "Dispute" },
  { status: "removed", label: "Remove" },
];

export default function ReviewDeskPage() {
  const { me, loading, signIn, signOut, isModerator } = useAuth();
  const [queue, setQueue] = useState<Deployment[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = useCallback(() => {
    listDeployments({ status: "under_review" })
      .then(setQueue)
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

      {queue?.map((d) => (
        <article key={d.id} className="review-card">
          <header>
            <h2>
              {d.city}, {d.state}
            </h2>
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
