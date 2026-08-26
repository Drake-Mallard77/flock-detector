import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";

import { ApiError, createDeployment, type EvidenceType } from "../lib/api";

const EVIDENCE_OPTIONS: Array<{ value: EvidenceType; label: string }> = [
  { value: "council_report", label: "Council report or staff memo" },
  { value: "contract", label: "Contract or agreement" },
  { value: "invoice", label: "Invoice or purchase record" },
  { value: "news_article", label: "News article" },
  { value: "foia_response", label: "FOIA / public records response" },
  { value: "user_photo", label: "Photo of a camera" },
];

export default function SubmitPage() {
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submittedId, setSubmittedId] = useState<string | null>(null);

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);

    const form = new FormData(e.currentTarget);
    const units = String(form.get("documented_units") ?? "").trim();
    const sources = String(form.get("source_links") ?? "")
      .split("\n")
      .map((s) => s.trim())
      .filter(Boolean);

    try {
      const res = await createDeployment({
        agency_name: String(form.get("agency_name") ?? "").trim(),
        city: String(form.get("city") ?? "").trim(),
        state: String(form.get("state") ?? "").trim().toUpperCase(),
        county: String(form.get("county") ?? "").trim() || undefined,
        documented_units: units ? Number(units) : undefined,
        evidence_type: form.get("evidence_type") as EvidenceType,
        source_links: sources,
        notes: String(form.get("notes") ?? "").trim() || undefined,
      });
      setSubmittedId(res.id);
    } catch (err) {
      setError(
        err instanceof ApiError && err.status === 429
          ? "You've submitted several records in a short time. Please wait a moment and try again."
          : err instanceof Error
            ? err.message
            : "Something went wrong submitting this record.",
      );
    } finally {
      setSubmitting(false);
    }
  }

  if (submittedId) {
    return (
      <div className="page">
        <h1>Thank you — submitted for review</h1>
        <div className="notice success">
          Your submission was received and is queued for moderator review. It won't appear
          publicly until a moderator has checked it against the sources you provided.
        </div>
        <p>
          <button className="primary" onClick={() => setSubmittedId(null)}>
            Submit another
          </button>{" "}
          <Link to="/deployments" style={{ marginLeft: "0.75rem" }}>
            Browse deployments
          </Link>
        </p>
      </div>
    );
  }

  return (
    <div className="page">
      <h1>Submit a sighting</h1>
      <p className="lede">
        This atlas publishes records that can be checked. The more specific your sources, the
        faster a moderator can verify and publish the record.
      </p>

      <div className="notice">
        <strong>Please don't submit anything you obtained unlawfully</strong>, and don't include
        personal information about individuals. Everything here is a submission for review — it
        is not published until a moderator verifies it.
      </div>

      {error && <div className="notice error">{error}</div>}

      <form onSubmit={handleSubmit}>
        <div className="form-field">
          <label htmlFor="agency_name">Agency or organization *</label>
          <input id="agency_name" name="agency_name" required placeholder="Springfield Police Department" />
        </div>

        <div className="form-field">
          <label htmlFor="city">City *</label>
          <input id="city" name="city" required placeholder="Springfield" />
        </div>

        <div className="form-field">
          <label htmlFor="state">State *</label>
          <input
            id="state"
            name="state"
            required
            maxLength={2}
            placeholder="IL"
            style={{ maxWidth: "6rem" }}
          />
          <span className="hint">Two-letter abbreviation.</span>
        </div>

        <div className="form-field">
          <label htmlFor="county">County</label>
          <input id="county" name="county" placeholder="Sangamon" />
        </div>

        <div className="form-field">
          <label htmlFor="documented_units">Documented number of cameras</label>
          <input
            id="documented_units"
            name="documented_units"
            type="number"
            min={0}
            style={{ maxWidth: "9rem" }}
          />
          <span className="hint">
            Leave blank if the records don't say — an unknown count is published as "?" rather
            than guessed.
          </span>
        </div>

        <div className="form-field">
          <label htmlFor="evidence_type">Evidence type *</label>
          <select id="evidence_type" name="evidence_type" required defaultValue="">
            <option value="" disabled>
              Select…
            </option>
            {EVIDENCE_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </div>

        <div className="form-field">
          <label htmlFor="source_links">Source links</label>
          <textarea
            id="source_links"
            name="source_links"
            placeholder={"https://example.gov/council-minutes.pdf\nhttps://example.gov/contract.pdf"}
          />
          <span className="hint">One URL per line. Public documents are strongest.</span>
        </div>

        <div className="form-field">
          <label htmlFor="notes">Notes</label>
          <textarea id="notes" name="notes" placeholder="Anything a reviewer should know." />
        </div>

        <button className="primary" type="submit" disabled={submitting}>
          {submitting ? "Submitting…" : "Submit for review"}
        </button>
      </form>
    </div>
  );
}
