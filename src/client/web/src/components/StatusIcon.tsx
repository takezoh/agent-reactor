// StatusIcon — per-status indicator (ADR-0078, supersedes ADR-0032).
//
// running / pending rotation lives on the outer <span> because SVG-element
// rotation is unreliable on Safari / older WebKit. The <span> is a plain
// inline-block whose CSS transform is universally supported.

import type { ReactNode } from "react";
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

// `satisfies` enforces exhaustive coverage at compile time: adding a new
// StatusKind without a GEOMETRY entry is a type error, not silent breakage.
const GEOMETRY = {
  running: (
    <>
      <circle className="status-icon__ring" cx="12" cy="12" r="9" />
      <path className="status-icon__arc" d="M21 12 A 9 9 0 1 1 12 3" />
    </>
  ),
  waiting: (
    <>
      <circle className="status-icon__dot status-icon__dot--1" cx="5" cy="12" r="2.4" />
      <circle className="status-icon__dot status-icon__dot--2" cx="12" cy="12" r="2.4" />
      <circle className="status-icon__dot status-icon__dot--3" cx="19" cy="12" r="2.4" />
    </>
  ),
  pending: <circle className="status-icon__dashed" cx="12" cy="12" r="8" />,
  idle: <circle className="status-icon__filled" cx="12" cy="12" r="5" />,
  stopped: <rect className="status-icon__square" x="6" y="6" width="12" height="12" rx="2" />,
  unknown: <line className="status-icon__dash" x1="6" y1="12" x2="18" y2="12" />,
} satisfies Record<StatusKind, ReactNode>;

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
  return (
    <span className={className} aria-hidden="true">
      <svg viewBox="0 0 24 24" focusable="false">{GEOMETRY[status]}</svg>
    </span>
  );
}
