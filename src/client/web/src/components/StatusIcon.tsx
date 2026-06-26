// StatusIcon — per-status SVG indicator (ADR-0078, supersedes ADR-0032
// "active-only spinner" rule).
//
// Every status renders an icon — active states (running / waiting) animate,
// terminal states (idle / stopped / pending / unknown) sit still or breathe
// gently. The element is always present in the DOM so callers get a stable
// layout slot regardless of status.
//
// Designs (24×24 viewBox, currentColor):
//   running  — faded full ring + 3/4 foreground arc, whole svg rotates
//   waiting  — three dots that bounce in sequence (ellipsis wave)
//   pending  — dashed ring rotating slowly
//   idle     — filled dot breathing (opacity pulse)
//   stopped  — solid rounded square, no motion (terminal state)
//   unknown  — short horizontal dash, no motion
//
// Caller contract:
//   - Color is currentColor; parent controls it via
//     .session-status-<status> / .run-state-<status>
//   - Size is 1em × 1em by default; parent can override
//   - active states ALSO carry the caller-supplied `activeClass` (typically
//     `run-state-spinner` or `session-status-spinner`) so existing DOM
//     contracts that query those class names continue to hold

import "../css/status-icon.css";

export type StatusKind = "running" | "waiting" | "idle" | "stopped" | "pending" | "unknown";

const KNOWN = new Set<string>(["running", "waiting", "idle", "stopped", "pending"]);
const ACTIVE = new Set<StatusKind>(["running", "waiting"]);

export function normalizeStatus(raw?: string): StatusKind {
  return (raw && KNOWN.has(raw) ? raw : "unknown") as StatusKind;
}

export function isActiveStatus(kind: StatusKind): boolean {
  return ACTIVE.has(kind);
}

export interface StatusIconProps {
  status: StatusKind;
  /** Class layered on active states only (e.g. legacy spinner contract). */
  activeClass?: string;
  /** Class layered on inactive states only. */
  inactiveClass?: string;
}

export function StatusIcon({ status, activeClass, inactiveClass }: StatusIconProps): JSX.Element {
  const active = isActiveStatus(status);
  const className = ["status-icon", `status-icon--${status}`, active ? activeClass : inactiveClass]
    .filter(Boolean)
    .join(" ");
  switch (status) {
    case "running":
      return (
        <svg className={className} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <circle className="status-icon__ring" cx="12" cy="12" r="9" />
          <path className="status-icon__arc" d="M21 12 A 9 9 0 0 1 12 21" />
        </svg>
      );
    case "waiting":
      return (
        <svg className={className} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <circle className="status-icon__dot status-icon__dot--1" cx="5" cy="12" r="2.4" />
          <circle className="status-icon__dot status-icon__dot--2" cx="12" cy="12" r="2.4" />
          <circle className="status-icon__dot status-icon__dot--3" cx="19" cy="12" r="2.4" />
        </svg>
      );
    case "pending":
      return (
        <svg className={className} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <circle className="status-icon__dashed" cx="12" cy="12" r="8" />
        </svg>
      );
    case "idle":
      return (
        <svg className={className} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <circle className="status-icon__filled" cx="12" cy="12" r="5" />
        </svg>
      );
    case "stopped":
      return (
        <svg className={className} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <rect className="status-icon__square" x="6" y="6" width="12" height="12" rx="2" />
        </svg>
      );
    default:
      return (
        <svg className={className} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <line className="status-icon__dash" x1="6" y1="12" x2="18" y2="12" />
        </svg>
      );
  }
}
