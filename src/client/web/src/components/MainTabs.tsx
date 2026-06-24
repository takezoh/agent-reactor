import { type ReactNode, useState } from "react";
import type { LogTab } from "../wire/server";
import { ContentArea, isSuppressed, kindOfTab } from "./LogTabs";
import "../css/view.css";

/**
 * MainTabs renders an exclusive tab strip combining a synthetic TERMINAL tab
 * with driver-provided log tabs (TRANSCRIPT / EVENTS). Only one body is
 * visually active at a time:
 *
 *   - TERMINAL active   → terminal slot is visible, log tab content is hidden
 *   - log tab active    → log tab content is visible, terminal slot is display:none
 *
 * The terminal slot is *always mounted* and toggled via CSS visibility so
 * xterm.js scrollback and the subscribe / unsubscribe lifecycle (ADR 0030)
 * survive tab switches. The ResizeObserver inside TerminalPane (ADR 0034)
 * picks up the display:none → visible transition and re-runs fit().
 */
export type MainTabsProps = {
  tabs: LogTab[];
  sessionId?: string;
  bearerToken?: string;
  fetchFn?: typeof fetch;
  suppressInfo?: boolean;
  /** Always-mounted terminal panel. Visibility is toggled by MainTabs. */
  terminalSlot: ReactNode;
};

type Active = { kind: "terminal" } | { kind: "log"; index: number };

export function MainTabs({
  tabs,
  sessionId = "",
  bearerToken = "",
  fetchFn,
  suppressInfo = false,
  terminalSlot,
}: MainTabsProps) {
  const [active, setActive] = useState<Active>({ kind: "terminal" });

  const isTerminalActive = active.kind === "terminal";
  const activeLogTab = active.kind === "log" ? tabs[active.index] : null;
  const activeKind = activeLogTab ? kindOfTab(activeLogTab) : null;
  const suppressed = activeLogTab ? isSuppressed(activeLogTab, suppressInfo) : false;

  return (
    <div className="main-tabs">
      <div className="log-tab-row" role="tablist">
        <button
          role="tab"
          type="button"
          aria-selected={isTerminalActive}
          className={isTerminalActive ? "log-tab active" : "log-tab"}
          onClick={() => setActive({ kind: "terminal" })}
        >
          TERMINAL
        </button>
        {tabs.map((t, i) => {
          const selected = active.kind === "log" && active.index === i;
          return (
            <button
              key={`${i}-${t.label}`}
              role="tab"
              type="button"
              aria-selected={selected}
              className={selected ? "log-tab active" : "log-tab"}
              onClick={() => setActive({ kind: "log", index: i })}
            >
              {t.label}
            </button>
          );
        })}
      </div>
      <div className="main-tabs-body">
        <div
          className={isTerminalActive ? "terminal-slot" : "terminal-slot hidden"}
          aria-hidden={!isTerminalActive}
        >
          {terminalSlot}
        </div>
        {!isTerminalActive && activeLogTab && !suppressed && activeKind !== null ? (
          <ContentArea
            sessionId={sessionId}
            kind={activeKind}
            bearerToken={bearerToken}
            fetchFn={fetchFn}
          />
        ) : !isTerminalActive ? (
          <div className="log-tab-content" role="tabpanel" />
        ) : null}
      </div>
    </div>
  );
}
