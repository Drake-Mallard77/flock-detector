import { useEffect, useRef } from "react";
import { Link } from "react-router-dom";
import L from "leaflet";

import type { DeploymentCameras } from "../lib/api";

const TILE_BASE = import.meta.env.VITE_TILE_BASE ?? "/tiles";
// Matches MapPage: Esri's Canvas basemaps have no tiles above 16 and serve a
// "map data not yet available" placeholder instead.
const MAX_TILE_ZOOM = 16;
const MAX_ZOOM = 19;

/**
 * The camera locations attributed to one agency record.
 *
 * These two datasets were unconnected until now: the map knew where cameras
 * are, the records knew who bought them, and nothing joined the two. The
 * join is the interesting part — "97 documented units" means much more
 * beside the streets those cameras actually sit on.
 *
 * Renders nothing when no cameras are linked, which is the majority case:
 * only about 15% of imported cameras carry an operator tag in OpenStreetMap.
 * An empty map would imply this agency has no cameras, which is a different
 * and false claim.
 */
export default function RecordMap({
  data,
  agency,
}: {
  data: DeploymentCameras;
  agency: string;
}) {
  const container = useRef<HTMLDivElement>(null);
  const map = useRef<L.Map | null>(null);

  useEffect(() => {
    if (!container.current || map.current || data.cameras.length === 0) return;

    const points: [number, number][] = data.cameras.map((c) => [c.lat, c.lng]);
    const m = L.map(container.current, {
      // A record map is a picture, not an exploration surface. Scroll-wheel
      // zoom would hijack the page scroll on the way past it.
      scrollWheelZoom: false,
      zoomControl: true,
    });
    map.current = m;

    L.tileLayer(
      `${TILE_BASE}/ArcGIS/rest/services/Canvas/World_Light_Gray_Base/MapServer/tile/{z}/{y}/{x}`,
      {
        attribution:
          'Tiles &copy; Esri | Camera data &copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
        maxZoom: MAX_ZOOM,
        maxNativeZoom: MAX_TILE_ZOOM,
      },
    ).addTo(m);

    const layer = L.layerGroup().addTo(m);
    for (const c of data.cameras) {
      L.circleMarker([c.lat, c.lng], {
        radius: 5,
        color: "#ffffff",
        weight: 1.5,
        fillColor: "#1b6b4a",
        fillOpacity: 1,
      }).addTo(layer);
    }

    // fitBounds with padding, then a zoom ceiling: a record whose cameras
    // sit on one street would otherwise open at maximum zoom showing a
    // single building, with no sense of where in the city it is.
    m.fitBounds(L.latLngBounds(points), { padding: [24, 24] });
    if (m.getZoom() > 15) m.setZoom(15);

    const observer = new ResizeObserver(() => m.invalidateSize());
    observer.observe(container.current);

    return () => {
      observer.disconnect();
      m.remove();
      map.current = null;
    };
  }, [data]);

  if (data.cameras.length === 0) {
    return (
      <p className="record-map-empty">
        No individual camera locations are attributed to this agency yet.
        OpenStreetMap records an operator for only some ALPR cameras, so a
        record can be well documented while none of its cameras are
        individually mapped.
      </p>
    );
  }

  // Centre of the linked cameras, so "view on the main map" opens where the
  // cameras are rather than at the record's stored centroid.
  const lat =
    data.cameras.reduce((sum, c) => sum + c.lat, 0) / data.cameras.length;
  const lng =
    data.cameras.reduce((sum, c) => sum + c.lng, 0) / data.cameras.length;

  return (
    <div className="record-map-wrap">
      <div className="record-map" ref={container} />
      <p className="record-map-note">
        {data.linked.toLocaleString()} mapped camera
        {data.linked === 1 ? "" : " locations"} attributed to {agency}.{" "}
        <Link to={`/?lat=${lat.toFixed(5)}&lng=${lng.toFixed(5)}&z=13`}>
          View on the main map
        </Link>
      </p>
    </div>
  );
}
