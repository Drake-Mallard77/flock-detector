// Typed client for the FlockWatch Go API (services/api). Kept hand-written
// rather than generated for now — the surface is small enough that a
// generator would be more machinery than it saves. See
// packages/shared-types if/when this needs to be shared with the mobile app.

const API_BASE =
  import.meta.env.VITE_API_BASE ?? "https://api.theflockwatcher.com";

export type DeploymentStatus =
  | "confirmed"
  | "contract_found"
  /** Published, but mapped in OpenStreetMap rather than verified against
   *  public records. Deliberately distinct from "confirmed". */
  | "osm_documented"
  | "under_review"
  | "disputed"
  | "removed";

export type EvidenceType =
  | "council_report"
  | "contract"
  | "invoice"
  | "news_article"
  | "foia_response"
  | "user_photo"
  | "osm_import";

export interface Deployment {
  id: string;
  agency_name: string;
  /** Readable URL segment, unique within the state. */
  slug: string;
  city: string;
  state: string;
  county?: string;
  lat?: number;
  lng?: number;
  documented_units?: number;
  evidence_type: EvidenceType;
  source_links: string[];
  status: DeploymentStatus;
  notes?: string;
  /** law_enforcement | government | education | private | unknown */
  operator_type?: string;
  last_reviewed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface CameraSighting {
  id: string;
  deployment_id?: string;
  lat: number;
  lng: number;
  direction?: number;
  camera_type?: string;
  /** Absent when OpenStreetMap didn't record one. */
  manufacturer?: string;
  photo_url?: string;
  source: "user_submission" | "osm_import";
  status: string;
  external_id?: string;
  state?: string;
  created_at: string;
}

export interface CameraFilters {
  source?: "osm_import" | "user_submission";
  status?: "confirmed" | "under_review";
  /** A manufacturer name, or the literal "unknown" for rows OSM left blank. */
  manufacturer?: string;
}

export interface NewDeployment {
  agency_name: string;
  city: string;
  state: string;
  county?: string;
  documented_units?: number;
  evidence_type: EvidenceType;
  source_links: string[];
  notes?: string;
}

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// Held in memory only. The provider in auth.tsx is responsible for
// persisting it; keeping the module's copy in memory means a stale token in
// storage can't silently outlive an explicit sign-out in this tab.
let authToken: string | null = null;

export function setAuthToken(token: string | null) {
  authToken = token;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(authToken ? { Authorization: `Bearer ${authToken}` } : {}),
      ...init?.headers,
    },
  });

  if (!res.ok) {
    // The API returns {"error": "..."} for its own failures; fall back to
    // the status text for anything that didn't come from the app itself
    // (e.g. an infrastructure-level error page).
    let message = res.statusText;
    try {
      const body = await res.json();
      if (typeof body?.error === "string") message = body.error;
    } catch {
      // Non-JSON response; keep statusText.
    }
    throw new ApiError(message, res.status);
  }

  return res.json() as Promise<T>;
}

export interface DeploymentFilters {
  state?: string;
  status?: DeploymentStatus;
  city?: string;
  /** Free-text search across agency, city, and state. Applied server-side. */
  q?: string;
  limit?: number;
  offset?: number;
}

export function listDeployments(filters: DeploymentFilters = {}) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) {
    if (value !== undefined && value !== "") params.set(key, String(value));
  }
  const qs = params.toString();
  return request<Deployment[]>(`/deployments${qs ? `?${qs}` : ""}`);
}

export function getDeployment(id: string) {
  return request<Deployment>(`/deployments/${id}`);
}

export function createDeployment(body: NewDeployment) {
  return request<{ id: string; status: string }>("/deployments", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export interface Me {
  user_id: string;
  email: string;
  role: "submitter" | "moderator" | "admin";
}

export function googleLogin(credential: string) {
  return request<{ token: string; email: string; role: string }>("/auth/google", {
    method: "POST",
    body: JSON.stringify({ credential }),
  });
}

export function getMe() {
  return request<Me>("/auth/me");
}

/**
 * Sets the same status on many deployments at once.
 *
 * Exists for the OSM-derived candidate queue, which runs to hundreds of
 * records. The server caps a single call at 200 and records the reviewer on
 * every row, so a bulk decision stays as attributable as an individual one.
 */
export function bulkReviewDeployments(ids: string[], status: DeploymentStatus) {
  return request<{ status: string; updated: number }>("/deployments/bulk-review", {
    method: "POST",
    body: JSON.stringify({ ids, status }),
  });
}

export function reviewDeployment(id: string, status: DeploymentStatus) {
  return request<{ id: string; status: string }>(`/deployments/${id}/review`, {
    method: "POST",
    body: JSON.stringify({ status }),
  });
}

export interface ManufacturerCount {
  manufacturer: string;
  count: number;
}

/** Manufacturers actually present in the data, most common first. */
export function listManufacturers() {
  return request<ManufacturerCount[]>("/cameras/manufacturers");
}

/** bbox is [west, south, east, north]. */
export function listCameras(
  bbox?: [number, number, number, number],
  filters: CameraFilters = {},
) {
  const params = new URLSearchParams();
  if (bbox) params.set("bbox", bbox.join(","));
  for (const [key, value] of Object.entries(filters)) {
    if (value) params.set(key, value);
  }
  const qs = params.toString();
  return request<CameraSighting[]>(`/cameras${qs ? `?${qs}` : ""}`);
}

export interface CameraCluster {
  lat: number;
  lng: number;
  count: number;
}

export interface CameraClusters {
  /**
   * Every camera matching the filters in this viewport — a true count, not
   * a capped one. /cameras caps its response, so at low zoom its length
   * says nothing about how many cameras are actually there.
   */
  total: number;
  cell_size_deg: number;
  clusters: CameraCluster[];
}

/**
 * Server-side aggregation for zoomed-out views, where returning individual
 * points would mean either an unusable payload or a misleading sample.
 *
 * zoom controls the grid cell size only, so bubbles stay a constant size on
 * screen instead of fragmenting as you zoom.
 */
export function listCameraClusters(
  bbox: [number, number, number, number],
  zoom: number,
  filters: CameraFilters = {},
) {
  const params = new URLSearchParams();
  params.set("bbox", bbox.join(","));
  params.set("zoom", String(Math.round(zoom)));
  for (const [key, value] of Object.entries(filters)) {
    if (value) params.set(key, value);
  }
  return request<CameraClusters>(`/cameras/clusters?${params.toString()}`);
}

export interface Place {
  label: string;
  lat: number;
  lng: number;
  /** [south, north, west, east] when the result covers an area. */
  boundingBox?: [number, number, number, number];
}

/**
 * Looks up a place name for the map's search box.
 *
 * Proxied through this origin (/geocode) rather than called directly, for
 * the same reasons as the map tiles: tracker blockers drop third-party
 * requests, and the geocoder never learns who searched for which address.
 * That second point matters more than usual here — a search on this site
 * can reveal where someone lives or is going.
 */
export async function searchPlaces(query: string, signal?: AbortSignal): Promise<Place[]> {
  const params = new URLSearchParams({
    q: query,
    format: "json",
    limit: "5",
    countrycodes: "us",
    addressdetails: "0",
  });
  const res = await fetch(`/geocode/search?${params}`, { signal });
  if (!res.ok) throw new ApiError("Place search is unavailable", res.status);

  const raw = (await res.json()) as Array<{
    display_name: string;
    lat: string;
    lon: string;
    boundingbox?: [string, string, string, string];
  }>;

  return raw.map((r) => ({
    label: r.display_name,
    lat: Number(r.lat),
    lng: Number(r.lon),
    boundingBox: r.boundingbox
      ? [Number(r.boundingbox[0]), Number(r.boundingbox[1]), Number(r.boundingbox[2]), Number(r.boundingbox[3])]
      : undefined,
  }));
}

export interface DeploymentCamera {
  lat: number;
  lng: number;
  direction?: number;
}

export interface DeploymentCameras {
  /**
   * Camera locations attributed to this agency.
   *
   * Not the same figure as the record's documented_units, and usually
   * lower: documented_units is what the evidence says the agency operates,
   * while this counts the ones OpenStreetMap contributors have mapped and
   * tagged with an operator. Presenting either as the other would overstate
   * what's actually known.
   */
  linked: number;
  cameras: DeploymentCamera[];
}

/** Camera locations linked to one deployment record. */
export function listDeploymentCameras(id: string) {
  return request<DeploymentCameras>(`/deployments/${encodeURIComponent(id)}/cameras`);
}

export interface StateStat {
  /** Two-letter USPS code. */
  state: string;
  /** Published agency records only — the set the atlas stands behind. */
  deployments: number;
  /** Every imported camera in the state. */
  cameras: number;
}

/** Per-state totals, for the location index. */
export function listStateStats() {
  return request<StateStat[]>("/stats/states");
}

/**
 * Resolves a record from its readable URL.
 *
 * The UUID form still works and always will — links to it are already
 * published and indexed, and a public-records project breaking its own
 * citations to tidy up URLs would be a poor trade.
 */
export function getDeploymentBySlug(state: string, slug: string) {
  return request<Deployment>(
    `/deployments/by-slug/${encodeURIComponent(state)}/${encodeURIComponent(slug)}`,
  );
}

/** Canonical path for a record. */
export function recordPath(d: Pick<Deployment, "state" | "slug">) {
  return `/state/${d.state.toLowerCase()}/${d.slug}`;
}

export interface DuplicateRecord {
  id: string;
  agency_name: string;
  city: string;
  state: string;
  status: DeploymentStatus;
  slug: string;
  documented_units?: number;
  /** Cameras currently attributed to this record. */
  linked_cameras: number;
  created_at: string;
}

export interface DuplicateGroup {
  state: string;
  records: DuplicateRecord[];
}

/**
 * Records the atlas holds more than once for the same agency.
 *
 * Grouped by the same normalised name the importer now uses to decide
 * identity, so this lists exactly what would have been treated as one
 * record had that check always ignored punctuation.
 */
export function listDuplicates() {
  return request<DuplicateGroup[]>("/deployments/duplicates");
}

/**
 * Folds `duplicateId` into `survivorId`: moves its cameras across and
 * retires it, pointing at where it went. Nothing is deleted.
 */
export function mergeDeployment(survivorId: string, duplicateId: string) {
  return request<{ survivor_id: string; duplicate_id: string; cameras_moved: number }>(
    `/deployments/${encodeURIComponent(survivorId)}/merge`,
    { method: "POST", body: JSON.stringify({ duplicate_id: duplicateId }) },
  );
}
