// CommandPalette — overlay DOM owner for the command palette feature.
//
// Spec: docs/specs/2026-06-24-web-ui-command-palette
// FR:
//   - FR-003 focus trap + blur on open
//   - FR-007 role="dialog" / aria-modal
//   - FR-017 Esc steps back (paramSelect → toolSelect; toolSelect → close)
//   - FR-018 戻る / 閉じる / overlay 外側クリックで離脱
//   - FR-020 submitting 表示
//   - FR-023 push 失効時に close + opener focus 復帰
//   - FR-029 refocusSeq 観測で input.focus()
// ADRs:
//   - 0030 terminal-keyed-remount: subscribe/unsubscribe を発行せず、
//     blur のみで TerminalPane の入力を抑制する。
//   - 0036 palette-2phase-store-architecture: DOM 副作用 (blur / focus /
//     portal mount) は本 component に局所化。store は純粋 state。
//   - 0039 palette-focus-trap-minimal: 本 component は useFocusTrap を
//     enabled=open で切り替えるだけ。trap 詳細は hook 側に委譲。
//
// Tool ctx assembly:
//   ParamSelectPhase needs ToolCtx (http / daemon / notify / store). We
//   build it here once per render — ToolSelectPhase builds its own ctx
//   (paramless tools fast-path goes through confirmTool → submit there).
//   Keeping ctx construction symmetrical between phases avoids divergence
//   in how http / notify / store are wired.

import type { KeyboardEvent, MouseEvent } from "react";
import { useEffect, useMemo, useRef } from "react";
import { createPortal } from "react-dom";
import { makeSessionsApi } from "../../api/sessions";
import { useFocusTrap } from "../../hooks/useFocusTrap";
import type { DaemonSnapshot, ToolCtx } from "../../lib/tools";
import { selectDaemonSnapshot, useDaemonStore } from "../../store/daemon";
import { useNotificationsStore } from "../../store/notifications";
import { usePaletteStore } from "../../store/palette";
import { ParamSelectPhase } from "./ParamSelectPhase";
import { ScopeSegment } from "./ScopeSegment";
import { ToolSelectPhase } from "./ToolSelectPhase";

// sessionConfig (ADR-0041) is null until makeSessionsApi().getSessionConfig()
// resolves; we keep the breadcrumb-on-open observability below so a
// regression that drops the slice mid-session-life is still surfaced rather
// than rendering as "you have zero projects". The actual []-fallback lives
// inside selectDaemonSnapshot (Y3 single source) so this component does not
// duplicate the projection.

// SessionsApi method names we require at ToolCtx construction time. If the
// httpFactory hands us back something missing any of these, ParamSelectPhase
// would crash deep inside submit() with a confusing `undefined is not a
// function`. We check up front so the root cause (broken factory) is on the
// same stack as the failure.
const REQUIRED_HTTP_METHODS = [
  "createSession",
  "deleteSession",
  "pushCommand",
  "getSessionConfig",
] as const;

function isValidSessionsApi(http: unknown): http is ToolCtx["http"] {
  if (http === null || typeof http !== "object") return false;
  const obj = http as Record<string, unknown>;
  return REQUIRED_HTTP_METHODS.every((name) => typeof obj[name] === "function");
}

export interface CommandPaletteProps {
  // httpFactory swaps the SessionsApi for hermetic tests. Production callers
  // omit it and get makeSessionsApi() — same shape ToolSelectPhase honors.
  httpFactory?: () => ToolCtx["http"];
}

export function CommandPalette(props: CommandPaletteProps = {}): JSX.Element | null {
  const open = usePaletteStore((s) => s.open);
  const phase = usePaletteStore((s) => s.phase);
  const opener = usePaletteStore((s) => s.opener);
  const submitting = usePaletteStore((s) => s.submitting);
  const error = usePaletteStore((s) => s.error);
  const refocusSeq = usePaletteStore((s) => s.refocusSeq);

  const dialogRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  // openerRef tracks the latest non-null opener. We update it on every
  // render so that if a future caller swaps opener mid-open we restore to
  // the new one — but we never overwrite to null. close() / back() reset
  // opener to null as part of the closed-state shape, so without this
  // "never go null" rule the cleanup would lose the focus target before
  // it gets a chance to restore it.
  const openerRef = useRef<HTMLElement | null>(null);
  if (opener !== null) openerRef.current = opener;

  // useFocusTrap is a true no-op when enabled=false (see ADR-0039), so
  // unconditionally calling it with `open` is correct even though the
  // component early-returns null below — the hook is called from the same
  // place in the render every time (React rules of hooks).
  useFocusTrap(dialogRef, open);

  // FR-003: on open, blur whatever currently owns DOM focus so the
  // underlying TerminalPane (xterm textarea) stops swallowing keys. We do
  // NOT subscribe/unsubscribe TerminalPane (ADR-0030 keeps that contract
  // single-owner); blur is a pseudo-defocus that the terminal interprets
  // naturally. On unmount (close / unmount-while-open) we restore focus
  // to the opener that openPalette() captured.
  //
  // We depend only on `open` (not on `opener`) so a future mid-open opener
  // change does NOT re-fire the effect body — re-running blur would steal
  // focus from the palette input the user is currently typing into. The
  // openerRef captures the latest non-null opener for the cleanup path.
  useEffect(() => {
    if (!open) return;
    const active = document.activeElement as HTMLElement | null;
    if (active !== null && typeof active.blur === "function") {
      active.blur();
    } else {
      // Observable fallback: activeElement was <body> or a non-blurrable
      // node, so FR-003 ("TerminalPane stops swallowing keys") may not
      // actually hold. We log a breadcrumb so the "palette is open but
      // xterm still receives keystrokes" failure mode is debuggable
      // rather than invisible.
      console.warn("[palette] open: activeElement not blurrable; FR-003 defocus is best-effort", {
        activeTag: active === null ? null : active.tagName,
      });
    }
    return () => {
      // Read opener from the ref — close() may have nulled the store
      // opener before this cleanup runs, but openerRef remembers the
      // last non-null value. Guard against detached opener nodes:
      // focus() on a detached element silently moves focus to <body>,
      // which would break FR-017 / FR-023 ("close → focus returns to
      // opener") without any error.
      const o = openerRef.current;
      if (o === null) return;
      if (typeof o.focus !== "function") return;
      if (!document.contains(o)) {
        console.warn(
          "[palette] close: opener detached from DOM; focus restore skipped (FR-017/FR-023 unobservable)",
          { openerTag: o.tagName },
        );
        return;
      }
      o.focus();
      // Clear the ref so the next open() cycle starts from a clean slate
      // and a no-opener open won't accidentally focus the stale opener
      // from the previous cycle.
      openerRef.current = null;
    };
  }, [open]);

  // FR-029: refocusSeq increments are the store's "please focus the
  // search input" signal. We only honor it while open — a stray
  // refocusInput() call while closed should not steal focus. Reading
  // refocusSeq inside the effect body (even just `void`) keeps biome's
  // useExhaustiveDependencies happy without dropping the trigger dep.
  useEffect(() => {
    void refocusSeq;
    if (!open) return;
    const el = inputRef.current;
    if (el !== null) el.focus();
  }, [refocusSeq, open]);

  // Build the ToolCtx ParamSelectPhase needs. We mirror ToolSelectPhase's
  // composition: primitive selectors over the daemon store, then a useMemo
  // that returns a stable snapshot object. httpFactory is captured so
  // tests can inject a fake without prop-drilling.
  //
  // NOTE: we use one primitive selector per field instead of a single
  // `(s) => s` selector — the latter re-renders this component on EVERY
  // daemon snapshot tick (sessions array identity, view status, etc.),
  // which would also burn through ctx useMemo identity (props.httpFactory
  // wouldn't be enough to stop it because daemon would change every tick).
  // Pulling fields independently keeps re-renders proportional to the
  // fields we actually read.
  const sessions = useDaemonStore((s) => s.sessions);
  const activeSessionID = useDaemonStore((s) => s.activeSessionID);
  const activeOccupant = useDaemonStore((s) => s.activeOccupant);
  // sessionConfig is the REST-fetched slice (ADR-0041). We subscribe to the
  // whole slice reference (a single zustand selector) so the snapshot memo
  // below re-fires on slice swap but not on every daemon-internal tick.
  // The actual projects/pushCommands extraction + []-fallback lives inside
  // selectDaemonSnapshot (Y3 single source) — keep it out of this file so
  // a future shape diff lands in one place.
  const sessionConfig = useDaemonStore((s) => s.sessionConfig);
  // Observability: log once per palette-open when sessionConfig has not yet
  // been hydrated. Without this breadcrumb a "ParamSelectPhase renders empty
  // projects" failure would look indistinguishable from a real "user has
  // zero projects configured" result. Tied to `open` so closed-palette
  // renders stay silent and the warning only fires when a user actually
  // tries to use the palette before the REST fetch resolved.
  const sessionConfigMissing = sessionConfig === null;
  useEffect(() => {
    if (!open) return;
    if (sessionConfigMissing) {
      console.warn(
        "[palette] sessionConfig not yet fetched; ParamSelectPhase will see empty projects/pushCommands until REST hydrate lands",
      );
    }
  }, [open, sessionConfigMissing]);

  const daemon = useMemo<DaemonSnapshot>(
    () => selectDaemonSnapshot({ sessions, activeSessionID, activeOccupant, sessionConfig }),
    [sessions, activeSessionID, activeOccupant, sessionConfig],
  );
  const ctx = useMemo<ToolCtx | null>(() => {
    const http = props.httpFactory ? props.httpFactory() : makeSessionsApi();
    // Validate the http shape up front. A factory that returns undefined or
    // a partial object would otherwise crash deep inside ParamSelectPhase's
    // submit handler with `Cannot read properties of undefined (reading
    // 'createSession')`, far from the actual root cause. Refusing to build
    // ctx surfaces the failure at construction time and keeps the palette
    // shell rendering — submit() falls through to a no-op, and the
    // breadcrumb tells operators which factory misbehaved.
    if (!isValidSessionsApi(http)) {
      console.error("[palette] httpFactory returned an invalid SessionsApi; ctx not built", {
        keys: http === null || typeof http !== "object" ? typeof http : Object.keys(http),
      });
      return null;
    }
    const paletteState = usePaletteStore.getState();
    return {
      http,
      daemon,
      // FR-021: daemonActions exposes the write-side daemon API ToolDef.submit
      // uses (selectSession after createSession). Bound via useDaemonStore.
      // getState() so the action call always lands on the current store
      // instance (zustand actions are stable singletons but the getter form
      // is consistent with the rest of ctx construction).
      daemonActions: {
        selectSession(id) {
          useDaemonStore.getState().selectSession(id);
        },
      },
      notify: {
        success(m) {
          useNotificationsStore.getState().add({ level: "info", message: m });
        },
        error(m) {
          useNotificationsStore.getState().add({ level: "error", message: m });
        },
      },
      store: {
        close: paletteState.close,
        clearActiveIf: paletteState.clearActiveIf,
      },
    };
  }, [daemon, props.httpFactory]);

  if (!open) return null;

  function onOverlayMouseDown(e: MouseEvent<HTMLDivElement>): void {
    // FR-018: outside-click closes. We treat "outside" as a mousedown whose
    // target IS the overlay itself — clicks bubbling up from the dialog
    // body get target === dialog descendant, which fails the check.
    if (e.target === e.currentTarget) {
      usePaletteStore.getState().close();
    }
  }

  function onDialogKeyDown(e: KeyboardEvent<HTMLDivElement>): void {
    // FR-017: Esc = back (paramSelect → toolSelect; toolSelect → close).
    // The store's back() implements both transitions.
    //
    // IME passthrough: while composing=true the user almost certainly means
    // "cancel current conversion candidate", not "step palette back". The
    // browser already routes Esc to the IME first when keyCode === 229; we
    // additionally check the store's composing flag so we don't swallow the
    // post-IME Esc that some platforms surface as key="Escape" with
    // composing=true still set. Leaving Esc unhandled lets the IME consume
    // it naturally.
    if (e.key === "Escape") {
      const state = usePaletteStore.getState();
      if (state.composing) return;
      e.preventDefault();
      state.back();
    }
  }

  return createPortal(
    <div className="palette-overlay" data-testid="palette-overlay" onMouseDown={onOverlayMouseDown}>
      <div
        ref={dialogRef}
        // biome-ignore lint/a11y/useSemanticElements: native <dialog> requires
        // showModal()/HTMLDialogElement APIs that don't compose with our
        // store-driven open state; the WAI-ARIA dialog role on a generic
        // container is the documented alternative.
        role="dialog"
        aria-modal="true"
        aria-labelledby="palette-title"
        className="palette-dialog"
        onKeyDown={onDialogKeyDown}
      >
        <header className="palette-header">
          <button
            type="button"
            aria-label="戻る"
            className="palette-header__back"
            onClick={() => usePaletteStore.getState().back()}
            data-testid="palette-back"
          >
            ←
          </button>
          <h2 id="palette-title" className="palette-header__title">
            Command Palette
          </h2>
          <button
            type="button"
            aria-label="閉じる"
            className="palette-header__close"
            onClick={() => usePaletteStore.getState().close()}
            data-testid="palette-close"
          >
            ×
          </button>
        </header>
        <ScopeSegment />
        {phase === "toolSelect" ? (
          <ToolSelectPhase inputRef={inputRef} httpFactory={props.httpFactory} />
        ) : ctx !== null ? (
          <ParamSelectPhase ctx={ctx} />
        ) : (
          // ctx === null means httpFactory returned an invalid SessionsApi.
          // We already logged a console.error; surface the same fact to the
          // user so they know the param phase is non-functional rather than
          // staring at an empty area.
          <div role="alert" className="palette-error" data-testid="palette-ctx-error">
            command palette は利用できません (http クライアントが不正)
          </div>
        )}
        {submitting && (
          <output className="palette-progress" data-testid="palette-progress">
            sending…
          </output>
        )}
        {error !== null && (
          <div role="alert" className="palette-error" data-testid="palette-error">
            {error}
          </div>
        )}
      </div>
    </div>,
    document.body,
  );
}
