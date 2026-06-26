import "../css/view.css";
import { StatusIcon, normalizeStatus } from "./StatusIcon";

export type RunStateBadgeProps = {
  status?: string;
};

export function RunStateBadge({ status }: RunStateBadgeProps) {
  const kind = normalizeStatus(status);
  return (
    <span className={`run-state-badge run-state-${kind}`} aria-label={`status: ${kind}`}>
      <StatusIcon status={kind} activeClass="run-state-spinner" inactiveClass="run-state-icon" />
      {kind}
    </span>
  );
}
