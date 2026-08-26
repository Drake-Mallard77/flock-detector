// Typed client for the FlockWatch Go API (services/api). Kept hand-written
// rather than generated for now — the surface is small enough that a
// generator would be more machinery than it saves. See
// packages/shared-types if/when this needs to be shared with the mobile app.

const API_BASE =
  import.meta.env.VITE_API_BASE ?? "https://flockwatch-api-wlfs54kbla-uc.a.run.app";

export type DeploymentStatus =
  | "confirmed"
  | "contract_found"
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
