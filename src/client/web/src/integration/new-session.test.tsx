// FR-A3 integration test — observes the full 5-step ordering of the
// New-Session flow end-to-end:
//
//   1. ctx.http.createSession(req)
//   2. ctx.daemonActions.selectSession(rc.id)
//   3. ctx.notify.success("Session created")
//   4. ctx.store.close()         (palette transitions open: true -> false)
//   5. opener.focus()            (CommandPalette's cleanup effect restores
//                                 focus to the Header's "New Session" button)
//
// Why this lives here (and not split across the unit tests):
//   - Unit tests for `newSessionTool` (lib/tools.test.ts) prove the
//     in-tool ordering (createSession -> selectSession -> notify -> close).
//   - Unit tests for the palette store (store/palette.test.ts) prove that
//     submit() invokes tool.submit + resets state to closed.
//   - Unit tests for CommandPalette (components/palette/CommandPalette.test.tsx)
//     prove that opener.focus() fires on the open=true -> false transition.
//   - Nothing previously asserted that all five steps observe in the right
//     order via a single shared timeline. FR-A3 (review blocker) demands a
//     single observation that pins the contract: if a future refactor moves
//     selectSession after close() — or fires opener.focus() before close —
//     the unit tests still pass but the user-visible behaviour breaks
//     (newly-created session not active, focus lost to <body>).
//
// We share one `calls` array across the four spies + a custom focus()
// override on the opener button. Each spy push()es a stable token, then
// the test asserts the array equals the documented order.

import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "../App";
// Mocking ./lib/platform mirrors App.test.tsx so useGlobalHotkey() in App
// does not crash when navigator surfaces are missing in the test env.
import { isMacPlatform } from "../lib/platform";
import { useDaemonStore } from "../store/daemon";
import { useNotificationsStore } from "../store/notifications";
import { usePaletteStore } from "../store/palette";

vi.mock("../lib/platform", () => ({
  isMacPlatform: vi.fn(),
}));

describe("FR-A3 new-session integration: 5-step ordering", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-24T00:00:00Z"));
    useDaemonStore.getState().reset();
    useNotificationsStore.setState({ items: [] });
    usePaletteStore.getState().close();
    vi.mocked(isMacPlatform).mockReturnValue(false);
    // Hang fetch forever so Connection.start() (mounted in App via useEffect)
    // never rejects and we are not racing the ws-ticket request in our
    // ordering assertion.
    vi.stubGlobal(
      "fetch",
      vi.fn(() => new Promise(() => {})),
    );
    window.location.hash = "#token=test";
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    window.location.hash = "";
    usePaletteStore.getState().close();
  });

  it("createSession -> selectSession -> notify.success -> close -> opener.focus()", async () => {
    // Seed daemon.sessionConfig with one project so ParamSelectPhase has
    // an option to pre-select for the project field. Without this the
    // project listbox is empty (FR-A4 path) and submit is unreachable.
    useDaemonStore.setState({
      sessions: [],
      activeSessionID: null,
      sessionConfig: {
        projects: [{ path: "/repo/a", isGit: false, isSandboxed: false }],
        pushCommands: [],
      },
    });

    // Shared timeline. Every observable side effect push()es here in the
    // order it fires; the assertion at the end is the FR-A3 contract.
    const calls: string[] = [];

    // 1. createSession spy on the SessionsApi factory inside CommandPalette.
    //    We patch via vi.spyOn on the daemon store + notifications store
    //    after render — but createSession lives behind the httpFactory the
    //    CommandPalette builds at construction. Patch it by intercepting
    //    fetch for POST /api/sessions.
    const origFetch = vi.mocked(fetch) as unknown as ReturnType<typeof vi.fn>;
    origFetch.mockImplementation((url: RequestInfo | URL, init?: RequestInit) => {
      const u = typeof url === "string" ? url : url.toString();
      if (u.endsWith("/api/session-config")) {
        // App's mount-time hydrate. Resolve with the same project we seeded.
        return Promise.resolve(
          new Response(
            JSON.stringify({
              commands: ["claude"],
              projects: [{ path: "/repo/a", isGit: false, isSandboxed: false }],
              push_commands: [],
            }),
            { status: 200, headers: { "Content-Type": "application/json" } },
          ),
        );
      }
      if (u.endsWith("/api/sessions") && init?.method === "POST") {
        calls.push("createSession");
        return Promise.resolve(
          new Response(JSON.stringify({ id: "sess-new" }), {
            status: 201,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }
      // ws-ticket / everything else: hang.
      return new Promise(() => {});
    });

    // 2. selectSession spy — wrap the daemon store action so we observe
    //    the call without breaking the actual state transition (the
    //    palette shell reads activeSessionID downstream).
    const realSelect = useDaemonStore.getState().selectSession;
    const selectSpy = vi.fn((id: string | null) => {
      calls.push("selectSession");
      realSelect(id);
    });
    useDaemonStore.setState({ selectSession: selectSpy });

    // 3. notify.success spy — wrap the notifications store add() so we
    //    observe the success toast keyed on its message text.
    const realAdd = useNotificationsStore.getState().add;
    const addSpy = vi.fn((item: Parameters<typeof realAdd>[0]) => {
      if (item.level === "info" && item.message === "Session created") {
        calls.push("notify.success(Session created)");
      }
      realAdd(item);
    });
    useNotificationsStore.setState({ add: addSpy });

    // 4. close spy — wrap the palette store's close action so we observe
    //    the call. The CommandPalette wires ctx.store.close to the
    //    closed-over reference captured at ctx-build time, so we must
    //    swap close BEFORE render and keep it stable for the lifetime
    //    of this test.
    const realClose = usePaletteStore.getState().close;
    const closeSpy = vi.fn(() => {
      calls.push("close");
      realClose();
    });
    usePaletteStore.setState({ close: closeSpy });

    render(<App />);

    // Capture the New Session button so we can (a) click it to open the
    // palette and (b) override its .focus() to record the timeline entry.
    const newBtn = screen.getByLabelText("New Session") as HTMLButtonElement;
    // 5. opener.focus spy — overriding the button's focus method captures
    //    the CommandPalette cleanup-effect call without us needing to spy
    //    on every HTMLElement.prototype.focus invocation (which would
    //    also catch ParamSelectPhase's internal focus moves).
    const realFocus = newBtn.focus.bind(newBtn);
    newBtn.focus = function focusOverride(): void {
      calls.push("opener.focus");
      realFocus();
    };

    // Open the palette via the Header CTA. This lands us at
    // phase='paramSelect', selectedToolId='new-session', cursor=0,
    // project listbox preselected to the single seeded project.
    act(() => {
      fireEvent.click(newBtn);
    });

    expect(usePaletteStore.getState().phase).toBe("paramSelect");
    expect(usePaletteStore.getState().selectedToolId).toBe("new-session");
    // Drain microtasks so useDynamicParamPreset's effect fires
    // setParam('project', '/repo/a') BEFORE we Enter past the project
    // field — otherwise advanceOrSubmit lands on a still-empty value.
    for (let i = 0; i < 5; i++) {
      await act(async () => {
        await Promise.resolve();
      });
    }
    expect(usePaletteStore.getState().paramValues.project).toBe("/repo/a");

    // Advance from project (cursor 0) to command (cursor 1) — the cursor
    // move is store-only (no calls timeline impact).
    act(() => {
      usePaletteStore.getState().moveCursor(+1);
    });

    // Type a command then submit. We drive Enter on the command input —
    // ParamTextInput's onKeyDown invokes advanceOrSubmit which (on the
    // final field) routes through usePaletteStore.submit(ctx), which
    // then calls newSessionTool.submit(ctx, payload), which fires the
    // 5-step sequence we're observing.
    // Use id-based query: two <label for="palette-param-command"> exist in the
    // tree (the fieldset wrapper + ParamTextInput's internal label) and
    // getByLabelText would error on the duplicate. The id is unique.
    const commandInput = document.getElementById("palette-param-command") as HTMLInputElement;
    expect(commandInput).not.toBeNull();
    act(() => {
      fireEvent.change(commandInput, { target: { value: "claude" } });
    });
    act(() => {
      fireEvent.keyDown(commandInput, { key: "Enter" });
    });

    // Drain microtasks for: fetch resolve -> selectSession ->
    // notify.success -> close (set initialClosedState) -> React re-render
    // -> CommandPalette cleanup effect -> opener.focus.
    for (let i = 0; i < 10; i++) {
      await act(async () => {
        await Promise.resolve();
      });
    }

    // FR-A3: the contract is the ordered sequence below. Equality (not
    // toContain) so a future regression that adds an extra side effect or
    // reorders the existing ones surfaces here as a diff rather than a
    // silent pass.
    expect(calls).toEqual([
      "createSession",
      "selectSession",
      "notify.success(Session created)",
      "close",
      "opener.focus",
    ]);

    // Sanity: the freshly-created session is now active (a regression of
    // selectSession would leave activeSessionID null even with the call
    // observed, since the spy delegates to realSelect).
    expect(useDaemonStore.getState().activeSessionID).toBe("sess-new");
  });
});
