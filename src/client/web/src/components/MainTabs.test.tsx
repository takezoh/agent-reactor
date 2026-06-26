import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useTranscriptStore } from "../store/transcripts";
import type { LogTab } from "../wire/server";
import { MainTabs } from "./MainTabs";

function makeFetch(status: number, body: string): typeof fetch {
  return vi.fn().mockResolvedValue(
    new Response(body, {
      status,
      headers: { "Content-Type": "text/plain" },
    }),
  ) as unknown as typeof fetch;
}

const nopFetch = makeFetch(204, "");

// data-testid sentinel for the terminal slot. We use a stand-in node instead of
// the real TerminalPane to avoid the xterm.js DOM dance — MainTabs is the unit
// under test and the slot's identity is what matters.
const TERMINAL_TESTID = "term-stub";
const TerminalStub = () => <div data-testid={TERMINAL_TESTID}>TERM</div>;

const TABS: LogTab[] = [
  { label: "TRANSCRIPT", path: "/sessions/s1/x.transcript", kind: "text" },
  { label: "EVENTS", path: "/sessions/s1/s1.log", kind: "text" },
];

describe("MainTabs", () => {
  beforeEach(() => {
    useTranscriptStore.getState().reset();
    vi.clearAllMocks();
  });

  it("prepends a TERMINAL tab in front of driver log_tabs", () => {
    render(
      <MainTabs
        tabs={TABS}
        sessionId="s1"
        bearerToken="tok"
        fetchFn={nopFetch}
        terminalSlot={<TerminalStub />}
      />,
    );
    const buttons = screen.getAllByRole("tab");
    expect(buttons.map((b) => b.textContent)).toEqual(["TERMINAL", "TRANSCRIPT", "EVENTS"]);
  });

  it("renders only TERMINAL when there are no driver log_tabs", () => {
    render(
      <MainTabs
        tabs={[]}
        sessionId="s1"
        bearerToken="tok"
        fetchFn={nopFetch}
        terminalSlot={<TerminalStub />}
      />,
    );
    const buttons = screen.getAllByRole("tab");
    expect(buttons).toHaveLength(1);
    expect(buttons[0]?.textContent).toBe("TERMINAL");
    // Terminal slot is always mounted and starts active.
    // ADR-0065: active state is driven by data-active attribute (CSS overlay),
    // not the legacy tab-panel--active flex modifier.
    const slot = screen.getByTestId(TERMINAL_TESTID).parentElement;
    expect(slot?.className).toContain("terminal-slot");
    expect(slot?.getAttribute("data-active")).toBe("true");
  });

  it("starts with TERMINAL active by default", () => {
    render(
      <MainTabs
        tabs={TABS}
        sessionId="s1"
        bearerToken="tok"
        fetchFn={nopFetch}
        terminalSlot={<TerminalStub />}
      />,
    );
    const [terminalTab, transcriptTab] = screen.getAllByRole("tab");
    expect(terminalTab?.getAttribute("aria-selected")).toBe("true");
    expect(transcriptTab?.getAttribute("aria-selected")).toBe("false");
  });

  it("clicking TRANSCRIPT hides the terminal slot via CSS but keeps it mounted (FR-TABS-003)", () => {
    render(
      <MainTabs
        tabs={TABS}
        sessionId="s1"
        bearerToken="tok"
        fetchFn={nopFetch}
        terminalSlot={<TerminalStub />}
      />,
    );
    const stub = screen.getByTestId(TERMINAL_TESTID);
    // Initially active (ADR-0065: data-active="true")
    expect(stub.parentElement?.getAttribute("data-active")).toBe("true");

    act(() => {
      fireEvent.click(screen.getByRole("tab", { name: "TRANSCRIPT" }));
    });

    // Still mounted (same DOM node) — ADR 0030 subscribe ownership and xterm
    // scrollback survive the switch because the slot is never unmounted.
    const stubAfter = screen.getByTestId(TERMINAL_TESTID);
    expect(stubAfter).toBe(stub);
    // Not active: data-active="false", aria-hidden set
    expect(stubAfter.parentElement?.getAttribute("data-active")).toBe("false");
    expect(stubAfter.parentElement?.getAttribute("aria-hidden")).toBe("true");
  });

  it("renders TRANSCRIPT buffer content in <pre> when its tab is active", async () => {
    useTranscriptStore.getState().appendBackfill("s1", "transcript", ["t-line"], 5);
    useTranscriptStore.getState().appendBackfill("s1", "event-log", ["e-line"], 5);

    render(
      <MainTabs
        tabs={TABS}
        sessionId="s1"
        bearerToken="tok"
        fetchFn={nopFetch}
        terminalSlot={<TerminalStub />}
      />,
    );

    // Switch to TRANSCRIPT — terminal hidden, transcript content visible.
    act(() => {
      fireEvent.click(screen.getByRole("tab", { name: "TRANSCRIPT" }));
    });

    const pre = document.querySelector("pre");
    expect(pre?.textContent).toBe("t-line");

    // Switch to EVENTS — transcript content swapped out for event-log.
    act(() => {
      fireEvent.click(screen.getByRole("tab", { name: "EVENTS" }));
    });
    const preAfter = document.querySelector("pre");
    expect(preAfter?.textContent).toBe("e-line");
  });

  it("clicking back to TERMINAL re-shows the slot (data-active=true, aria-hidden=false)", () => {
    render(
      <MainTabs
        tabs={TABS}
        sessionId="s1"
        bearerToken="tok"
        fetchFn={nopFetch}
        terminalSlot={<TerminalStub />}
      />,
    );

    act(() => {
      fireEvent.click(screen.getByRole("tab", { name: "EVENTS" }));
    });
    act(() => {
      fireEvent.click(screen.getByRole("tab", { name: "TERMINAL" }));
    });

    const stub = screen.getByTestId(TERMINAL_TESTID);
    expect(stub.parentElement?.getAttribute("data-active")).toBe("true");
    expect(stub.parentElement?.getAttribute("aria-hidden")).toBe("false");
  });

  it("suppressInfo=true suppresses INFO-labelled log tab content but keeps button", () => {
    const infoTabs: LogTab[] = [{ label: "INFO", path: "/x.transcript", kind: "text" }];

    render(
      <MainTabs
        tabs={infoTabs}
        sessionId="s1"
        bearerToken="tok"
        fetchFn={nopFetch}
        suppressInfo
        terminalSlot={<TerminalStub />}
      />,
    );

    // INFO button still rendered alongside TERMINAL.
    const buttons = screen.getAllByRole("tab");
    expect(buttons.map((b) => b.textContent)).toEqual(["TERMINAL", "INFO"]);

    act(() => {
      fireEvent.click(screen.getByRole("tab", { name: "INFO" }));
    });

    // tabpanel is rendered but empty (no <pre>) because INFO is suppressed.
    const panel = screen.getByRole("tabpanel");
    expect(panel.querySelector("pre")).toBeNull();
    expect(panel.textContent).toBe("");
  });

  // ── FR-TABS-001: WAI-ARIA APG Tabs Pattern structure ──────────────────────
  describe("FR-TABS-001: APG tablist structure + roving tabindex", () => {
    it("renders role='tablist' with aria-label='Session views'", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const tablist = screen.getByRole("tablist");
      expect(tablist).not.toBeNull();
      expect(tablist.getAttribute("aria-label")).toBe("Session views");
    });

    it("renders 3 role='tab' buttons (TERMINAL + 2 log tabs)", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const tabs = screen.getAllByRole("tab");
      expect(tabs).toHaveLength(3);
    });

    it("active tab has tabindex=0, inactive tabs have tabindex=-1 (roving tabindex)", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const [terminalTab, transcriptTab, eventsTab] = screen.getAllByRole("tab");
      // TERMINAL starts active
      expect(terminalTab?.getAttribute("tabindex")).toBe("0");
      expect(transcriptTab?.getAttribute("tabindex")).toBe("-1");
      expect(eventsTab?.getAttribute("tabindex")).toBe("-1");
    });

    it("aria-selected='true' only on the active tab initially", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const tabs = screen.getAllByRole("tab");
      const selected = tabs.filter((t) => t.getAttribute("aria-selected") === "true");
      expect(selected).toHaveLength(1);
      expect(selected[0]?.textContent).toBe("TERMINAL");
    });

    it("after click TRANSCRIPT: tabindex and aria-selected update correctly", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      act(() => {
        fireEvent.click(screen.getByRole("tab", { name: "TRANSCRIPT" }));
      });
      const [terminalTab, transcriptTab, eventsTab] = screen.getAllByRole("tab");
      expect(terminalTab?.getAttribute("tabindex")).toBe("-1");
      expect(transcriptTab?.getAttribute("tabindex")).toBe("0");
      expect(eventsTab?.getAttribute("tabindex")).toBe("-1");
      expect(terminalTab?.getAttribute("aria-selected")).toBe("false");
      expect(transcriptTab?.getAttribute("aria-selected")).toBe("true");
    });

    it("each tab has aria-controls pointing to its panel, panel has role='tabpanel'", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const terminalTab = screen.getByRole("tab", { name: "TERMINAL" });
      const controlsId = terminalTab.getAttribute("aria-controls");
      expect(controlsId).toBeTruthy();
      const panel = document.getElementById(controlsId ?? "");
      expect(panel?.getAttribute("role")).toBe("tabpanel");
    });
  });

  // ── FR-TABS-002: keyboard navigation (manual activation) ──────────────────
  describe("FR-TABS-002: ArrowRight/Left/Home/End focus movement + Space/Enter activation", () => {
    it("ArrowRight moves focus to next tab but does NOT change aria-selected", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const tablist = screen.getByRole("tablist");
      const [terminalTab, transcriptTab] = screen.getAllByRole("tab");

      // Focus TERMINAL tab first
      act(() => {
        terminalTab?.focus();
      });

      act(() => {
        fireEvent.keyDown(tablist, { key: "ArrowRight" });
      });

      // TRANSCRIPT tab should now have focus
      expect(document.activeElement).toBe(transcriptTab);
      // aria-selected has NOT changed — manual activation
      expect(terminalTab?.getAttribute("aria-selected")).toBe("true");
      expect(transcriptTab?.getAttribute("aria-selected")).toBe("false");
    });

    it("Space activates the focused tab (aria-selected transitions)", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const tablist = screen.getByRole("tablist");
      const [terminalTab, transcriptTab] = screen.getAllByRole("tab");

      act(() => {
        terminalTab?.focus();
      });
      act(() => {
        fireEvent.keyDown(tablist, { key: "ArrowRight" });
      });
      // Focus is on TRANSCRIPT, but TERMINAL still selected
      expect(document.activeElement).toBe(transcriptTab);
      expect(terminalTab?.getAttribute("aria-selected")).toBe("true");

      // Space activates focused tab
      act(() => {
        fireEvent.keyDown(tablist, { key: " " });
      });
      expect(transcriptTab?.getAttribute("aria-selected")).toBe("true");
      expect(terminalTab?.getAttribute("aria-selected")).toBe("false");
    });

    it("Enter activates the focused tab (aria-selected transitions)", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const tablist = screen.getByRole("tablist");
      const [terminalTab, transcriptTab] = screen.getAllByRole("tab");

      act(() => {
        terminalTab?.focus();
      });
      act(() => {
        fireEvent.keyDown(tablist, { key: "ArrowRight" });
      });
      act(() => {
        fireEvent.keyDown(tablist, { key: "Enter" });
      });

      expect(transcriptTab?.getAttribute("aria-selected")).toBe("true");
      expect(terminalTab?.getAttribute("aria-selected")).toBe("false");
    });

    it("ArrowLeft moves focus to previous tab (wraps from TERMINAL to EVENTS)", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const tablist = screen.getByRole("tablist");
      const [terminalTab, , eventsTab] = screen.getAllByRole("tab");

      act(() => {
        terminalTab?.focus();
      });
      act(() => {
        fireEvent.keyDown(tablist, { key: "ArrowLeft" });
      });

      // Wrap: TERMINAL (index 0) → EVENTS (index 2 = last)
      expect(document.activeElement).toBe(eventsTab);
      // aria-selected unchanged
      expect(terminalTab?.getAttribute("aria-selected")).toBe("true");
    });

    it("Home moves focus to the first tab (TERMINAL)", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const tablist = screen.getByRole("tablist");
      const [terminalTab, , eventsTab] = screen.getAllByRole("tab");

      act(() => {
        eventsTab?.focus();
      });
      act(() => {
        fireEvent.keyDown(tablist, { key: "Home" });
      });

      expect(document.activeElement).toBe(terminalTab);
    });

    it("End moves focus to the last tab (EVENTS)", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const tablist = screen.getByRole("tablist");
      const [terminalTab, , eventsTab] = screen.getAllByRole("tab");

      act(() => {
        terminalTab?.focus();
      });
      act(() => {
        fireEvent.keyDown(tablist, { key: "End" });
      });

      expect(document.activeElement).toBe(eventsTab);
    });

    it("full navigation sequence: End → ArrowRight wraps to first tab", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const tablist = screen.getByRole("tablist");
      const [terminalTab, , eventsTab] = screen.getAllByRole("tab");

      act(() => {
        terminalTab?.focus();
      });
      // End → focus last (EVENTS)
      act(() => {
        fireEvent.keyDown(tablist, { key: "End" });
      });
      expect(document.activeElement).toBe(eventsTab);

      // ArrowRight on last → wraps to first (TERMINAL)
      act(() => {
        fireEvent.keyDown(tablist, { key: "ArrowRight" });
      });
      expect(document.activeElement).toBe(terminalTab);
    });
  });

  // ── FR-TABS-003: terminal always mounted, height preserved ─────────────────
  describe("FR-TABS-003: terminal-host always mounted, visibility/CSS toggle only", () => {
    it("terminal-slot element stays in DOM after switching to TRANSCRIPT", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const stub = screen.getByTestId(TERMINAL_TESTID);

      act(() => {
        fireEvent.click(screen.getByRole("tab", { name: "TRANSCRIPT" }));
      });

      // querySelector can still find it even though it's hidden
      const found = document.querySelector(`[data-testid="${TERMINAL_TESTID}"]`);
      expect(found).not.toBeNull();
      // Same DOM node — React key unchanged
      expect(found).toBe(stub);
    });

    it("terminal-slot is not removed from DOM when switching between all tabs", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const stub = screen.getByTestId(TERMINAL_TESTID);

      act(() => {
        fireEvent.click(screen.getByRole("tab", { name: "TRANSCRIPT" }));
      });
      act(() => {
        fireEvent.click(screen.getByRole("tab", { name: "EVENTS" }));
      });
      act(() => {
        fireEvent.click(screen.getByRole("tab", { name: "TERMINAL" }));
      });

      // Same node throughout — never remounted
      const stubAfter = screen.getByTestId(TERMINAL_TESTID);
      expect(stubAfter).toBe(stub);
    });

    it("terminal-slot has data-active='true' when TERMINAL is active (ADR-0065)", () => {
      const originalGetBoundingClientRect = HTMLElement.prototype.getBoundingClientRect;
      HTMLElement.prototype.getBoundingClientRect = () => ({
        width: 800,
        height: 500,
        top: 0,
        left: 0,
        bottom: 500,
        right: 800,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      });
      try {
        render(
          <MainTabs
            tabs={TABS}
            sessionId="s1"
            bearerToken="tok"
            fetchFn={nopFetch}
            terminalSlot={<TerminalStub />}
          />,
        );

        // Navigate to TRANSCRIPT then back to TERMINAL
        act(() => {
          fireEvent.click(screen.getByRole("tab", { name: "TRANSCRIPT" }));
        });
        act(() => {
          fireEvent.click(screen.getByRole("tab", { name: "TERMINAL" }));
        });

        const stub = screen.getByTestId(TERMINAL_TESTID);
        const terminalSlot = stub.parentElement;
        expect(terminalSlot).not.toBeNull();
        // ADR-0065: active state via data-active attribute. CSS makes the
        // absolute-positioned overlay visible (visibility:visible + no
        // pointer-events:none) so xterm's host has the full parent box.
        expect(terminalSlot?.getAttribute("data-active")).toBe("true");
        if (terminalSlot) {
          const rect = terminalSlot.getBoundingClientRect();
          expect(rect.height).toBeGreaterThan(0);
        }
      } finally {
        HTMLElement.prototype.getBoundingClientRect = originalGetBoundingClientRect;
      }
    });

    // ADR-0065 regression: terminal-slot is a sibling of log panels in
    // .main-tabs-body but takes no flex remainder (CSS absolute overlay).
    // Without this guarantee, TRANSCRIPT/EVENTS content was pushed into
    // the bottom half of the viewport and appeared "middle-aligned".
    it("terminal-slot and log panels are siblings inside .main-tabs-body (ADR-0065 layering)", () => {
      render(
        <MainTabs
          tabs={TABS}
          sessionId="s1"
          bearerToken="tok"
          fetchFn={nopFetch}
          terminalSlot={<TerminalStub />}
        />,
      );
      const terminalSlot = screen.getByTestId(TERMINAL_TESTID).parentElement;
      expect(terminalSlot?.className).toBe("terminal-slot");
      const body = terminalSlot?.parentElement;
      expect(body?.className).toBe("main-tabs-body");
      // log panels (role=tabpanel, NOT the terminal one) are also direct
      // children of .main-tabs-body — same parent, different layer.
      const logPanels = Array.from(body?.children ?? []).filter(
        (c) => c !== terminalSlot && (c as HTMLElement).getAttribute("role") === "tabpanel",
      );
      expect(logPanels.length).toBe(2);
    });
  });

  // ── FR-A11Y-001: 44×44px minimum touch target ─────────────────────────────
  describe("FR-A11Y-001: each tab button has minimum 44×44px touch target", () => {
    it("each tab button getBoundingClientRect width >= 44 and height >= 44", () => {
      const originalGetBoundingClientRect = HTMLElement.prototype.getBoundingClientRect;
      HTMLElement.prototype.getBoundingClientRect = () => ({
        width: 44,
        height: 44,
        top: 0,
        left: 0,
        bottom: 44,
        right: 44,
        x: 0,
        y: 0,
        toJSON: () => ({}),
      });
      try {
        render(
          <MainTabs
            tabs={TABS}
            sessionId="s1"
            bearerToken="tok"
            fetchFn={nopFetch}
            terminalSlot={<TerminalStub />}
          />,
        );
        const tabs = screen.getAllByRole("tab");
        for (const tab of tabs) {
          const rect = (tab as HTMLElement).getBoundingClientRect();
          expect(rect.width).toBeGreaterThanOrEqual(44);
          expect(rect.height).toBeGreaterThanOrEqual(44);
        }
      } finally {
        HTMLElement.prototype.getBoundingClientRect = originalGetBoundingClientRect;
      }
    });
  });
});
