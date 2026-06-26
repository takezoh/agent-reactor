import { type KeyboardEvent, type ReactNode, useRef, useState } from "react";
import type { LogTab } from "../wire/server";
import { ContentArea, isSuppressed, kindOfTab } from "./LogTabs";
import "../css/view.css";

/**
 * MainTabs renders an exclusive tab strip combining a synthetic TERMINAL tab
 * with driver-provided log tabs (TRANSCRIPT / EVENTS). Only one body is
 * visually active at a time:
 *
 *   - TERMINAL active   → terminal slot is visible, log tab content is hidden
 *   - log tab active    → log tab content is visible, terminal slot is invisible
 *
 * ADR-0065: the terminal slot is an absolute overlay over .main-tabs-body, NOT
 * a flex sibling of the log panels. This decouples xterm's host size from the
 * log-panel display state so the log panels and the terminal never compete for
 * flex remainder. Without this separation, the inactive terminal-slot used to
 * grab half of the flex remainder via `flex: 1 1 0` and push log content into
 * the bottom half of the viewport ("middle-aligned content" regression).
 *
 * The terminal slot is *always mounted* and toggled via data-active +
 * visibility/pointer-events/aria-hidden so xterm.js scrollback and the
 * subscribe / unsubscribe lifecycle (ADR 0030) survive tab switches. The
 * ResizeObserver inside TerminalPane (ADR 0034) only observes the host's
 * own box, which now stays at the .main-tabs-body size regardless of which
 * tab is active — no fit() needs to be re-driven on tab switch.
 *
 * Keyboard navigation follows WAI-ARIA APG Tabs Pattern (manual activation):
 *   - ArrowRight / ArrowLeft / Home / End → move focus among tabs only
 *   - Space or Enter → activate the focused tab (aria-selected transition)
 *
 * ADR-0061: focus movement and activation are deliberately separated.
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

function indexToActive(index: number, _tabCount: number): Active {
  return index === 0 ? { kind: "terminal" } : { kind: "log", index: index - 1 };
}

export function MainTabs({
  tabs,
  sessionId = "",
  bearerToken = "",
  fetchFn,
  suppressInfo = false,
  terminalSlot,
}: MainTabsProps) {
  const [active, setActive] = useState<Active>({ kind: "terminal" });
  // focusedIndex tracks which tab has roving-tabindex focus (may differ from active)
  const [focusedIndex, setFocusedIndex] = useState<number>(0);
  const tabRefs = useRef<Array<HTMLButtonElement | null>>([]);

  const totalTabs = 1 + tabs.length; // TERMINAL + log tabs

  const isTerminalActive = active.kind === "terminal";

  function activate(index: number) {
    const next = indexToActive(index, tabs.length);
    setActive(next);
    setFocusedIndex(index);
  }

  function handleKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    let next = focusedIndex;

    switch (e.key) {
      case "ArrowRight":
        next = (focusedIndex + 1) % totalTabs;
        break;
      case "ArrowLeft":
        next = (focusedIndex - 1 + totalTabs) % totalTabs;
        break;
      case "Home":
        next = 0;
        break;
      case "End":
        next = totalTabs - 1;
        break;
      case " ":
      case "Enter":
        activate(focusedIndex);
        e.preventDefault();
        return;
      default:
        return;
    }

    e.preventDefault();
    setFocusedIndex(next);
    tabRefs.current[next]?.focus();
  }

  return (
    <div className="main-tabs">
      <div
        className="main-tab-list log-tab-row"
        role="tablist"
        aria-label="Session views"
        onKeyDown={handleKeyDown}
      >
        <button
          ref={(el) => {
            tabRefs.current[0] = el;
          }}
          id="main-tab-terminal"
          role="tab"
          type="button"
          aria-selected={isTerminalActive ? "true" : "false"}
          aria-controls="main-tabpanel-terminal"
          tabIndex={isTerminalActive ? 0 : -1}
          className={isTerminalActive ? "main-tab log-tab active" : "main-tab log-tab"}
          onClick={() => activate(0)}
          onFocus={() => setFocusedIndex(0)}
        >
          TERMINAL
        </button>
        {tabs.map((t, i) => {
          const tabIndex = i + 1;
          const selected = active.kind === "log" && active.index === i;
          const panelId = `main-tabpanel-log-${i}`;
          const tabId = `main-tab-log-${i}`;
          return (
            <button
              key={`${i}-${t.label}`}
              ref={(el) => {
                tabRefs.current[tabIndex] = el;
              }}
              id={tabId}
              role="tab"
              type="button"
              aria-selected={selected ? "true" : "false"}
              aria-controls={panelId}
              tabIndex={selected ? 0 : -1}
              className={selected ? "main-tab log-tab active" : "main-tab log-tab"}
              onClick={() => activate(tabIndex)}
              onFocus={() => setFocusedIndex(tabIndex)}
            >
              {t.label}
            </button>
          );
        })}
      </div>
      <div className="main-tabs-body">
        {/* Log panels are normal-flow flex children: only the active one is
            display:flex; others are display:none via the `hidden` attribute.
            Because only one is ever in flow at a time, they never compete for
            flex remainder with each other. */}
        {tabs.map((t, i) => {
          const selected = active.kind === "log" && active.index === i;
          const panelId = `main-tabpanel-log-${i}`;
          const tabId = `main-tab-log-${i}`;
          const tabKind = kindOfTab(t);
          const isSuppressedTab = isSuppressed(t, suppressInfo);
          return (
            <div
              key={`panel-${i}-${t.label}`}
              id={panelId}
              role="tabpanel"
              aria-labelledby={tabId}
              className={selected ? "tab-panel tab-panel--active" : "tab-panel"}
              hidden={!selected ? true : undefined}
            >
              {selected && !isSuppressedTab && tabKind !== null ? (
                <ContentArea
                  sessionId={sessionId}
                  kind={tabKind}
                  bearerToken={bearerToken}
                  fetchFn={fetchFn}
                />
              ) : (
                <div className="log-tab-content" />
              )}
            </div>
          );
        })}
        {/* ADR-0065: terminal-slot is an absolute overlay layer on
            .main-tabs-body. It is always mounted (subscribe ownership /
            ADR-0030) but takes no space in the flex column flow, so it
            never pushes log content into the bottom half. data-active
            drives visibility + pointer-events via view.css, and aria-hidden
            mirrors it for AT. */}
        <div
          id="main-tabpanel-terminal"
          role="tabpanel"
          aria-labelledby="main-tab-terminal"
          className="terminal-slot"
          data-active={isTerminalActive ? "true" : "false"}
          aria-hidden={!isTerminalActive}
        >
          {terminalSlot}
        </div>
      </div>
    </div>
  );
}
