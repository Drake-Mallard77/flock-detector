import type { DeploymentStatus } from "../lib/api";

const LABELS: Record<DeploymentStatus, string> = {
  confirmed: "Confirmed",
  contract_found: "Contract found",
  osm_documented: "OSM documented",
  under_review: "Under review",
  disputed: "Disputed",
  removed: "Removed",
};

export default function StatusBadge({ status }: { status: DeploymentStatus }) {
  return (
    <span className={`status status-${status}`}>{LABELS[status] ?? status}</span>
  );
}
