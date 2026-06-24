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
    // Terminal slot is always mounted and starts active (visible).
    const slot = screen.getByTestId(TERMINAL_TESTID).parentElement;
    expect(slot?.className).toBe("terminal-slot");
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

  it("clicking TRANSCRIPT hides the terminal slot via .hidden but keeps it mounted", () => {
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
    expect(stub.parentElement?.className).toBe("terminal-slot");

    act(() => {
      fireEvent.click(screen.getByRole("tab", { name: "TRANSCRIPT" }));
    });

    // Still mounted (same node), but wrapper carries the .hidden class so
    // CSS display:none takes over. ADR 0030 subscribe ownership and xterm
    // scrollback survive the switch because the slot is never unmounted.
    const stubAfter = screen.getByTestId(TERMINAL_TESTID);
    expect(stubAfter).toBe(stub);
    expect(stubAfter.parentElement?.className).toBe("terminal-slot hidden");
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

  it("clicking back to TERMINAL re-shows the slot (no .hidden, aria-hidden=false)", () => {
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
    expect(stub.parentElement?.className).toBe("terminal-slot");
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
});
