import { useEffect, useRef, useState } from "react";
import maplibregl from "maplibre-gl";

import {
  listCameras,
  listManufacturers,
  type CameraFilters,
  type CameraSighting,
  type ManufacturerCount,
} from "../lib/api";

// CARTO's Positron basemap, NOT tile.openstreetmap.org. OSM's tile servers
// actively block application traffic under their tile usage policy
// (https://operations.osmfoundation.org/policies/tiles/) — they return
// HTTP 200 with an `x-blocked: Access denied` header and a placeholder
// image, so the map looks silently broken rather than erroring. CARTO
// permits this use and its muted palette suits a records project.
// The underlying data is still OSM-derived, hence both attributions.
const STYLE: maplibregl.StyleSpecification = {
  version: 8,
  sources: {
    basemap: {
      type: "raster",
      tiles: [
        "https://a.basemaps.cartocdn.com/light_all/{z}/{x}/{y}.png",
        "https://b.basemaps.cartocdn.com/light_all/{z}/{x}/{y}.png",
        "https://c.basemaps.cartocdn.com/light_all/{z}/{x}/{y}.png",
      ],
      tileSize: 256,
      attribution:
        '© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors © <a href="https://carto.com/attributions">CARTO</a>',
    },
  },
  layers: [{ id: "basemap", type: "raster", source: "basemap" }],
};

/**
 * Clamps a viewport to valid WGS84 ranges.
 *
 * At low zoom on a wide screen, MapLibre's getBounds() legitimately returns
 * longitudes outside [-180, 180] (the world repeats horizontally). Passing
 * those straight through made the API return an empty array with HTTP 200 —
 * no error, just a silently empty map. If the view spans more than the whole
 * globe, fall back to full world coverage rather than an inverted box.
 */
function clampBBox(b: maplibregl.LngLatBounds): [number, number, number, number] {
  const west = b.getWest();
  const east = b.getEast();
  const spansGlobe = east - west >= 360;

  return [
    spansGlobe ? -180 : Math.max(-180, Math.min(180, west)),
    Math.max(-90, Math.min(90, b.getSouth())),
    spansGlobe ? 180 : Math.max(-180, Math.min(180, east)),
    Math.max(-90, Math.min(90, b.getNorth())),
  ];
}

// Mirrors the server-side LIMIT in services/api's handleListCameras. If the
// response hits exactly this, results were almost certainly truncated.
const API_LIMIT = 1000;

/**
 * Escapes text before it goes into a MapLibre popup.
 *
 * Popup content is built as an HTML string, and these values originate from
 * OpenStreetMap tags (editable by anyone) and public submissions — i.e.
 * untrusted input. Interpolating them raw would be a stored-XSS vector.
 */
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
  const map = useRef<maplibregl.Map | null>(null);
  const refetchTimer = useRef<number | undefined>(undefined);
  const [count, setCount] = useState<number | null>(null);
  const [truncated, setTruncated] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<CameraFilters>({});
  const [manufacturers, setManufacturers] = useState<ManufacturerCount[]>([]);

  // loadCameras runs from map event handlers that are registered once, so
  // reading `filters` directly there would capture the initial value. A ref
  // keeps those handlers looking at current state.
  const filtersRef = useRef(filters);
  filtersRef.current = filters;

  useEffect(() => {
    if (!container.current || map.current) return;

    const m = new maplibregl.Map({
      container: container.current,
      style: STYLE,
      center: [-98.5, 39.8], // continental US
      zoom: 3.6,
    });
    map.current = m;

    m.addControl(new maplibregl.NavigationControl({ showCompass: false }), "top-right");
    m.addControl(new maplibregl.ScaleControl({ unit: "imperial" }), "bottom-right");

    // Surface style/tile failures instead of rendering a blank white page —
    // the failure mode that hid the OSM tile block during development.
    m.on("error", (e) => {
      console.error("MapLibre error:", e.error);
      setError("The base map failed to load. Camera data may still be listed below.");
    });

    m.on("load", () => {
      m.addSource("cameras", {
        type: "geojson",
        data: { type: "FeatureCollection", features: [] },
        // Clustering matters here, not just for looks: a full US import is
        // 100k+ points and drawing them individually at low zoom would
        // stall the browser.
        cluster: true,
        clusterRadius: 45,
        clusterMaxZoom: 13,
      });

      m.addLayer({
        id: "clusters",
        type: "circle",
        source: "cameras",
        filter: ["has", "point_count"],
        paint: {
          "circle-color": "#2c3e50",
          "circle-opacity": 0.82,
          "circle-radius": ["step", ["get", "point_count"], 14, 25, 19, 100, 25, 750, 32],
        },
      });

      m.addLayer({
        id: "cluster-count",
        type: "symbol",
        source: "cameras",
        filter: ["has", "point_count"],
        layout: {
          "text-field": ["get", "point_count_abbreviated"],
          "text-size": 12,
        },
        paint: { "text-color": "#ffffff" },
      });

      m.addLayer({
        id: "camera-point",
        type: "circle",
        source: "cameras",
        filter: ["!", ["has", "point_count"]],
        paint: {
          "circle-color": "#1b6b4a",
          "circle-radius": 5,
          "circle-stroke-width": 1.5,
          "circle-stroke-color": "#ffffff",
        },
      });

      m.on("click", "clusters", (e) => {
        const feature = m.queryRenderedFeatures(e.point, { layers: ["clusters"] })[0];
        const clusterId = feature?.properties?.cluster_id;
        if (clusterId == null) return;
        const source = m.getSource("cameras") as maplibregl.GeoJSONSource;
        void source.getClusterExpansionZoom(clusterId).then((zoom) => {
          m.easeTo({
            center: (feature.geometry as GeoJSON.Point).coordinates as [number, number],
            zoom,
          });
        });
      });

      m.on("click", "camera-point", (e) => {
        const f = e.features?.[0];
        if (!f) return;
        const p = f.properties ?? {};
        const [lng, lat] = (f.geometry as GeoJSON.Point).coordinates;
        new maplibregl.Popup({ closeButton: true })
          .setLngLat([lng, lat])
          .setHTML(
            `<h3>${escapeHtml(p.manufacturer ? String(p.manufacturer) : "ALPR camera")}</h3>
             <div>${p.camera_type ? escapeHtml(String(p.camera_type)) : "Type not recorded"}${
               p.direction !== undefined && p.direction !== null
                 ? ` · facing ${Number(p.direction)}°`
                 : ""
             }</div>
             <div class="popup-source">
               ${p.source === "osm_import" ? "Source: OpenStreetMap" : "Source: community report"}
             </div>`,
          )
          .addTo(m);
      });

      for (const layer of ["clusters", "camera-point"]) {
        m.on("mouseenter", layer, () => (m.getCanvas().style.cursor = "pointer"));
        m.on("mouseleave", layer, () => (m.getCanvas().style.cursor = ""));
      }

      void loadCameras(m);
      // Refetch for the visible area as the user pans/zooms. The API caps
      // each response at 1,000 rows and a single state can hold several
      // thousand, so fetching once without a bbox would silently show an
      // arbitrary subset of the country. Debounced so a drag doesn't fire
      // a request per frame.
      m.on("moveend", () => {
        if (refetchTimer.current) window.clearTimeout(refetchTimer.current);
        refetchTimer.current = window.setTimeout(() => void loadCameras(m), 300);
      });
    });

    return () => {
      if (refetchTimer.current) window.clearTimeout(refetchTimer.current);
      m.remove();
      map.current = null;
    };
  }, []);

  // Vendor list comes from the data, not a hardcoded array — OSM
  // contributors add manufacturers over time, and a static list goes
  // silently out of date (it once listed 4 while the data held 20+).
  // A failure here only costs the dropdown, so it doesn't surface an error.
  useEffect(() => {
    listManufacturers()
      .then(setManufacturers)
      .catch(() => setManufacturers([]));
  }, []);

  // Refetch when a filter changes (the map-event path only fires on pan/zoom).
  useEffect(() => {
    const m = map.current;
    if (m?.isStyleLoaded()) void loadCameras(m);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filters]);

  async function loadCameras(m: maplibregl.Map) {
    const bbox = clampBBox(m.getBounds());

    try {
      const cameras: CameraSighting[] = await listCameras(bbox, filtersRef.current);
      const source = m.getSource("cameras") as maplibregl.GeoJSONSource | undefined;
      if (!source) return;
      source.setData({
        type: "FeatureCollection",
        features: cameras.map((c) => ({
          type: "Feature" as const,
          geometry: { type: "Point" as const, coordinates: [c.lng, c.lat] },
          properties: {
            id: c.id,
            camera_type: c.camera_type ?? null,
            manufacturer: c.manufacturer ?? null,
            direction: c.direction ?? null,
            source: c.source,
          },
        })),
      });
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
          <span className="legend-dot" style={{ background: "#1b6b4a" }} />
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
