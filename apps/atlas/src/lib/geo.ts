/**
 * Geometry for drawing which way a camera faces.
 *
 * OpenStreetMap records `direction` on a surveillance node as the compass
 * bearing the camera points: 0 is north, increasing clockwise. 87.6% of the
 * imported cameras carry one, which is why it's worth drawing rather than
 * leaving as a number in a popup.
 */

/** Mean Earth radius, metres. */
const EARTH_RADIUS_M = 6_371_008.8;

const toRad = (deg: number) => (deg * Math.PI) / 180;
const toDeg = (rad: number) => (rad * 180) / Math.PI;

/**
 * Point reached by travelling `distanceM` from [lat, lng] along `bearingDeg`.
 *
 * Great-circle rather than a flat-earth offset. At the tens of metres this
 * is used for the difference is far below a pixel, but the flat version
 * needs a cos(latitude) correction that is easy to omit and produces wedges
 * that skew increasingly with latitude — wrong in a way that looks fine in
 * Atlanta and obviously broken in Anchorage.
 */
export function destinationPoint(
  lat: number,
  lng: number,
  bearingDeg: number,
  distanceM: number,
): [number, number] {
  const d = distanceM / EARTH_RADIUS_M;
  const brg = toRad(bearingDeg);
  const lat1 = toRad(lat);
  const lng1 = toRad(lng);

  const lat2 = Math.asin(
    Math.sin(lat1) * Math.cos(d) + Math.cos(lat1) * Math.sin(d) * Math.cos(brg),
  );
  const lng2 =
    lng1 +
    Math.atan2(
      Math.sin(brg) * Math.sin(d) * Math.cos(lat1),
      Math.cos(d) - Math.sin(lat1) * Math.sin(lat2),
    );

  return [toDeg(lat2), toDeg(lng2)];
}

/**
 * Ground distance covered by one screen pixel at a given latitude and zoom,
 * in Web Mercator.
 *
 * Used to size the direction wedges in metres so they come out a constant
 * size on screen. A fixed ground size would be more physically honest, but
 * this project does not know any camera's actual read range, so a fixed
 * ground radius would be inventing a figure. A fixed screen size is plainly
 * a symbol rather than a measurement.
 */
export function metresPerPixel(lat: number, zoom: number): number {
  return (156543.03392 * Math.cos(toRad(lat))) / Math.pow(2, zoom);
}

/**
 * Vertices of a wedge centred on [lat, lng], opening `spreadDeg` wide around
 * `bearingDeg`.
 *
 * Deliberately a wedge and not an arrow. An arrow reads as "the camera is
 * over there"; a wedge reads as "this is the area it covers", which is
 * closer to what a direction tag means. The spread is not a claim about
 * field of view — it is wide enough to read at a glance and no wider.
 */
export function directionWedge(
  lat: number,
  lng: number,
  bearingDeg: number,
  radiusM: number,
  spreadDeg = 55,
  steps = 8,
): Array<[number, number]> {
  const start = bearingDeg - spreadDeg / 2;
  const points: Array<[number, number]> = [[lat, lng]];
  for (let i = 0; i <= steps; i++) {
    points.push(
      destinationPoint(lat, lng, start + (spreadDeg * i) / steps, radiusM),
    );
  }
  return points;
}
