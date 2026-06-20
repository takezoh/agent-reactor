import "../css/view.css";

export type RunStateBadgeProps = {
  status?: string;
};

const KNOWN = new Set(["running", "waiting", "idle", "stopped", "pending"]);

export function RunStateBadge({ status }: RunStateBadgeProps) {
  const normalized = status && KNOWN.has(status) ? status : "unknown";
  return (
    <span
      className={`run-state-badge run-state-${normalized}`}
      aria-label={`status: ${normalized}`}
    >
      {normalized}
    </span>
  );
}
