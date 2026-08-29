import { useEffect, useState } from "react";
import { Link } from "react-router-dom";

import { getCoverage, type Coverage } from "../lib/api";

const pct = (part: number, whole: number) =>
  whole === 0 ? "0%" : `${Math.round((part / whole) * 100)}%`;

const n = (v: number) => v.toLocaleString();

/**
 * What the atlas knows, and what it doesn't.
 *
 * Every figure is fetched rather than written into the page. A
 * hand-maintained "about the data" section is wrong within a week and
 * nobody notices, which is worse than not having one — it looks like a
 * claim and behaves like a guess.
 *
 * The uncomfortable numbers are here on purpose. The share of cameras that
 * can't be attributed to any agency, and the share of records nobody has
 * checked against a public record, are the two facts most likely to be
 * asked about by a journalist deciding whether to cite this. Answering
 * before being asked is the difference between a records project and a
 * scraper with a nice map.
 */
export default function CoveragePage() {
  const [c, setC] = useState<Coverage | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    getCoverage()
      .then((v) => !cancelled && setC(v))
      .catch((err: unknown) =>
        !cancelled &&
        setError(err instanceof Error ? err.message : "Could not load coverage figures"),
      );
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div className="page">
      <h1>Coverage and gaps</h1>
      <p className="lede">
        How much of the picture this atlas actually holds — including the
        parts it can't account for.
      </p>

      {error && <div className="notice error">{error}</div>}
      {!c && !error && <p className="state">Loading figures…</p>}

      {c && (
        <>
          <div className="coverage-grid">
            <div className="coverage-stat">
              <strong>{n(c.cameras)}</strong>
              <span>camera locations mapped, across {c.states} states</span>
            </div>
            <div className="coverage-stat">
              <strong>{n(c.published_records)}</strong>
              <span>agency records published</span>
            </div>
            <div className="coverage-stat">
              <strong>{n(c.verified_records)}</strong>
              <span>
                checked against a public record ({pct(c.verified_records, c.published_records)}{" "}
                of published)
              </span>
            </div>
            <div className="coverage-stat">
              <strong>{pct(c.cameras_with_operator, c.cameras)}</strong>
              <span>of cameras name an operator in OpenStreetMap</span>
            </div>
          </div>

          <div className="prose">
            <h2>Cameras nobody can attribute</h2>
            <p>
              Only {n(c.cameras_with_operator)} of {n(c.cameras)} camera
              locations ({pct(c.cameras_with_operator, c.cameras)}) carry an{" "}
              <code>operator</code> tag in OpenStreetMap, and{" "}
              {n(c.cameras_linked)} are attached to a published record. The
              remainder are cameras someone mapped in the field without
              recording who runs them.
            </p>
            <p>
              This atlas does not guess. Inferring an owner from proximity —
              assuming a camera near a city boundary belongs to that city's
              police — would manufacture exactly the kind of attribution the
              project exists to avoid making. So those cameras appear on the{" "}
              <Link to="/">map</Link> and in the{" "}
              <Link to="/methodology">exports</Link>, and belong to no record.
            </p>

            <h2>Records nobody has verified yet</h2>
            <p>
              {c.verified_records === 0 ? (
                <>
                  <strong>None</strong> of the {n(c.published_records)}{" "}
                  published records have been checked against a council report,
                  contract, invoice, news article, or FOIA response. Every one
                  is derived from OpenStreetMap and carries the{" "}
                  <strong>OSM documented</strong> status, which says so.
                </>
              ) : (
                <>
                  {n(c.verified_records)} of {n(c.published_records)} published
                  records ({pct(c.verified_records, c.published_records)}) have
                  been checked against a council report, contract, invoice,
                  news article, or FOIA response. The rest are derived from
                  OpenStreetMap and carry the <strong>OSM documented</strong>{" "}
                  status.
                </>
              )}
            </p>
            <p>
              That distinction is the reason those are two different statuses
              rather than one. A record derived from volunteer mapping is a
              lead; a record backed by a document a public body filed is a
              finding. Calling the first the second would be the most damaging
              thing this project could do.
            </p>

            <h2>What "not listed here" does not mean</h2>
            <p>
              An agency absent from this atlas is not an agency without
              cameras. It is an agency nobody has mapped or documented yet.
              Absence of a record is not evidence of absence, and no figure on
              this page should be read as a count of what exists — only of
              what has been recorded.
            </p>

            <h2>How current this is</h2>
            <p>
              Camera data refreshes weekly from OpenStreetMap.{" "}
              {c.last_import ? (
                <>
                  The most recent refresh completed{" "}
                  <strong>{new Date(c.last_import).toLocaleString()}</strong>.
                </>
              ) : (
                <>No import has been recorded yet.</>
              )}{" "}
              Individual records also carry their own "last reviewed" date, which
              is a different and usually older thing.
            </p>

            <p>
              {n(c.cameras_with_direction)} cameras (
              {pct(c.cameras_with_direction, c.cameras)}) record which way they
              face, which is what the direction cones on the map are drawn
              from. The rest are shown as a plain dot.
            </p>
          </div>
        </>
      )}
    </div>
  );
}
