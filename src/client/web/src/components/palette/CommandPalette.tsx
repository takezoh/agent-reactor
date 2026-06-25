// CommandPalette — overlay DOM owner for the command palette feature.
//
// Spec: docs/specs/2026-06-24-web-ui-command-palette
// FR:
//   - FR-003 focus trap + blur on open
//   - FR-007 role="dialog" / aria-modal
//   - FR-009 active context header (setActiveContextSnapshot wiring)
//   - FR-010 InlineStatus announce on active session change
//   - FR-012 frozenSnapshotRef capture on submitting false→true
//   - FR-013 frozenSnapshotRef release on submitting true→false
//   - FR-017 Esc steps back (paramSelect → toolSelect; toolSelect → close)
//   - FR-018 Back / Close / overlay outside-click dismissal
//   - FR-020 surfaces the submitting state (StatusBadge)
//   - FR-023 close + restore opener focus when push becomes invalid
//   - FR-024 StatusBadge priority text
//   - FR-025 StatusBadge: ctx===null → 'Unavailable', loading → 'Loading commands...'
//   - FR-029 input.focus() driven by refocusSeq observation
//   - FR-033 InlineStatus announce suppressed while submitting
// ADRs:
//   - 0030 terminal-keyed-remount
//   - 0036 palette-2phase-store-architecture
//   - 0039 palette-focus-trap-minimal
//   - 0050 unified listbox
//   - 0055 submit-freeze-lift-state
//   - 0057 InlineStatus single aria-live slot

import type { KeyboardEvent, MouseEvent } from "react";
import { useEffect, useRef } from "react";
import { createPortal } from "react-dom";
import { useFocusTrap } from "../../hooks/useFocusTrap";
import type { ToolCtx } from "../../lib/tools";
import { listTools } from "../../lib/tools";
import { useDaemonStore } from "../../store/daemon";
import { usePaletteStore } from "../../store/palette";
import { useDaemonSnapshot } from "../../store/useDaemonSnapshot";
import { ActiveContextHeader } from "./ActiveContextHeader";
import { InlineStatus } from "./InlineStatus";
import { ParamSelectPhase } from "./ParamSelectPhase";
import { StatusBadge } from "./StatusBadge";
import { ToolSelectPhase } from "./ToolSelectPhase";
import { useActiveContextBridge } from "./hooks/useActiveContextBridge";
import { useAnnounceMessage } from "./hooks/useAnnounceMessage";
import { useFrozenSnapshot } from "./hooks/useFrozenSnapshot";
import { useToolCtx } from "./hooks/useToolCtx";

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
  const announceSeq = usePaletteStore((s) => s.announceSeq);
  const activeContextSnapshot = usePaletteStore((s) => s.activeContextSnapshot);
  const flashSeq = usePaletteStore((s) => s.flashSeq);
  const setActiveContextSnapshot = usePaletteStore((s) => s.setActiveContextSnapshot);

  const dialogRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  // openerRef tracks the latest non-null opener so close/cleanup can
  // always find the focus target even after store resets opener to null.
  const openerRef = useRef<HTMLElement | null>(null);
  if (opener !== null) openerRef.current = opener;

  useFocusTrap(dialogRef, open);

  // FR-003: blur on open; restore opener focus on close.
  useEffect(() => {
    if (!open) return;
    const active = document.activeElement as HTMLElement | null;
    if (active !== null && typeof active.blur === "function") {
      active.blur();
    } else {
      console.warn("[palette] open: activeElement not blurrable; FR-003 defocus is best-effort", {
        activeTag: active === null ? null : active.tagName,
      });
    }
    return () => {
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
      openerRef.current = null;
    };
  }, [open]);

  // FR-029: refocusSeq increments → focus the search input.
  useEffect(() => {
    void refocusSeq;
    if (!open) return;
    const el = inputRef.current;
    if (el !== null) el.focus();
  }, [refocusSeq, open]);

  // Daemon snapshot (primitive selectors to minimize re-renders).
  const activeSessionID = useDaemonStore((s) => s.activeSessionID);
  const sessions = useDaemonStore((s) => s.sessions);
  const sessionConfig = useDaemonStore((s) => s.sessionConfig);
  const daemon = useDaemonSnapshot();

  // Warn once per palette-open when sessionConfig is missing.
  const sessionConfigMissing = sessionConfig === null;
  useEffect(() => {
    if (!open) return;
    if (sessionConfigMissing) {
      console.warn(
        "[palette] sessionConfig not yet fetched; ParamSelectPhase will see empty projects/pushCommands until REST hydrate lands",
      );
    }
  }, [open, sessionConfigMissing]);

  // FR-009: derive + push active context snapshot (suppressed while submitting).
  useActiveContextBridge(
    submitting,
    activeSessionID,
    sessions,
    sessionConfig,
    setActiveContextSnapshot,
  );

  // FR-012 / FR-013 / ADR-0055: capture/release frozenSnapshot.
  const { frozenSnapshotRef } = useFrozenSnapshot(
    submitting,
    daemon,
    activeContextSnapshot,
    flashSeq,
  );

  // FR-010 / FR-033 / ADR-0057: announce active context change to InlineStatus.
  const announceRef = useAnnounceMessage(announceSeq, submitting, activeContextSnapshot);

  // ctx construction (includes frozenActiveContext for UAC-018).
  const frozenActiveContext = frozenSnapshotRef.current?.activeContext ?? undefined;
  const ctx = useToolCtx(daemon, props.httpFactory, frozenActiveContext);

  // FR-024 / FR-025: StatusBadge text computation.
  const liveStatusBadgeText: string | null = (() => {
    if (phase === "toolSelect") {
      const all = listTools(daemon, daemon.pushCommands);
      const enabledCount = all.filter((t) => t.disabledReason(daemon) === null).length;
      if (enabledCount === 0) {
        if (sessionConfig === null) return "Loading commands...";
        return "No commands available";
      }
    }
    return null;
  })();

  if (!open) return null;

  function onOverlayMouseDown(e: MouseEvent<HTMLDivElement>): void {
    if (e.target === e.currentTarget) {
      usePaletteStore.getState().close();
    }
  }

  function onDialogKeyDown(e: KeyboardEvent<HTMLDivElement>): void {
    if (e.key === "Escape") {
      const state = usePaletteStore.getState();
      if (state.composing) return;
      e.preventDefault();
      state.back();
    }
  }

  // Frozen list props for ToolSelectPhase.
  const frozen = frozenSnapshotRef.current;
  const frozenListProps = frozen
    ? { frozenList: frozen.sortedList, frozenCursor: frozen.sortedListCursor }
    : {};

  // Frozen header props (ADR-0055: lock flashSeq at capture time).
  const frozenHeaderSnapshot = frozen?.activeContext;
  const headerFlashSeq = frozen !== null ? frozen.flashSeq : flashSeq;

  const statusBadgeText = submitting ? "Sending..." : liveStatusBadgeText;

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
            aria-label="Back"
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
            aria-label="Close"
            className="palette-header__close"
            onClick={() => usePaletteStore.getState().close()}
            data-testid="palette-close"
          >
            ×
          </button>
        </header>
        {ctx !== null && (
          <ActiveContextHeader snapshot={frozenHeaderSnapshot} flashSeq={headerFlashSeq} />
        )}
        <InlineStatus announce={announceRef.current} />
        {ctx === null ? (
          <StatusBadge text="Unavailable" />
        ) : (
          <StatusBadge text={statusBadgeText} submitting={submitting} />
        )}
        {phase === "toolSelect" ? (
          <ToolSelectPhase
            inputRef={inputRef}
            httpFactory={props.httpFactory}
            {...frozenListProps}
          />
        ) : ctx !== null ? (
          <ParamSelectPhase ctx={ctx} />
        ) : (
          <div role="alert" className="palette-error" data-testid="palette-ctx-error">
            Command palette unavailable (http client invalid)
          </div>
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
