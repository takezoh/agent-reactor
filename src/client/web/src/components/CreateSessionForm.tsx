import { useEffect, useId, useRef, useState } from "react";
import type { Connection } from "../socket/connection";
import { useDaemonStore } from "../store/daemon";

type SessionConfig = {
  default_command: string;
  commands: string[];
  projects: string[];
};

const PROJECT_PLACEHOLDER = "Project directory (absolute path, e.g. /home/me/myrepo)";
const COMMAND_PLACEHOLDER = "Command (filter or type a custom one)";

// CreateSessionForm is the entry-point — the sidebar gets a single "New
// Session" trigger; the actual project/command form lives inside a popup
// dialog rendered only while open. Mounting the dialog conditionally keeps
// the /api/session-config fetch lazy (only fires when the user opens the
// popup) and resets local form state between opens.
export function CreateSessionForm({
  conn,
  bearerToken,
}: {
  conn: Connection;
  bearerToken: string;
}) {
  const [open, setOpen] = useState(false);
  return (
    <div className="create-session-trigger">
      <button type="button" onClick={() => setOpen(true)}>
        New Session
      </button>
      {open && (
        <CreateSessionDialog conn={conn} bearerToken={bearerToken} onClose={() => setOpen(false)} />
      )}
    </div>
  );
}

function CreateSessionDialog({
  conn: _conn,
  bearerToken,
  onClose,
}: {
  conn: Connection;
  bearerToken: string;
  onClose: () => void;
}) {
  const [project, setProject] = useState("");
  const [command, setCommand] = useState("");
  // worktree and host mirror the TUI palette's Tab / Shift-Tab toggles:
  // worktree=true asks the daemon to create a git worktree (LaunchOptions.
  // Worktree.Enabled), host=true forces sandbox="host" (SandboxOverrideHost,
  // bypassing the per-project sandbox config). Both default off, and we only
  // include them in the POST body when truthy so the legacy minimal-body
  // wire shape is preserved for the default path.
  const [worktree, setWorktree] = useState(false);
  const [host, setHost] = useState(false);
  const [cfg, setCfg] = useState<SessionConfig | null>(null);
  const [cfgErr, setCfgErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const selectSession = useDaemonStore((s) => s.selectSession);

  const projectsListId = useId();
  const commandsListId = useId();
  const dialogRef = useRef<HTMLDialogElement | null>(null);
  const projectInputRef = useRef<HTMLInputElement | null>(null);

  // Drive the native <dialog> via showModal/close when available (gets the
  // browser-managed backdrop, focus trap, ESC handling for free) and fall
  // back to the `open` attribute so the dialog still renders under
  // happy-dom / older browsers that don't implement showModal.
  useEffect(() => {
    const d = dialogRef.current;
    if (!d) return undefined;
    if (typeof d.showModal === "function" && !d.open) {
      try {
        d.showModal();
      } catch {
        d.setAttribute("open", "");
      }
    } else {
      d.setAttribute("open", "");
    }
    projectInputRef.current?.focus();
    const onCancel = (e: Event) => {
      e.preventDefault();
      onClose();
    };
    d.addEventListener("cancel", onCancel);
    return () => {
      d.removeEventListener("cancel", onCancel);
    };
  }, [onClose]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const resp = await fetch("/api/session-config", {
          headers: { Authorization: `Bearer ${bearerToken}` },
        });
        if (!resp.ok) {
          const detail = await resp.text().catch(() => "");
          throw new Error(detail.trim() || `/api/session-config ${resp.status}`);
        }
        const c = (await resp.json()) as SessionConfig;
        if (cancelled) return;
        setCfg(c);
        // Seed command from default_command, then the first commands entry.
        // We do NOT invent a fallback — that would resurrect the hardcoding
        // the user explicitly wanted gone. An empty commands list just
        // means the input starts blank and the user types their own.
        setCommand((curr) => curr || c.default_command || c.commands[0] || "");
      } catch (e) {
        if (cancelled) return;
        setCfgErr(e instanceof Error ? e.message : String(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [bearerToken]);

  // Mirror the gateway's isAbsoluteProjectPath rule (server/web/mux.go) so
  // we can fail fast in the browser instead of waiting on a 400. Server and
  // client deliberately agree on "starts with '/'"; if the server rule
  // tightens, the network round-trip will surface the real error.
  const projectTrimmed = project.trim();
  const commandTrimmed = command.trim();
  const projectIsAbsolute = projectTrimmed.startsWith("/");
  const canSubmit = projectIsAbsolute && commandTrimmed !== "" && !busy;

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!projectIsAbsolute) {
      setErr("Project directory must be an absolute path (start with '/').");
      return;
    }
    if (commandTrimmed === "") {
      setErr("Command is required.");
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      const reqBody: {
        project: string;
        command: string;
        worktree?: boolean;
        sandbox?: "host";
      } = { project: projectTrimmed, command: commandTrimmed };
      if (worktree) reqBody.worktree = true;
      if (host) reqBody.sandbox = "host";
      const resp = await fetch("/api/sessions", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${bearerToken}`,
        },
        body: JSON.stringify(reqBody),
      });
      if (!resp.ok) {
        // Surface the gateway's body verbatim — it carries the reason and
        // "(request_id=…)" so a screenshot is enough to grep server.log.
        const detail = await resp.text().catch(() => "");
        throw new Error(detail.trim() || `POST /api/sessions failed: ${resp.status}`);
      }
      const body = (await resp.json()) as { id: string };
      // ADR 0030: TerminalPane (keyed remount) is the SOLE owner of
      // subscribe/unsubscribe for the active session. Selecting the new
      // session id flips `activeSessionID` → App re-renders → the keyed
      // TerminalPane mount subscribes. Calling conn.subscribe here would
      // double-subscribe on every create-session path.
      selectSession(body.id);
      onClose();
    } catch (e2) {
      setErr(e2 instanceof Error ? e2.message : String(e2));
    } finally {
      setBusy(false);
    }
  }

  const projects = cfg?.projects ?? [];
  const commands = cfg?.commands ?? [];

  return (
    <dialog ref={dialogRef} className="create-session-dialog">
      <form className="create-session" onSubmit={onSubmit}>
        <h3>New session</h3>
        <label>
          Project directory
          <input
            ref={projectInputRef}
            type="text"
            value={project}
            onChange={(e) => setProject(e.target.value)}
            placeholder={PROJECT_PLACEHOLDER}
            aria-label="Project directory"
            list={projectsListId}
            disabled={busy}
            autoComplete="off"
          />
          <datalist id={projectsListId}>
            {projects.map((p) => (
              <option key={p} value={p} />
            ))}
          </datalist>
        </label>
        <label>
          Command
          <input
            type="text"
            value={command}
            onChange={(e) => setCommand(e.target.value)}
            placeholder={COMMAND_PLACEHOLDER}
            aria-label="Command"
            list={commandsListId}
            disabled={busy || cfg === null}
            autoComplete="off"
          />
          <datalist id={commandsListId}>
            {commands.map((c) => (
              <option key={c} value={c} />
            ))}
          </datalist>
        </label>
        <div className="create-session-toggles">
          <label className="create-session-toggle">
            <input
              type="checkbox"
              checked={worktree}
              onChange={(e) => setWorktree(e.target.checked)}
              disabled={busy}
              aria-label="Create git worktree"
            />
            <span>Worktree (git)</span>
          </label>
          <label className="create-session-toggle">
            <input
              type="checkbox"
              checked={host}
              onChange={(e) => setHost(e.target.checked)}
              disabled={busy}
              aria-label="Run on host (skip sandbox)"
            />
            <span>Run on host (skip sandbox)</span>
          </label>
        </div>
        {cfgErr && <span className="error">session-config: {cfgErr}</span>}
        {err && <span className="error">{err}</span>}
        <div className="create-session-actions">
          <button type="button" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button type="submit" disabled={!canSubmit}>
            Create
          </button>
        </div>
      </form>
    </dialog>
  );
}
