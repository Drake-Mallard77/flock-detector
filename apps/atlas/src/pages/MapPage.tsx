import { useEffect, useRef, useState } from "react";
import maplibregl from "maplibre-gl";

import { listCameras, type CameraSighting } from "../lib/api";

// OSM raster tiles via a free public endpoint — no API key, no vendor
// lock-in, and consistent with the ODbL-attributed data underneath. If tile
// usage ever gets heavy enough to strain OSM's tile policy, this is the
// thing to swap for a self-hosted or paid vector source.
const STYLE: maplibregl.StyleSpecification = {
  version: 8,
  sources: {
    osm: {
      type: "raster",
      tiles: ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"],
      tileSize: 256,
      attribution: "© OpenStreetMap contributors",
    },
  },
  layers: [{ id: "osm", type: "raster", source: "osm" }],
};

// Mirrors the server-side LIMIT in services/api's handleListCameras. If the
// response hits exactly this, results were almost certainly truncated.
const API_LIMIT = 1000;

export default function MapPage() {
  const container = useRef<HTMLDivElement>(null);
  const map = useRef<maplibregl.Map | null>(null);
  const refetchTimer = useRef<number | undefined>(undefined);
  const [count, setCount] = useState<number | null>(null);
  const [truncated, setTruncated] = useState(false);
  const [error, setError] = useState<string | null>(null);

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
            `<h3>ALPR camera</h3>
             <div>${p.camera_type ? String(p.camera_type) : "Type unknown"}${
               p.direction !== undefined && p.direction !== null
                 ? ` · facing ${String(p.direction)}°`
                 : ""
             }</div>
             <div style="color:#6b6b66;margin-top:.3rem">
               ${p.source === "osm_import" ? "Source: OpenStreetMap" : "Source: user submission"}
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

  async function loadCameras(m: maplibregl.Map) {
    const b = m.getBounds();
    const bbox: [number, number, number, number] = [
      b.getWest(),
      b.getSouth(),
      b.getEast(),
      b.getNorth(),
    ];

    try {
      const cameras: CameraSighting[] = await listCameras(bbox);
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

  return (
    <div className="map-wrap">
      <div className="map-root" ref={container} />
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
