import type { Connection } from "../socket/connection";
import "../css/view.css";
import { useDaemonStore } from "../store/daemon";
import type { Card } from "../wire/server";

export function displayLabel(card: Card, id: string): string {
  const t = card.title?.trim();
  if (t) return t;
  const s = card.subtitle?.trim();
  if (s) return s;
  return id;
}

const KNOWN = new Set(["running", "waiting", "idle", "stopped", "pending"]);
const ACTIVE = new Set(["running", "waiting"]);

function normalizeStatus(status?: string): string {
  return status && KNOWN.has(status) ? status : "unknown";
}

// conn is retained in the prop signature for API compatibility; SessionList
// does not own subscriptions (ADR 0030) — TerminalPane is the sole owner.
export function SessionList({ conn: _conn }: { conn: Connection }) {
  const sessions = useDaemonStore((s) => s.sessions);
  const activeId = useDaemonStore((s) => s.activeSessionID);
  const selectSession = useDaemonStore((s) => s.selectSession);

  return (
    <ul className="session-list" aria-label="sessions">
      {sessions.map((s) => {
        const normalized = normalizeStatus(s.view.status);
        const active = ACTIVE.has(normalized);
        return (
          <li key={s.id}>
            <button
              type="button"
              className={s.id === activeId ? "active" : ""}
              onClick={() => {
                selectSession(s.id);
              }}
            >
              <span
                className={`session-status-slot session-status-${normalized}`}
                aria-label={`status: ${normalized}`}
                title={normalized}
              >
                {active && <span className="session-status-spinner" aria-hidden="true" />}
              </span>
              <span className="title">{displayLabel(s.view.card, s.id)}</span>
            </button>
          </li>
        );
      })}
    </ul>
  );
}
