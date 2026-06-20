import type { Connection } from "../socket/connection";
import { useDaemonStore } from "../store/daemon";
import { RunStateBadge } from "./RunStateBadge";

export function SessionList({ conn }: { conn: Connection }) {
  const sessions = useDaemonStore((s) => s.sessions);
  const activeId = useDaemonStore((s) => s.activeSessionID);
  const selectSession = useDaemonStore((s) => s.selectSession);

  return (
    <ul className="session-list" aria-label="sessions">
      {sessions.map((s) => (
        <li key={s.id}>
          <button
            type="button"
            className={s.id === activeId ? "active" : ""}
            onClick={async () => {
              if (activeId && activeId !== s.id) {
                await conn.unsubscribe(activeId);
              }
              selectSession(s.id);
              await conn.subscribe(s.id);
            }}
          >
            <span className="title">{s.view.card.title ?? s.id}</span>
            <RunStateBadge status={s.view.status} />
          </button>
        </li>
      ))}
    </ul>
  );
}
