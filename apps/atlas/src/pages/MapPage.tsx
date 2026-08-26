import { useEffect, useRef, useState } from "react";
import L from "leaflet";
import "leaflet.markercluster";

import {
  listCameras,
  listManufacturers,
  type CameraFilters,
  type CameraSighting,
  type ManufacturerCount,
} from "../lib/api";

// Same-origin in production (Caddy proxies it); the Vite dev server proxies
// the same path locally. See apps/atlas/Caddyfile.
const TILE_BASE = import.meta.env.VITE_TILE_BASE ?? "/tiles";

// Mirrors the server-side LIMIT in services/api's handleListCameras. A
// response that hits exactly this was almost certainly truncated.
const API_LIMIT = 1000;

/**
 * Leaflet, not MapLibre GL.
 *
 * MapLibre requires WebGL, which isn't available everywhere — disabled by
 * privacy tooling as a fingerprinting surface, by GPU blocklists, or by
 * hardware acceleration being off. When it's missing the map simply cannot
 * draw, and this project's readers shouldn't have to weaken their setup (or
 * own newer hardware) to look at public records. Leaflet renders raster
 * tiles as ordinary DOM images and works without WebGL, at roughly 1/20th
 * the bundle size.
 *
 * Tiles are Esri's Light Gray Canvas, proxied through this origin. Others
 * ruled out along the way: tile.openstreetmap.org blocks application
 * traffic under OSM's tile usage policy; CARTO watermarks keyless tiles
 * "API KEY REQUIRED"; OpenFreeMap is vector-only at street zoom, which
 * would put WebGL back in the critical path. Proxying keeps requests
 * first-party, so tracker blockers don't drop them and the tile host never
 * learns who is looking at which area.
 *
 * Esri serves these {z}/{y}/{x} — row before column, the reverse of the
 * usual slippy-map order.
 */
const BASE_TILES = `${TILE_BASE}/ArcGIS/rest/services/Canvas/World_Light_Gray_Base/MapServer/tile/{z}/{y}/{x}`;

// Place names ship as a separate overlay in this basemap, so the labels
// stay crisp above the camera markers instead of being buried by them.
const LABEL_TILES = `${TILE_BASE}/ArcGIS/rest/services/Canvas/World_Light_Gray_Reference/MapServer/tile/{z}/{y}/{x}`;

const ATTRIBUTION =
  'Tiles &copy; Esri | Camera data &copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';

/** Escapes untrusted text before it goes into a Leaflet popup's HTML. */
function escapeHtml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

export default function MapPage() {
  const container = useRef<HTMLDivElement>(null);
  const map = useRef<L.Map | null>(null);
  const clusterLayer = useRef<L.MarkerClusterGroup | null>(null);
  const refetchTimer = useRef<number | undefined>(undefined);

  const [count, setCount] = useState<number | null>(null);
  const [truncated, setTruncated] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<CameraFilters>({});
  const [manufacturers, setManufacturers] = useState<ManufacturerCount[]>([]);

  // Map event handlers are registered once, so reading `filters` directly
  // inside them would capture the initial value. A ref keeps them current.
  const filtersRef = useRef(filters);
  filtersRef.current = filters;

  useEffect(() => {
    if (!container.current || map.current) return;

    const m = L.map(container.current, {
      center: [39.8, -98.5], // continental US
      zoom: 4,
      worldCopyJump: true,
    });
    map.current = m;

    L.tileLayer(BASE_TILES, { attribution: ATTRIBUTION, maxZoom: 18 }).addTo(m);
    L.tileLayer(LABEL_TILES, { maxZoom: 18, pane: "shadowPane" }).addTo(m);

    // Clustering isn't cosmetic: a full US import is 100k+ points, and
    // drawing individual markers at low zoom would stall the browser.
    const cluster = L.markerClusterGroup({
      chunkedLoading: true,
      showCoverageOnHover: false,
    });
    clusterLayer.current = cluster;
    m.addLayer(cluster);

    // Leaflet measures its container on init; this page is lazy-loaded, so
    // it can mount before layout settles. Observing the container covers
    // that plus later changes (rotation, mobile toolbars collapsing).
    const observer = new ResizeObserver(() => m.invalidateSize());
    observer.observe(container.current);

    void loadCameras(m);
    // Refetch the visible area on pan/zoom. The API caps each response at
    // 1,000 rows and a single state can hold several thousand, so a single
    // unfiltered fetch would silently show an arbitrary subset of the
    // country. Debounced so a drag doesn't fire a request per frame.
    m.on("moveend", () => {
      if (refetchTimer.current) window.clearTimeout(refetchTimer.current);
      refetchTimer.current = window.setTimeout(() => void loadCameras(m), 300);
    });

    return () => {
      if (refetchTimer.current) window.clearTimeout(refetchTimer.current);
      observer.disconnect();
      m.remove();
      map.current = null;
      clusterLayer.current = null;
    };
  }, []);

  // Vendor list comes from the data, not a hardcoded array — OSM
  // contributors add manufacturers over time, and a static list goes
  // silently out of date. A failure here only costs the dropdown.
  useEffect(() => {
    listManufacturers()
      .then(setManufacturers)
      .catch(() => setManufacturers([]));
  }, []);

  // Refetch when a filter changes (the map-event path only fires on move).
  useEffect(() => {
    if (map.current) void loadCameras(map.current);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters]);

  async function loadCameras(m: L.Map) {
    const b = m.getBounds();
    // Clamp to valid WGS84: at low zoom the world repeats horizontally, so
    // Leaflet can legitimately report longitudes beyond ±180. Passing those
    // through made the API return an empty array with HTTP 200 — no error,
    // just an empty map.
    const bbox: [number, number, number, number] = [
      Math.max(-180, b.getWest()),
      Math.max(-90, b.getSouth()),
      Math.min(180, b.getEast()),
      Math.min(90, b.getNorth()),
    ];

    try {
      const cameras: CameraSighting[] = await listCameras(bbox, filtersRef.current);
      const cluster = clusterLayer.current;
      if (!cluster) return;

      cluster.clearLayers();
      const markers = cameras.map((c) => {
        const marker = L.circleMarker([c.lat, c.lng], {
          radius: 5,
          color: "#ffffff",
          weight: 1.5,
          fillColor: "#1b6b4a",
          fillOpacity: 1,
        });
        const heading =
          c.direction !== undefined && c.direction !== null
            ? ` · facing ${Number(c.direction)}°`
            : "";
        marker.bindPopup(
          `<h3>${escapeHtml(c.manufacturer ?? "ALPR camera")}</h3>
           <div>${c.camera_type ? escapeHtml(c.camera_type) : "Type not recorded"}${heading}</div>
           <div class="popup-source">${
             c.source === "osm_import" ? "Source: OpenStreetMap" : "Source: community report"
           }</div>`,
        );
        return marker;
      });
      cluster.addLayers(markers);

      setCount(cameras.length);
      setTruncated(cameras.length >= API_LIMIT);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load camera locations");
    }
  }

  function setFilter(key: keyof CameraFilters, value: string) {
    setFilters((prev) => {
      const next = { ...prev };
      if (!value) delete next[key];
      else next[key] = value as never;
      return next;
    });
  }

  return (
    <div className="map-wrap">
      <div className="map-root" ref={container} />

      <div className="map-filters">
        <label>
          <span>Source</span>
          <select
            value={filters.source ?? ""}
            onChange={(e) => setFilter("source", e.target.value)}
          >
            <option value="">All sources</option>
            <option value="osm_import">OpenStreetMap</option>
            <option value="user_submission">Community reports</option>
          </select>
        </label>

        <label>
          <span>Review status</span>
          <select
            value={filters.status ?? ""}
            onChange={(e) => setFilter("status", e.target.value)}
          >
            <option value="">Any status</option>
            <option value="confirmed">Confirmed</option>
            <option value="under_review">Under review</option>
          </select>
        </label>

        <label>
          <span>Manufacturer</span>
          <select
            value={filters.manufacturer ?? ""}
            onChange={(e) => setFilter("manufacturer", e.target.value)}
          >
            <option value="">Any manufacturer</option>
            {manufacturers.map((m) => (
              <option key={m.manufacturer} value={m.manufacturer}>
                {m.manufacturer} ({m.count.toLocaleString()})
              </option>
            ))}
            <option value="unknown">Not recorded</option>
          </select>
        </label>
      </div>

      <div className="map-legend">
        <h2>Camera locations</h2>
        <div className="legend-row">
          <span className="legend-dot legend-dot-camera" />
          <span>Documented ALPR camera</span>
        </div>
        <p className="legend-note">
          {error
            ? error
            : count === null
              ? "Loading…"
              : truncated
                ? `Showing the first ${count.toLocaleString()} cameras in view — there are more here. Zoom in to see them all.`
                : `${count.toLocaleString()} camera${count === 1 ? "" : "s"} in view.`}
        </p>
      </div>
    </div>
  );
}
