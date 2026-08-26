import { useEffect, useRef, useState } from "react";

import { searchPlaces, type Place } from "../lib/api";

/**
 * Place search for the map.
 *
 * Debounced at 500ms and cancels in-flight requests, because Nominatim's
 * usage policy caps us at roughly one request per second — firing per
 * keystroke would get the shared instance to block us outright.
 */
export default function PlaceSearch({ onSelect }: { onSelect: (place: Place) => void }) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Place[]>([]);
  const [open, setOpen] = useState(false);
  const [status, setStatus] = useState<"idle" | "searching" | "empty" | "error">("idle");
  const abortRef = useRef<AbortController | null>(null);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const trimmed = query.trim();
    if (trimmed.length < 3) {
      setResults([]);
      setStatus("idle");
      return;
    }

    const timer = window.setTimeout(() => {
      abortRef.current?.abort();
      const controller = new AbortController();
      abortRef.current = controller;
      setStatus("searching");

      searchPlaces(trimmed, controller.signal)
        .then((places) => {
          setResults(places);
          setStatus(places.length ? "idle" : "empty");
          setOpen(true);
        })
        .catch((err: unknown) => {
          // An aborted request isn't a failure — it means the user kept
          // typing, and a newer request is already in flight.
          if (err instanceof DOMException && err.name === "AbortError") return;
          setResults([]);
          setStatus("error");
          setOpen(true);
        });
    }, 500);

    return () => window.clearTimeout(timer);
  }, [query]);

  // Close the dropdown on an outside click.
  useEffect(() => {
    function onDocClick(e: MouseEvent) {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDocClick);
    return () => document.removeEventListener("mousedown", onDocClick);
  }, []);

  function choose(place: Place) {
    onSelect(place);
    setQuery(place.label.split(",")[0] ?? place.label);
    setOpen(false);
  }

  return (
    <div className="place-search" ref={boxRef}>
      <input
        type="search"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onFocus={() => results.length && setOpen(true)}
        onKeyDown={(e) => {
          if (e.key === "Escape") setOpen(false);
          // Enter picks the top hit, so a search can be completed without
          // reaching for the mouse.
          if (e.key === "Enter" && results.length) choose(results[0]);
        }}
        placeholder="Search a city or address…"
        aria-label="Search for a place on the map"
      />

      {open && (status !== "idle" || results.length > 0) && (
        <ul className="place-results">
          {status === "searching" && <li className="place-status">Searching…</li>}
          {status === "empty" && <li className="place-status">No places found.</li>}
          {status === "error" && <li className="place-status">Place search is unavailable.</li>}
          {results.map((p) => (
            <li key={`${p.lat},${p.lng}`}>
              <button type="button" onClick={() => choose(p)}>
                {p.label}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
