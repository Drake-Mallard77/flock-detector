import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";
import L from "leaflet";
import "leaflet.markercluster";

import PlaceSearch from "../components/PlaceSearch";
import { directionWedge, metresPerPixel } from "../lib/geo";
import { useTheme } from "../lib/theme";

import {
  listCameraClusters,
  listCameras,
  listManufacturers,
  type CameraClusters,
  type CameraFilters,
  type CameraSighting,
  type ManufacturerCount,
  type Place,
} from "../lib/api";

// Same-origin in production (Caddy proxies it); the Vite dev server proxies
// the same path locally. See apps/atlas/Caddyfile.
const TILE_BASE = import.meta.env.VITE_TILE_BASE ?? "/tiles";

// Mirrors the server-side LIMIT in services/api's handleListCameras. A
// response that hits exactly this was almost certainly truncated.
const API_LIMIT = 5000;

// Esri's Canvas basemaps hold real tiles only to zoom 16. Above that the
// service still answers HTTP 200 — with a grey placeholder reading "Map data
// not yet available", which is what turned the map into a blank watermarked
// sheet when you zoomed in one step too far.
//
// maxNativeZoom stops Leaflet requesting those levels at all and upscales
// the z16 tiles instead. Slightly soft, but continuous, and the camera
// markers stay crisp because they're vectors. maxZoom is kept higher than
// the tile ceiling deliberately: at street level the useful thing on screen
// is the dots and their spacing, not basemap detail.
const MAX_TILE_ZOOM = 16;
const MAX_ZOOM = 19;

// Above this many cameras in view, the map draws server-computed cluster
// bubbles instead of individual points. Below it, every camera in view is
// drawn individually.
//
// It's a density rule rather than a zoom rule on purpose: zoom alone is a
// poor proxy. Downtown Atlanta at zoom 10 holds ~1,100 cameras while most of
// Wyoming at the same zoom holds none, and any fixed zoom threshold is
// therefore wrong for one of them.
const RAW_POINT_THRESHOLD = 2000;

// Zoom at which cameras stop being clustered and direction wedges appear.
//
// Both switch together on purpose. A wedge is only meaningful once you can
// see which road a camera watches, and a wedge without its own dot — which
// is what clustering produces — is worse than no wedge at all.
const WEDGE_MIN_ZOOM = 15;

// Default view: the continental US.
const DEFAULT_CENTER: [number, number] = [39.8, -98.5];
const DEFAULT_ZOOM = 4;

/**
 * The map's view and filters live in the URL.
 *
 * Without this a map view can't be linked to. Someone looking at the
 * cameras around one city has no way to send that to anyone, cite it, or
 * bookmark it — reloading dropped them back to the whole country. For a
 * public-records project the citable link is close to the point.
 *
 * Centre and zoom rather than a bounding box: a bbox reproduces a
 * different view on a different sized window, while centre+zoom shows the
 * same place everywhere. It also matches what every other map on the web
 * puts in its URL, so the format is guessable.
 *
 * Five decimal places is a little over a metre — far finer than this data
 * claims to be, and enough to keep the URL short.
 */
function readViewFromParams(params: URLSearchParams): {
  center: [number, number];
  zoom: number;
} {
  const lat = Number(params.get("lat"));
  const lng = Number(params.get("lng"));
  const zoom = Number(params.get("z"));
  // Number("") is 0, so every value is range-checked rather than tested for
  // NaN alone — otherwise a missing lat silently means the equator.
  const valid =
    Number.isFinite(lat) && Math.abs(lat) <= 90 &&
    Number.isFinite(lng) && Math.abs(lng) <= 180 &&
    Number.isFinite(zoom) && zoom >= 1 && zoom <= MAX_ZOOM &&
    params.has("lat") && params.has("lng") && params.has("z");

  return valid
    ? { center: [lat, lng], zoom }
    : { center: DEFAULT_CENTER, zoom: DEFAULT_ZOOM };
}

// URL values are validated against the same sets the API accepts, not cast.
// A hand-edited or truncated link would otherwise send a value the API
// rejects with a 400, turning a slightly wrong URL into an error page
// instead of a map with one filter ignored.
const SOURCE_VALUES = ["osm_import", "user_submission"] as const;
const STATUS_VALUES = ["confirmed", "under_review"] as const;

function readFiltersFromParams(params: URLSearchParams): CameraFilters {
  const out: CameraFilters = {};

  const source = params.get("source");
  if (source && (SOURCE_VALUES as readonly string[]).includes(source)) {
    out.source = source as CameraFilters["source"];
  }

  const status = params.get("status");
  if (status && (STATUS_VALUES as readonly string[]).includes(status)) {
    out.status = status as CameraFilters["status"];
  }

  // Manufacturer is free text by nature — the list comes from the data and
  // changes as OSM contributors add vendors — so it can't be checked
  // against a fixed set. An unknown value simply matches nothing.
  const manufacturer = params.get("manufacturer");
  if (manufacturer) out.manufacturer = manufacturer;

  return out;
}

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
 * Tiles are Esri's Gray Canvas (Light or Dark to match the theme),
 * proxied through this origin. Others
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
const tileSet = (variant: "Light" | "Dark") => ({
  base: `${TILE_BASE}/ArcGIS/rest/services/Canvas/World_${variant}_Gray_Base/MapServer/tile/{z}/{y}/{x}`,
  labels: `${TILE_BASE}/ArcGIS/rest/services/Canvas/World_${variant}_Gray_Reference/MapServer/tile/{z}/{y}/{x}`,
});


const ATTRIBUTION =
  'Tiles &copy; Esri | Camera data &copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';

/**
 * Reads a CSS custom property off <html>.
 *
 * Leaflet paints markers onto a canvas/SVG with literal colour values, so
 * they can't inherit from the stylesheet the way DOM elements do. Pulling
 * the token at draw time keeps one source of truth for the palette instead
 * of duplicating hex codes here.
 */
function themeColor(name: string, fallback: string): string {
  if (typeof window === "undefined") return fallback;
  const v = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  return v || fallback;
}

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
  // Server-computed bubbles live on their own plain layer, deliberately not
  // inside the markercluster group. Putting them in it made markercluster
  // cluster the bubbles themselves and label each group with the number of
  // bubbles it had swallowed — so a national view of 136,008 cameras showed
  // "4", "3", "5". The counts are already final; they must not be re-grouped.
  const aggregateLayer = useRef<L.LayerGroup | null>(null);
  const refetchTimer = useRef<number | undefined>(undefined);
  // Guards against out-of-order responses; see loadCameras.
  const loadToken = useRef(0);
  const baseLayer = useRef<L.TileLayer | null>(null);
  const labelLayer = useRef<L.TileLayer | null>(null);
  const { active: theme } = useTheme();

  const [count, setCount] = useState<number | null>(null);
  const [truncated, setTruncated] = useState(false);
  // Whether the map is currently showing aggregated bubbles rather than one
  // marker per camera. The legend has to say which, or a bubble reading
  // "14k" looks like fourteen thousand individual dots failed to render.
  const [aggregated, setAggregated] = useState(false);
  // Whether direction wedges are on screen right now, so the legend
  // describes what is actually drawn rather than what might be.
  const [showingWedges, setShowingWedges] = useState(false);
  // Phones only — CSS keeps the controls open and the toggle hidden on
  // wider screens, so this state is inert there.
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [searchParams, setSearchParams] = useSearchParams();
  // Read once. After mount the map itself owns the view and writes to the
  // URL; treating the URL as the source of truth on every render would
  // fight Leaflet for control and re-centre the map mid-drag.
  const initialView = useRef(readViewFromParams(searchParams));
  const [filters, setFilters] = useState<CameraFilters>(() =>
    readFiltersFromParams(searchParams),
  );
  // setSearchParams identity changes each render; the map's event handlers
  // are registered once, so they read it through a ref.
  const setSearchParamsRef = useRef(setSearchParams);
  setSearchParamsRef.current = setSearchParams;
  const [manufacturers, setManufacturers] = useState<ManufacturerCount[]>([]);

  // Map event handlers are registered once, so reading `filters` directly
  // inside them would capture the initial value. A ref keeps them current.
  const filtersRef = useRef(filters);
  filtersRef.current = filters;

  // Shown on the collapsed toggle. Without it, a filter set on a phone and
  // then hidden behind the collapsed panel silently changes what the map
  // shows, with nothing on screen saying so.
  const activeFilterCount = Object.values(filters).filter(Boolean).length;

  useEffect(() => {
    if (!container.current || map.current) return;

    const m = L.map(container.current, {
      center: initialView.current.center,
      zoom: initialView.current.zoom,
      worldCopyJump: true,
      // Zoom buttons moved off the top-left, where they sat on top of the
      // filter bar and clipped the search input.
      zoomControl: false,
    });
    L.control.zoom({ position: "topright" }).addTo(m);
    map.current = m;

    const tiles = tileSet(theme === "dark" ? "Dark" : "Light");
    baseLayer.current = L.tileLayer(tiles.base, {
      attribution: ATTRIBUTION,
      maxZoom: MAX_ZOOM,
      maxNativeZoom: MAX_TILE_ZOOM,
    }).addTo(m);
    labelLayer.current = L.tileLayer(tiles.labels, {
      maxZoom: MAX_ZOOM,
      maxNativeZoom: MAX_TILE_ZOOM,
      pane: "shadowPane",
    }).addTo(m);

    // Clustering isn't cosmetic: a full US import is 100k+ points, and
    // drawing individual markers at low zoom would stall the browser.
    const cluster = L.markerClusterGroup({
      chunkedLoading: true,
      showCoverageOnHover: false,
      // Below this, dots group into bubbles; at or above it every camera is
      // drawn individually.
      //
      // It exists to keep the dots and the direction wedges telling the same
      // story. Wedges live on a plain layer and are never absorbed into a
      // bubble, so without a hard cut-off a street could show individual
      // wedges floating beside clustered dots — the same cameras rendered
      // two contradictory ways at once.
      disableClusteringAtZoom: WEDGE_MIN_ZOOM,
    });
    clusterLayer.current = cluster;
    m.addLayer(cluster);

    // Plain group: these bubbles already carry final counts and must not be
    // clustered again. See the aggregateLayer declaration above.
    const aggregates = L.layerGroup();
    aggregateLayer.current = aggregates;
    m.addLayer(aggregates);

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
      writeViewToUrl(m);
    });

    return () => {
      if (refetchTimer.current) window.clearTimeout(refetchTimer.current);
      observer.disconnect();
      m.remove();
      map.current = null;
      clusterLayer.current = null;
      aggregateLayer.current = null;
    };
  }, []);

  // Swap the basemap when the theme changes. Only the tile URLs change —
  // rebuilding the map would lose the viewport and reset the user's
  // position, which is worse than a brief tile reload.
  useEffect(() => {
    const m = map.current;
    if (!m || !baseLayer.current || !labelLayer.current) return;
    const tiles = tileSet(theme === "dark" ? "Dark" : "Light");
    baseLayer.current.setUrl(tiles.base);
    labelLayer.current.setUrl(tiles.labels);
    // Markers carry literal colours, so they need redrawing too.
    void loadCameras(m);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [theme]);

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

    // Each load is tagged so a slow response from an earlier viewport can't
    // overwrite a newer one. Panning fires these faster than they return,
    // and an out-of-order result paints markers for somewhere you've left.
    const token = ++loadToken.current;

    try {
      // Always ask for the aggregate first: it is cheap, and its total is
      // the only trustworthy answer to "how many cameras are in view" —
      // /cameras caps its response, so counting what it returns understates
      // reality by whatever the cap truncated.
      const summary = await listCameraClusters(bbox, m.getZoom(), filtersRef.current);
      if (token !== loadToken.current) return;

      if (summary.total > RAW_POINT_THRESHOLD) {
        drawClusterBubbles(summary);
        setCount(summary.total);
        setAggregated(true);
        setShowingWedges(false);
        setError(null);
        return;
      }

      const cameras: CameraSighting[] = await listCameras(bbox, filtersRef.current);
      if (token !== loadToken.current) return;
      const cluster = clusterLayer.current;
      if (!cluster) return;

      cluster.clearLayers();
      // Drop any bubbles from the previous, more zoomed-out draw; otherwise
      // a bubble counting these cameras sits on top of the cameras
      // themselves.
      aggregateLayer.current?.clearLayers();
      const stroke = themeColor("--bg", "#ffffff");
      const fill = themeColor("--status-confirmed", "#1b6b4a");

      // Wedges are sized in metres to land at a constant number of pixels,
      // so they stay legible as you zoom rather than vanishing or swamping
      // the map. Computed once per load from the viewport centre: over a
      // single screen the latitude term barely moves, and doing it per
      // camera would be thousands of cos() calls for a sub-pixel
      // difference.
      const wedgeRadiusM = metresPerPixel(m.getCenter().lat, m.getZoom()) * 22;

      const markers = cameras.map((c) => {
        const marker = L.circleMarker([c.lat, c.lng], {
          radius: 5,
          color: stroke,
          weight: 1.5,
          fillColor: fill,
          fillOpacity: 1,
        });
        const heading =
          c.direction !== undefined && c.direction !== null
            ? ` · facing ${Number(c.direction)}°`
            : "";
        // The other half of the camera/record join: from a dot on the map to
        // the agency it's attributed to. Only present when the camera
        // carries an operator tag that matched a record — about 15% of
        // imported cameras — so the link is offered rather than assumed.
        const recordLink = c.deployment_id
          ? `<div class="popup-record"><a href="/deployments/${encodeURIComponent(
              c.deployment_id,
            )}">View the agency record</a></div>`
          : "";

        marker.bindPopup(
          `<h3>${escapeHtml(c.manufacturer ?? "ALPR camera")}</h3>
           <div>${c.camera_type ? escapeHtml(c.camera_type) : "Type not recorded"}${heading}</div>
           <div class="popup-source">${
             c.source === "osm_import" ? "Source: OpenStreetMap" : "Source: community report"
           }</div>
           ${recordLink}`,
        );
        return marker;
      });
      cluster.addLayers(markers);

      // Direction wedges go on the plain layer, not into the cluster group.
      // markercluster hides a marker's geometry once it's absorbed into a
      // bubble, so a clustered wedge would disappear while its dot stayed —
      // and at these zooms the wedge is the informative half.
      //
      // Only for cameras that actually record a direction. Drawing a
      // default heading for the other 12% would invent a fact, on a map
      // whose whole claim is that it doesn't.
      // Matched to the clustering cut-off above: below it the dots are
      // grouped into bubbles, and loose wedges beside them would describe
      // cameras the map is not individually showing.
      const wedgeLayer = m.getZoom() >= WEDGE_MIN_ZOOM ? aggregateLayer.current : null;
      setShowingWedges(Boolean(wedgeLayer));
      if (wedgeLayer) {
        for (const c of cameras) {
          if (c.direction === undefined || c.direction === null) continue;
          L.polygon(directionWedge(c.lat, c.lng, Number(c.direction), wedgeRadiusM), {
            color: fill,
            weight: 0,
            fillColor: fill,
            // Faint: this is context around the dot, not a second dataset.
            // At full strength a dense street reads as a solid green smear.
            fillOpacity: 0.28,
            // The dot underneath stays the click target; the wedge would
            // otherwise swallow taps aimed at a camera behind it.
            interactive: false,
          }).addTo(wedgeLayer);
        }
      }

      setCount(cameras.length);
      setTruncated(cameras.length >= API_LIMIT);
      setAggregated(false);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load camera locations");
    }
  }

  // Draws one bubble per server-computed cell.
  //
  // These deliberately reuse markercluster's own class names so they inherit
  // the same brand styling as the client-side clusters shown when zoomed in.
  // Two visually different kinds of cluster bubble on one map would imply a
  // distinction that doesn't exist — both mean "this many cameras here".
  function drawClusterBubbles(summary: CameraClusters) {
    const aggregates = aggregateLayer.current;
    if (!aggregates) return;
    // Both layers are cleared on every draw. Leaving the other populated
    // would show the same cameras twice — once as points, once inside a
    // bubble counting them.
    clusterLayer.current?.clearLayers();
    aggregates.clearLayers();

    const markers = summary.clusters.map((c) => {
      // Matches markercluster's own size bands so a bubble doesn't change
      // size purely because the map crossed the aggregation threshold.
      const size = c.count < 100 ? "small" : c.count < 1000 ? "medium" : "large";
      const label =
        c.count >= 10000
          ? `${Math.round(c.count / 1000)}k`
          : c.count >= 1000
            ? `${(c.count / 1000).toFixed(1)}k`
            : String(c.count);

      return L.marker([c.lat, c.lng], {
        icon: L.divIcon({
          html: `<div><span>${label}</span></div>`,
          className: `marker-cluster marker-cluster-${size}`,
          iconSize: L.point(40, 40),
        }),
        // Announced to screen readers, which otherwise get an unlabelled
        // marker; the visible text is abbreviated ("14k") and the exact
        // figure is worth keeping available.
        title: `${c.count.toLocaleString()} cameras`,
      }).on("click", () => {
        // Zoom toward the cluster rather than opening a popup: at this
        // density the useful action is "show me what's inside".
        const m = map.current;
        if (m) m.setView([c.lat, c.lng], Math.min(m.getZoom() + 3, MAX_ZOOM));
      });
    });

    markers.forEach((mk) => aggregates.addLayer(mk));
  }

  function goToPlace(place: Place) {
    const m = map.current;
    if (!m) return;
    // Prefer the result's own bounding box so a city fills the viewport
    // rather than landing at an arbitrary zoom on its centre point.
    if (place.boundingBox) {
      const [south, north, west, east] = place.boundingBox;
      m.fitBounds([
        [south, west],
        [north, east],
      ]);
    } else {
      m.setView([place.lat, place.lng], 14);
    }
    // The moveend handler refetches cameras for the new viewport.
  }

  function setFilter(key: keyof CameraFilters, value: string) {
    setFilters((prev) => {
      const next = { ...prev };
      if (!value) delete next[key];
      else next[key] = value as never;
      return next;
    });

    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (value) next.set(key, value);
        else next.delete(key);
        return next;
      },
      // Changing a filter is a deliberate act, so unlike panning it earns a
      // history entry — Back should undo the filter.
      { replace: false },
    );
  }

  // Writes the current centre and zoom to the query string.
  //
  // replace: true, because panning a map is exploration, not navigation.
  // Pushing an entry per pan would bury whatever page the visitor arrived
  // from under dozens of near-identical map states, and Back would crawl
  // through them instead of leaving the map.
  function writeViewToUrl(m: L.Map) {
    const c = m.getCenter();
    setSearchParamsRef.current(
      (prev) => {
        const next = new URLSearchParams(prev);
        next.set("lat", c.lat.toFixed(5));
        next.set("lng", c.lng.toFixed(5));
        next.set("z", String(m.getZoom()));
        return next;
      },
      { replace: true },
    );
  }

  return (
    <div className="map-wrap">
      <div className="map-root" ref={container} />

      <div className={`map-filters${filtersOpen ? " is-open" : ""}`}>
        {/* Collapsed by default on phones, where the filter panel covered
            roughly a third of the map and the map is the entire point of
            the page. Hidden entirely above the breakpoint by CSS, so on a
            laptop the controls are simply always visible — no extra click
            introduced for the case that was already fine. */}
        <button
          type="button"
          className="map-filters-toggle"
          aria-expanded={filtersOpen}
          aria-controls="map-filter-controls"
          onClick={() => setFiltersOpen((open) => !open)}
        >
          {filtersOpen ? "Hide filters" : "Search and filter"}
          {activeFilterCount > 0 ? ` (${activeFilterCount})` : ""}
        </button>

        <div className="map-filters-body" id="map-filter-controls">
          <PlaceSearch onSelect={goToPlace} />

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
      </div>

      <div className="map-legend">
        <h2>Camera locations</h2>
        <div className="legend-row">
          <span className="legend-dot legend-dot-camera" />
          <span>Documented ALPR camera</span>
        </div>
        {/* Shown only when individual cameras are drawn — at aggregate zoom
            there are no wedges to explain, and a legend describing things
            that aren't on screen is just noise. */}
        {showingWedges && (
          <div className="legend-row">
            <span className="legend-wedge" />
            <span>Direction it faces, where recorded</span>
          </div>
        )}
        <p className="legend-note">
          {error
            ? error
            : count === null
              ? "Loading…"
              : aggregated
                ? // A real total now, not a capped one. The previous wording
                  // ("showing the first 1,000") described a limitation of the
                  // request rather than the data, and made a nationwide view
                  // of 136,000 cameras look like a sparse scattering.
                  `${count.toLocaleString()} cameras in view, grouped by area. Zoom in to see individual locations.`
                : truncated
                  ? `Showing ${count.toLocaleString()} cameras in view — there are more here. Zoom in to see them all.`
                  : `${count.toLocaleString()} camera${count === 1 ? "" : "s"} in view.`}
        </p>
      </div>
    </div>
  );
}
