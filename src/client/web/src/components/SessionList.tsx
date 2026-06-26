// SessionList — sidebar session picker, partitioned by workspace and grouped
// by project. Mirrors the arc TUI sidebar structure (workspace switcher row
// → alphabetical project headers with fold toggles → sessions under each).
//
// Layered listbox model:
//   - Workspace switcher: SegmentedControl (radiogroup) — only rendered when
//     ≥ 2 workspaces exist (TUI workspaceBarVisible parity).
//   - Per-project section: disclosure button (aria-expanded) + nested
//     UnifiedListbox of sessions. Each project has its own cursor; this is
//     deliberate — keeps keyboard nav contained inside a project so the
//     active-session highlight never drifts to a different project.
//
// ADRs retained:
//   - ADR-0076 (2-slot Title + Subtitle, "New Session" placeholder, supersedes 0033)
//   - ADR-0032 (session-status-slot + session-status-spinner kept)
//   - ADR-0030 (conn prop retained for API compat; SessionList does not own
//     subscriptions — TerminalPane owns them)
//   - FR-A11Y-001 (44×44px touch target on every interactive row)
//   - FR-TOKEN-001/002 (--row-* sizing tokens shared with palette listbox)

import { useEffect, useId, useMemo, useState } from "react";
import { driverColor } from "../lib/driverColor";
import type { Connection } from "../socket/connection";
import "../css/view.css";
import {
  DEFAULT_WORKSPACE,
  groupSessionsByProject,
  selectDistinctWorkspaces,
  useDaemonStore,
} from "../store/daemon";
import type { Card, SessionInfo } from "../wire/server";
import { SegmentedControl } from "./primitives/SegmentedControl";
import { TagPill } from "./primitives/TagPill";
import { UnifiedListbox } from "./primitives/UnifiedListbox";

// ---------------------------------------------------------------------------
// Title / Subtitle slot policy (ADR-0076)
//
// The session ID is NEVER rendered as user-visible text — operators do not
// need to read it to identify a session. When both Title and Subtitle are
// empty the Title slot falls back to TITLE_PLACEHOLDER; the Subtitle row
// is hidden entirely when there is nothing to show.
// ---------------------------------------------------------------------------

export const TITLE_PLACEHOLDER = "New Session";

export function titleText(card: Card): string {
  return card.title?.trim() || TITLE_PLACEHOLDER;
}

export function subtitleText(card: Card): string {
  return card.subtitle?.trim() ?? "";
}

/**
 * @deprecated Use {@link titleText} / {@link subtitleText} directly. Kept
 * only for tests that still target the legacy 1-slot chain.
 */
export function displayLabel(card: Card, _id: string): string {
  return titleText(card);
}

// ---------------------------------------------------------------------------
// Status helpers (ADR-0032)
// ---------------------------------------------------------------------------

const KNOWN = new Set(["running", "waiting", "idle", "stopped", "pending"]);
const ACTIVE = new Set(["running", "waiting"]);

function normalizeStatus(status?: string): string {
  return status && KNOWN.has(status) ? status : "unknown";
}

// ---------------------------------------------------------------------------
// SessionRow — one row rendered inside UnifiedListbox as label prop
// ---------------------------------------------------------------------------
//
// Card layout (ADR-0076 — Title + Subtitle as complementary slots):
//
//   ┃ ● <title or "New Session">      [driver]
//   ┃   <subtitle (CSS-clamped to ≈25ch, ellipsis)>
//   ┃   [tag] [tag]  <border_badge>
//
// Title is always shown (with TITLE_PLACEHOLDER fallback). Subtitle is a
// separate row, rendered only when non-empty. Width clamping happens in
// CSS (max-width + text-overflow: ellipsis) so the full string stays in
// the DOM — copy/find/screen-readers all see the original text. The Go
// driver layer also caps the raw value at 30 code-points as a backstop.

interface SessionRowProps {
  session: SessionInfo;
  isActive: boolean;
}

function SessionRow({ session, isActive }: SessionRowProps) {
  const card = session.view.card;
  const status = session.view.status;
  const normalized = normalizeStatus(status);
  const activeRun = ACTIVE.has(normalized);
  const title = titleText(card);
  const subtitle = subtitleText(card);

  const driver = session.root_driver?.trim() || undefined;
  // Memoize the chip style by driver name so the {backgroundColor, color}
  // literal keeps stable identity across renders (React's reconciler can
  // skip the style attribute update when the object hasn't changed).
  const driverStyle = useMemo(() => {
    if (!driver) return undefined;
    const c = driverColor(driver);
    return { backgroundColor: c.bg, color: c.fg };
  }, [driver]);
  const tags = card.tags ?? [];
  const borderBadge = card.border_badge;
  const showTags = tags.length > 0 || Boolean(borderBadge);

  return (
    <div
      className={["session-list__row", isActive ? "session-list__row--active" : ""]
        .filter(Boolean)
        .join(" ")}
      data-session-id={session.id}
    >
      <span
        className={`session-status-slot session-status-${normalized}`}
        aria-label={`status: ${normalized}`}
        title={normalized}
      >
        {activeRun && <span className="session-status-spinner" aria-hidden="true" />}
      </span>
      <div className="session-list__content">
        <div className="session-list__title-row">
          <span className="session-list__title title">{title}</span>
          {driver && (
            <span
              className="session-list__driver"
              aria-label={`driver: ${driver}`}
              title={driver}
              style={driverStyle}
            >
              {driver}
            </span>
          )}
        </div>
        {subtitle && <div className="session-list__subtitle">{subtitle}</div>}
        {showTags && (
          <div className="session-list__tags" aria-label="session tags">
            {tags.map((t, i) => (
              <TagPill key={`${i}-${t.text}`} tag={t} className="session-list__tag" />
            ))}
            {borderBadge && <span className="session-list__badge">{borderBadge}</span>}
          </div>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// WorkspaceSwitcher — segmented control wrapping the daemon store's
// selectedWorkspace state. Renders nothing when ≤ 1 workspace exists.
// ---------------------------------------------------------------------------

interface WorkspaceSwitcherProps {
  workspaces: string[];
  selected: string;
  onChange: (next: string) => void;
}

function WorkspaceSwitcher({ workspaces, selected, onChange }: WorkspaceSwitcherProps) {
  if (workspaces.length < 2) return null;
  return (
    <div className="session-list__workspace-bar" data-role="workspace-switcher">
      <SegmentedControl
        ariaLabel="workspaces"
        segments={workspaces.map((w) => ({ value: w, label: w }))}
        value={selected}
        onChange={onChange}
        idPrefix="ws"
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// ProjectGroup — disclosure header + nested UnifiedListbox of sessions
// ---------------------------------------------------------------------------

interface ProjectGroupProps {
  /** Display name (basename of projectPath). */
  project: string;
  /** Full path — the unique identity used as fold-state key. */
  projectPath: string;
  sessions: SessionInfo[];
  folded: boolean;
  /** Receives the projectPath (not the display name) so distinct paths
   *  with the same basename can be folded independently. */
  onToggleFold: (projectPath: string) => void;
  activeId: string | null;
  daemonDisconnected: boolean;
  selectSession: (id: string) => void;
}

function ProjectGroup({
  project,
  projectPath,
  sessions,
  folded,
  onToggleFold,
  activeId,
  daemonDisconnected,
  selectSession,
}: ProjectGroupProps) {
  // Per-project cursor (aria-activedescendant). Independent from
  // activeSessionID so ArrowDown/Up navigates without committing. When the
  // committed selection lands in this project, sync the cursor onto it.
  const activeInGroup = sessions.some((s) => s.id === activeId);
  const [cursorId, setCursorId] = useState<string | null>(
    activeInGroup ? activeId : (sessions[0]?.id ?? null),
  );
  // Re-sync cursorId on activeId / session-set changes. Uses a functional
  // update so cursorId itself doesn't need to be in the dep list — that
  // would re-trigger the effect on every ArrowDown and snap the cursor
  // back to activeId. Two branches:
  //   - active session is in this group → cursor follows it
  //   - current cursor target was removed → reset to first row (code-review
  //     #4: view-update can delete the cursored session while active lives
  //     elsewhere; without this the cursor would dangle on an unknown id)
  useEffect(() => {
    setCursorId((prev) => {
      if (activeInGroup) return activeId;
      if (prev !== null && !sessions.some((s) => s.id === prev)) {
        return sessions[0]?.id ?? null;
      }
      return prev;
    });
  }, [activeId, activeInGroup, sessions]);

  // useId gives a stable, valid id token without exposing the (possibly
  // free-form) project path to HTML id / aria-controls IDREF rules
  // (code-review #3). The static suffixes make the relation between the
  // disclosure button and its panel readable in devtools.
  const uid = useId();
  const headerId = `${uid}-header`;
  const panelId = `${uid}-panel`;

  return (
    <section className="session-list__project" data-role="project-group">
      <button
        type="button"
        id={headerId}
        className="session-list__project-header"
        aria-expanded={!folded}
        aria-controls={panelId}
        onClick={() => onToggleFold(projectPath)}
        title={projectPath}
      >
        <span className="session-list__project-chevron" aria-hidden="true">
          {folded ? "▶" : "▼"}
        </span>
        <span className="session-list__project-name">{project}</span>
        <span className="session-list__project-count" aria-hidden="true">
          {sessions.length}
        </span>
      </button>
      {!folded && (
        <section id={panelId} aria-labelledby={headerId} className="session-list__project-panel">
          <UnifiedListbox
            ariaLabel={`sessions in ${project}`}
            items={sessions.map((s) => ({
              id: s.id,
              label: <SessionRow session={s} isActive={s.id === activeId} />,
              disabled: daemonDisconnected,
              disabledReason: daemonDisconnected ? "Daemon disconnected" : undefined,
            }))}
            activeId={cursorId}
            onActiveChange={(id) => setCursorId(id)}
            onActivate={(id) => selectSession(id)}
          />
        </section>
      )}
    </section>
  );
}

// ---------------------------------------------------------------------------
// SessionList
// ---------------------------------------------------------------------------

// conn is retained in the prop signature for API compatibility; SessionList
// does not own subscriptions (ADR 0030) — TerminalPane is the sole owner.
export function SessionList({ conn: _conn }: { conn: Connection }) {
  const sessions = useDaemonStore((s) => s.sessions);
  const activeId = useDaemonStore((s) => s.activeSessionID);
  const selectSession = useDaemonStore((s) => s.selectSession);
  const daemonDisconnected = useDaemonStore((s) => s.daemonDisconnected);
  const selectedWorkspace = useDaemonStore((s) => s.selectedWorkspace);
  const setSelectedWorkspace = useDaemonStore((s) => s.setSelectedWorkspace);
  const foldedProjects = useDaemonStore((s) => s.foldedProjects);
  const toggleProjectFold = useDaemonStore((s) => s.toggleProjectFold);

  // Identity-stable sessions (applyViewUpdate preserves refs when content
  // is unchanged) → useMemo can skip these passes on unrelated store updates
  // (status / occupant / sessionConfig). Without memoization every SessionRow
  // re-renders on a 1Hz tick from elsewhere in the tree (code-review #6).
  const workspaces = useMemo(() => selectDistinctWorkspaces(sessions), [sessions]);
  const groups = useMemo(
    () => groupSessionsByProject(sessions, selectedWorkspace),
    [sessions, selectedWorkspace],
  );

  const empty = groups.length === 0;

  return (
    <div className="session-list" data-workspace={selectedWorkspace}>
      <WorkspaceSwitcher
        workspaces={workspaces}
        selected={selectedWorkspace}
        onChange={setSelectedWorkspace}
      />
      {empty ? (
        <output className="session-list__empty">
          {selectedWorkspace === DEFAULT_WORKSPACE
            ? "No sessions yet."
            : `No sessions in workspace "${selectedWorkspace}".`}
        </output>
      ) : (
        <div className="session-list__projects">
          {groups.map((g) => (
            <ProjectGroup
              // projectPath is the unique identity (basenames can collide
              // across distinct paths). React key + foldedProjects key both
              // ride projectPath so two repos named 'web' don't merge nor
              // share a fold state.
              key={g.projectPath}
              project={g.project}
              projectPath={g.projectPath}
              sessions={g.sessions}
              folded={foldedProjects.has(g.projectPath)}
              onToggleFold={toggleProjectFold}
              activeId={activeId}
              daemonDisconnected={daemonDisconnected}
              selectSession={selectSession}
            />
          ))}
        </div>
      )}
    </div>
  );
}
