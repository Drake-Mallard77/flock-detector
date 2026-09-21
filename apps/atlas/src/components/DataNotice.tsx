import { useEffect, useState } from "react";

import { API_BASE } from "../lib/api";
import "./DataNotice.css";

/**
 * Tells visitors when the record data is unavailable.
 *
 * Without this the site fails silently in the worst way: every page still
 * returns 200 and renders its layout, so the map is simply empty and the
 * tables simply have no rows. Nothing distinguishes "there are no cameras
 * here" from "the database is down", and on a public-records project the
 * difference is the whole point — an empty map of a surveilled city is a
 * factual claim, and we should not be making it by accident.
 *
 * Driven by a probe rather than a hardcoded flag so it clears itself. If
 * the database comes back early, or later than expected, the banner
 * disappears on the next page load with no deploy and no stale promise
 * left on screen. A date baked into the markup would need a second deploy
 * to remove, which is exactly the deploy nobody remembers to do.
 */
export default function DataNotice() {
  const [down, setDown] = useState(false);

  useEffect(() => {
    const ac = new AbortController();

    // /health/data is the API's own answer to "is the record data usable".
    // It already returns 503 both when the database is unreachable and when
    // the newest import has gone stale, which are the two cases worth
    // telling a reader about — so this needs no separate endpoint.
    fetch(`${API_BASE}/health/data`, { signal: ac.signal })
      .then((r) => setDown(!r.ok))
      // A network-level failure is also a real outage from the reader's
      // side, so it counts. AbortError is not: that is just navigation.
      .catch((e) => {
        if (e?.name !== "AbortError") setDown(true);
      });

    return () => ac.abort();
  }, []);

  if (!down) return null;

  return (
    // role="status" so the banner is announced when it appears rather than
    // only being seen. It is polite, not an alert: the page is still usable
    // and interrupting someone mid-sentence would not make it more so.
    <div className="data-notice" role="status">
      <strong>Camera data is temporarily unavailable.</strong>{" "}
      <span>
        The database behind this atlas reached a monthly limit on its hosting
        plan and is expected back on 1 October. Pages, methodology and
        documentation still work — maps and record listings will be empty
        until then.
      </span>
    </div>
  );
}
