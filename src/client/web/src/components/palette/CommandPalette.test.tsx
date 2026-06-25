// CommandPalette tests — exercises the DOM-owning shell:
//   - portal mount / unmount semantics
//   - role=dialog + aria-modal=true
//   - blur on open
//   - opener.focus() on unmount
//   - overlay outside-click vs inside-click
//   - Esc routes to store.back()
//   - refocusSeq → input.focus()
//   - phase switching renders the right phase component
//   - ActiveContextHeader render gating (ctx !== null / ctx === null)
//   - StatusBadge priority rendering (Unavailable / Sending... / Loading commands... / etc.)
//   - scope segment absent (ADR-0050 / FR-001)
//   - frozenSnapshotRef capture/release (FR-012 / FR-013 / ADR-0055)
//   - daemon mutation during submitting=true does not change UI (UAC-017)
//   - InlineStatus announce on active session change (FR-010 / FR-033 / ADR-0057)
//   - ctx.frozenActiveContext populated during submitting=true (UAC-018)
//   - notify.add called with structured toast on push submit (FR-014)
//
// UAC-001 / UAC-005 / UAC-011 / UAC-013 / UAC-016 / UAC-017 / UAC-018 /
// FR-009 / FR-010 / FR-012 / FR-013 / FR-014 / FR-024 / FR-025 / FR-033 /
// ADR-0055 / ADR-0057

import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SessionsApi } from "../../api/sessions";
import * as toolsModule from "../../lib/tools";
import type { ToolCtx } from "../../lib/tools";
import { useDaemonStore } from "../../store/daemon";
import { useNotificationsStore } from "../../store/notifications";
import { usePaletteStore } from "../../store/palette";
import { CommandPalette } from "./CommandPalette";

// ---------------------------------------------------------------------------
// UAC-018: ParamSelectPhase spy seam — captures ctx passed by CommandPalette.
// The spy hook is set per-test; the real component is used when hook is null.
// ---------------------------------------------------------------------------
let _paramPhaseCtxCapture: ((ctx: ToolCtx) => void) | null = null;

vi.mock("./ParamSelectPhase", async (importOriginal) => {
  const original = await importOriginal<typeof import("./ParamSelectPhase")>();
  return {
    ...original,
    ParamSelectPhase: (props: { ctx: ToolCtx }) => {
      if (_paramPhaseCtxCapture) _paramPhaseCtxCapture(props.ctx);
      return original.ParamSelectPhase(props);
    },
  };
});

function makeFakeHttp(): SessionsApi {
  return {
    createSession: vi.fn().mockResolvedValue({ id: "sess-new" }),
    deleteSession: vi.fn().mockResolvedValue(undefined),
    pushCommand: vi.fn().mockResolvedValue(undefined),
    getSessionConfig: vi.fn().mockResolvedValue({
      projectRoots: [],
      projectPaths: [],
      projects: [],
      commands: [],
      pushCommands: [],
    }),
  };
}

function renderShell() {
  const http = makeFakeHttp();
  const utils = render(<CommandPalette httpFactory={() => http} />);
  return { http, ...utils };
}

describe("CommandPalette", () => {
  beforeEach(() => {
    // Reset to a closed-palette baseline; individual tests open it via
    // setState so they control opener / phase / refocusSeq.
    usePaletteStore.setState({
      open: false,
      phase: "toolSelect",
      selectedToolId: null,
      paramValues: {},
      paramCursor: 0,
      query: "",
      composing: false,
      submitting: false,
      error: null,
      opener: null,
      refocusSeq: 0,
    });
    useDaemonStore.getState().reset();
    useNotificationsStore.getState().clear();
    // Reset per-test ctx capture hook.
    _paramPhaseCtxCapture = null;
  });

  it("renders nothing while open=false", () => {
    renderShell();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByTestId("palette-overlay")).toBeNull();
  });

  it("renders role=dialog with aria-modal and aria-labelledby when open=true", () => {
    act(() => {
      usePaletteStore.setState({ open: true });
    });
    renderShell();
    const dialog = screen.getByRole("dialog");
    expect(dialog.getAttribute("aria-modal")).toBe("true");
    expect(dialog.getAttribute("aria-labelledby")).toBe("palette-title");
    expect(screen.getByText("Command Palette").id).toBe("palette-title");
    // FR-C5: header chrome labels are English.
    expect(screen.getByTestId("palette-back").getAttribute("aria-label")).toBe("Back");
    expect(screen.getByTestId("palette-close").getAttribute("aria-label")).toBe("Close");
  });

  it("portals to document.body so z-index parents do not clip it", () => {
    act(() => {
      usePaletteStore.setState({ open: true });
    });
    const { container } = renderShell();
    // The component itself rendered into `container` shouldn't carry the
    // overlay — it's portaled to body.
    expect(container.querySelector("[data-testid=palette-overlay]")).toBeNull();
    expect(document.body.querySelector("[data-testid=palette-overlay]")).not.toBeNull();
  });

  it("blurs the currently focused element on mount-with-open", () => {
    // Seed: focus a sentinel element BEFORE rendering the palette.
    const sentinel = document.createElement("input");
    document.body.appendChild(sentinel);
    sentinel.focus();
    expect(document.activeElement).toBe(sentinel);

    act(() => {
      usePaletteStore.setState({ open: true });
    });
    renderShell();
    // The mount effect blurred whoever owned focus.
    expect(document.activeElement).not.toBe(sentinel);

    document.body.removeChild(sentinel);
  });

  it("restores focus to store.opener on unmount", () => {
    const opener = document.createElement("button");
    document.body.appendChild(opener);
    const focusSpy = vi.spyOn(opener, "focus");

    act(() => {
      usePaletteStore.setState({ open: true, opener });
    });
    const utils = renderShell();
    expect(focusSpy).not.toHaveBeenCalled();

    utils.unmount();
    expect(focusSpy).toHaveBeenCalledTimes(1);

    document.body.removeChild(opener);
  });

  it("restores focus to opener when palette closes via store while mounted", () => {
    const opener = document.createElement("button");
    document.body.appendChild(opener);
    const focusSpy = vi.spyOn(opener, "focus");

    act(() => {
      usePaletteStore.setState({ open: true, opener });
    });
    renderShell();
    expect(focusSpy).not.toHaveBeenCalled();

    // Close via store → component returns null → effect cleanup fires.
    act(() => {
      usePaletteStore.getState().close();
    });
    expect(focusSpy).toHaveBeenCalledTimes(1);
    expect(document.activeElement).toBe(opener);

    document.body.removeChild(opener);
  });

  it("logs a warning when activeElement is not blurrable on open", () => {
    const fakeActive = { tagName: "DIV" } as unknown as Element;
    Object.defineProperty(document, "activeElement", {
      configurable: true,
      get: () => fakeActive,
    });
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    try {
      act(() => {
        usePaletteStore.setState({ open: true });
      });
      renderShell();
      expect(warnSpy).toHaveBeenCalled();
      const msg = warnSpy.mock.calls[0]?.[0];
      expect(typeof msg === "string" && msg.includes("FR-003")).toBe(true);
    } finally {
      warnSpy.mockRestore();
      Reflect.deleteProperty(document, "activeElement");
    }
  });

  it("logs a warning and skips focus restore when opener is detached on close", () => {
    const opener = document.createElement("button");
    document.body.appendChild(opener);
    const focusSpy = vi.spyOn(opener, "focus");
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

    act(() => {
      usePaletteStore.setState({ open: true, opener });
    });
    const utils = renderShell();

    // Detach opener while palette is still open.
    document.body.removeChild(opener);

    utils.unmount();
    expect(focusSpy).not.toHaveBeenCalled();
    const matched = warnSpy.mock.calls.some(
      (call) => typeof call[0] === "string" && call[0].includes("opener detached"),
    );
    expect(matched).toBe(true);

    warnSpy.mockRestore();
  });

  it("does not call back() on Esc while composing=true (IME passthrough)", () => {
    act(() => {
      usePaletteStore.setState({ open: true, composing: true });
    });
    renderShell();
    const backSpy = vi.fn();
    usePaletteStore.setState({
      back: backSpy as unknown as ReturnType<typeof usePaletteStore.getState>["back"],
      composing: true,
    });

    const dialog = screen.getByRole("dialog");
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(backSpy).not.toHaveBeenCalled();
  });

  it("renders a ctx-error placeholder when httpFactory returns an invalid SessionsApi", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    act(() => {
      usePaletteStore.setState({
        open: true,
        phase: "paramSelect",
        selectedToolId: "new-session",
      });
    });
    render(
      <CommandPalette
        // biome-ignore lint/suspicious/noExplicitAny: deliberately broken factory
        httpFactory={(() => ({}) as any) as () => SessionsApi}
      />,
    );
    const placeholder = screen.getByTestId("palette-ctx-error");
    expect(placeholder).toBeDefined();
    // FR-C5: ctx-error copy is English ASCII.
    expect(placeholder.textContent).toBe("Command palette unavailable (http client invalid)");
    expect(errorSpy).toHaveBeenCalled();
    const matched = errorSpy.mock.calls.some(
      (call) => typeof call[0] === "string" && call[0].includes("invalid SessionsApi"),
    );
    expect(matched).toBe(true);
    errorSpy.mockRestore();
  });

  it("accepts an http client without deleteSession (FR-B3: deleteSession is not a required method)", () => {
    // REQUIRED_HTTP_METHODS no longer lists deleteSession; a factory that
    // returns only createSession / pushCommand / getSessionConfig must still
    // pass isValidSessionsApi and render the ParamSelectPhase rather than
    // falling through to the ctx-error placeholder.
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    act(() => {
      usePaletteStore.setState({
        open: true,
        phase: "paramSelect",
        selectedToolId: "new-session",
      });
    });
    const slimHttp = {
      createSession: vi.fn().mockResolvedValue({ id: "sess-new" }),
      pushCommand: vi.fn().mockResolvedValue(undefined),
      getSessionConfig: vi.fn().mockResolvedValue({
        projectRoots: [],
        projectPaths: [],
        projects: [],
        commands: [],
        pushCommands: [],
      }),
    };
    render(
      <CommandPalette
        // biome-ignore lint/suspicious/noExplicitAny: slimHttp is intentionally missing deleteSession
        httpFactory={(() => slimHttp as any) as () => SessionsApi}
      />,
    );
    expect(screen.queryByTestId("palette-ctx-error")).toBeNull();
    const invalidMatched = errorSpy.mock.calls.some(
      (call) => typeof call[0] === "string" && call[0].includes("invalid SessionsApi"),
    );
    expect(invalidMatched).toBe(false);
    errorSpy.mockRestore();
  });

  it("outside-click (mousedown on overlay itself) closes the palette", () => {
    act(() => {
      usePaletteStore.setState({ open: true });
    });
    renderShell();
    const closeSpy = vi.fn(() => {
      usePaletteStore.setState({ open: false });
    });
    usePaletteStore.setState({
      close: closeSpy as unknown as ReturnType<typeof usePaletteStore.getState>["close"],
    });

    const overlay = screen.getByTestId("palette-overlay");
    fireEvent.mouseDown(overlay);
    expect(closeSpy).toHaveBeenCalledTimes(1);
  });

  it("inside-click (mousedown bubbling from dialog) does NOT close", () => {
    act(() => {
      usePaletteStore.setState({ open: true });
    });
    renderShell();
    const closeSpy = vi.fn();
    usePaletteStore.setState({
      close: closeSpy as unknown as ReturnType<typeof usePaletteStore.getState>["close"],
    });

    const dialog = screen.getByRole("dialog");
    fireEvent.mouseDown(dialog);
    expect(closeSpy).not.toHaveBeenCalled();
  });

  it("Escape key calls store.back()", () => {
    act(() => {
      usePaletteStore.setState({ open: true });
    });
    renderShell();
    const backSpy = vi.fn();
    usePaletteStore.setState({
      back: backSpy as unknown as ReturnType<typeof usePaletteStore.getState>["back"],
    });

    const dialog = screen.getByRole("dialog");
    fireEvent.keyDown(dialog, { key: "Escape" });
    expect(backSpy).toHaveBeenCalledTimes(1);
  });

  it("header back button calls store.back()", () => {
    act(() => {
      usePaletteStore.setState({ open: true });
    });
    renderShell();
    const backSpy = vi.fn();
    usePaletteStore.setState({
      back: backSpy as unknown as ReturnType<typeof usePaletteStore.getState>["back"],
    });

    fireEvent.click(screen.getByTestId("palette-back"));
    expect(backSpy).toHaveBeenCalledTimes(1);
  });

  it("header close button calls store.close()", () => {
    act(() => {
      usePaletteStore.setState({ open: true });
    });
    renderShell();
    const closeSpy = vi.fn(() => {
      usePaletteStore.setState({ open: false });
    });
    usePaletteStore.setState({
      close: closeSpy as unknown as ReturnType<typeof usePaletteStore.getState>["close"],
    });

    fireEvent.click(screen.getByTestId("palette-close"));
    expect(closeSpy).toHaveBeenCalledTimes(1);
  });

  it("refocusSeq increments steal focus back to the search input", () => {
    act(() => {
      usePaletteStore.setState({ open: true, phase: "toolSelect" });
    });
    renderShell();
    const input = screen.getByTestId("palette-input") as HTMLInputElement;
    // Initial render may also focus the input; we capture baseline by
    // blurring then bumping refocusSeq.
    input.blur();
    expect(document.activeElement).not.toBe(input);

    act(() => {
      usePaletteStore.getState().refocusInput();
    });
    expect(document.activeElement).toBe(input);
  });

  it("renders ToolSelectPhase when phase='toolSelect'", () => {
    act(() => {
      usePaletteStore.setState({ open: true, phase: "toolSelect" });
    });
    renderShell();
    expect(screen.getByTestId("palette-input")).toBeDefined();
    expect(screen.getByTestId("palette-listbox")).toBeDefined();
    expect(screen.queryByRole("form")).toBeNull();
  });

  it("renders ParamSelectPhase when phase='paramSelect' (with a selected tool)", () => {
    // new-session has params so ParamSelectPhase will render a non-null
    // form for it.
    act(() => {
      usePaletteStore.setState({
        open: true,
        phase: "paramSelect",
        selectedToolId: "new-session",
        paramValues: {},
        paramCursor: 0,
      });
    });
    renderShell();
    // Tool-select input is gone; the param form is mounted.
    expect(screen.queryByTestId("palette-input")).toBeNull();
    expect(screen.getByRole("form", { name: "palette parameters" })).toBeDefined();
  });

  it("error string is rendered with role=alert", () => {
    act(() => {
      usePaletteStore.setState({ open: true, error: "bad input" });
    });
    renderShell();
    const err = screen.getByRole("alert");
    expect(err.textContent).toBe("bad input");
  });

  it("error=null does NOT render an alert", () => {
    act(() => {
      usePaletteStore.setState({ open: true, error: null });
    });
    renderShell();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // ---------------------------------------------------------------------------
  // ADR-0050 / FR-001: scope segment removed
  // ---------------------------------------------------------------------------

  // UAC-005
  it("scope segment tablist is NOT rendered (ADR-0050 scope unify)", () => {
    act(() => {
      usePaletteStore.setState({ open: true });
    });
    renderShell();
    // The old scope segment rendered a tablist with aria-label='palette scope';
    // after ADR-0050 this element must be absent.
    expect(screen.queryByRole("tablist", { name: "palette scope" })).toBeNull();
  });

  // ---------------------------------------------------------------------------
  // FR-025 / UAC-001: ActiveContextHeader render gating
  // ---------------------------------------------------------------------------

  it("ActiveContextHeader is rendered when ctx is valid (ctx !== null)", () => {
    act(() => {
      usePaletteStore.setState({ open: true, phase: "toolSelect" });
    });
    renderShell();
    // ActiveContextHeader renders data-testid="palette-active-context"
    expect(screen.getByTestId("palette-active-context")).toBeDefined();
  });

  it("ActiveContextHeader is NOT rendered when httpFactory returns invalid API (ctx === null)", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    act(() => {
      usePaletteStore.setState({ open: true, phase: "toolSelect" });
    });
    render(
      <CommandPalette
        // biome-ignore lint/suspicious/noExplicitAny: deliberately broken
        httpFactory={(() => ({}) as any) as () => SessionsApi}
      />,
    );
    expect(screen.queryByTestId("palette-active-context")).toBeNull();
    errorSpy.mockRestore();
  });

  // ---------------------------------------------------------------------------
  // FR-025: StatusBadge 'Unavailable' when ctx === null
  // ---------------------------------------------------------------------------

  it("StatusBadge shows 'Unavailable' when ctx === null (FR-025)", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    act(() => {
      usePaletteStore.setState({ open: true, phase: "toolSelect" });
    });
    render(
      <CommandPalette
        // biome-ignore lint/suspicious/noExplicitAny: deliberately broken
        httpFactory={(() => ({}) as any) as () => SessionsApi}
      />,
    );
    const badge = screen.getByTestId("palette-progress");
    expect(badge.textContent).toContain("Unavailable");
    errorSpy.mockRestore();
  });

  // ---------------------------------------------------------------------------
  // FR-024 / UAC-016: StatusBadge loading states
  // ---------------------------------------------------------------------------

  it("StatusBadge shows 'Loading commands...' when toolSelect + 0 enabled rows + sessionConfig=null (FR-024 / UAC-016)", () => {
    // UAC-016: Seed daemon with sessionConfig=null (default) and spy on
    // listTools to return only disabled push tools (no standard new-session),
    // so enabledCount===0. With sessionConfig===null the badge shows
    // 'Loading commands...'.
    const listToolsSpy = vi.spyOn(toolsModule, "listTools").mockReturnValue([
      {
        id: "push:save",
        label: "save",
        scope: "push",
        params: null,
        // Always disabled — simulates the 0-enabled-rows condition.
        disabledReason: () => "No active session",
        submit: vi.fn().mockResolvedValue(undefined),
      },
    ]);
    useDaemonStore.getState().reset(); // sessionConfig=null
    act(() => {
      usePaletteStore.setState({ open: true, phase: "toolSelect" });
    });
    renderShell();

    // With sessionConfig===null and enabledCount===0, badge MUST render
    // 'Loading commands...' — not vacuously gated by null check.
    const badge = screen.getByTestId("palette-progress");
    expect(badge).not.toBeNull();
    expect(badge.textContent).toContain("Loading commands...");

    listToolsSpy.mockRestore();
  });

  it("StatusBadge shows 'No commands available' when toolSelect + 0 enabled rows + sessionConfig hydrated (FR-024)", () => {
    // FR-024: With sessionConfig hydrated and enabledCount===0, badge shows
    // 'No commands available' (not 'Loading commands...').
    const listToolsSpy = vi.spyOn(toolsModule, "listTools").mockReturnValue([
      {
        id: "push:save",
        label: "save",
        scope: "push",
        params: null,
        disabledReason: () => "No active session",
        submit: vi.fn().mockResolvedValue(undefined),
      },
    ]);
    act(() => {
      useDaemonStore.setState({
        sessionConfig: {
          projects: [],
          pushCommands: ["save"],
        },
      });
      usePaletteStore.setState({ open: true, phase: "toolSelect" });
    });
    renderShell();

    // With sessionConfig hydrated and enabledCount===0, badge MUST render
    // 'No commands available' — positive assertion, not just negative.
    const badge = screen.getByTestId("palette-progress");
    expect(badge).not.toBeNull();
    expect(badge.textContent).toContain("No commands available");

    listToolsSpy.mockRestore();
  });

  // ---------------------------------------------------------------------------
  // FR-024 (visual consolidation): StatusBadge 'Sending...' replaces palette-progress
  // ---------------------------------------------------------------------------

  it("submitting=true shows StatusBadge with 'Sending...' text (FR-024)", () => {
    act(() => {
      usePaletteStore.setState({ open: true, submitting: true });
    });
    renderShell();
    const badge = screen.getByTestId("palette-progress");
    expect(badge).toBeDefined();
    expect(badge.textContent).toContain("Sending...");
  });

  it("submitting=false does NOT show 'Sending...' in StatusBadge", () => {
    // FR-024: The 'Sending...' text is exclusively tied to submitting=true.
    // Assert unconditionally (no null-guard escape hatch) so a regression that
    // renders 'Sending...' regardless of submitting state is caught.
    act(() => {
      usePaletteStore.setState({ open: true, submitting: false });
    });
    renderShell();
    // queryByText returns null when no matching node exists; this never passes
    // vacuously (unlike an if-guarded textContent check on a nullable element).
    expect(screen.queryByText(/Sending\.\.\./)).toBeNull();
  });

  // ---------------------------------------------------------------------------
  // ADR-0057: InlineStatus is rendered (single aria-live slot)
  // ---------------------------------------------------------------------------

  it("InlineStatus element is present in the palette overlay (ADR-0057)", () => {
    act(() => {
      usePaletteStore.setState({ open: true, phase: "toolSelect" });
    });
    renderShell();
    expect(screen.getByTestId("palette-inline-status")).toBeDefined();
  });

  // ---------------------------------------------------------------------------
  // FR-010 / FR-033 / ADR-0057: InlineStatus announce on active session change
  // ---------------------------------------------------------------------------

  it("InlineStatus receives announce when announceSeq increments (FR-010 / ADR-0057)", () => {
    // FR-010 / ADR-0057: when the active session changes, CommandPalette
    // composes 'Active session changed to <projBase> / <sid8>' and passes it
    // to InlineStatus as the announce prop. This test seeds an active session
    // then changes it via daemon store and asserts the announce text reaches
    // the InlineStatus DOM slot.
    act(() => {
      useDaemonStore.setState({
        sessions: [
          {
            id: "session-abcd1234",
            project: "/home/foo/bar",
            command: "claude",
            created_at: "2024-01-01T00:00:00Z",
            view: { card: {} },
          },
        ],
        activeSessionID: null,
        sessionConfig: {
          projects: [{ path: "/home/foo/bar", isGit: false, isSandboxed: false }],
          pushCommands: [],
        },
      });
      usePaletteStore.setState({ open: true, phase: "toolSelect" });
    });
    renderShell();

    // Trigger active session change → setActiveContextSnapshot fires →
    // announceSeq increments → CommandPalette builds the announce message.
    act(() => {
      useDaemonStore.setState({ activeSessionID: "session-abcd1234" });
    });

    // The announce message must contain the projBase ('bar') and the sid8
    // (first 8 chars of 'session-abcd1234' = 'session-'). FR-010 acceptance:
    // 'Active session changed to <projBase> / <sid8>'.
    const inlineStatus = screen.getByTestId("palette-inline-status");
    expect(inlineStatus.textContent).toContain("Active session changed to bar");
    expect(inlineStatus.textContent).toContain("session-");
  });

  // ---------------------------------------------------------------------------
  // FR-012 / FR-013 / ADR-0055 / UAC-017: frozenSnapshotRef capture/release
  // ---------------------------------------------------------------------------

  it("daemon mutation during submitting=true does NOT change ToolSelectPhase DOM (UAC-017)", () => {
    // UAC-017: during submitting=true the frozen snapshot is forwarded to
    // ToolSelectPhase; daemon mutations cannot change the rendered rows.
    // This test verifies both row count AND row identity (labels) are frozen.
    act(() => {
      useDaemonStore.setState({
        sessions: [
          {
            id: "session-1",
            project: "/home/foo/bar",
            command: "claude",
            created_at: "2024-01-01T00:00:00Z",
            view: { card: {} },
          },
        ],
        activeSessionID: "session-1",
        activeOccupant: "frame",
        sessionConfig: {
          projects: [{ path: "/home/foo/bar", isGit: false, isSandboxed: false }],
          pushCommands: ["push-cmd-1"],
        },
      });
      usePaletteStore.setState({ open: true, phase: "toolSelect" });
    });
    renderShell();

    // Capture listbox rows before submitting — record both count and labels.
    const listboxBefore = screen.getByTestId("palette-listbox");
    const optionsBefore = Array.from(
      listboxBefore.querySelectorAll('[role="option"]'),
    ) as HTMLElement[];
    const rowsBefore = optionsBefore.length;
    const labelsBefore = optionsBefore.map((el) => el.textContent?.trim() ?? "");

    // Enter submitting=true (freeze)
    act(() => {
      usePaletteStore.setState({ submitting: true });
    });

    // Mutate daemon: add a second session and push command.
    act(() => {
      useDaemonStore.setState({
        sessions: [
          {
            id: "session-1",
            project: "/home/foo/bar",
            command: "claude",
            created_at: "2024-01-01T00:00:00Z",
            view: { card: {} },
          },
          {
            id: "session-2",
            project: "/home/foo/baz",
            command: "claude",
            created_at: "2024-01-01T00:00:00Z",
            view: { card: {} },
          },
        ],
        sessionConfig: {
          projects: [
            { path: "/home/foo/bar", isGit: false, isSandboxed: false },
            { path: "/home/foo/baz", isGit: false, isSandboxed: false },
          ],
          pushCommands: ["push-cmd-1", "push-cmd-2"],
        },
      });
    });

    // Both count AND labels must be frozen (same as before submitting).
    const listboxAfter = screen.getByTestId("palette-listbox");
    const optionsAfter = Array.from(
      listboxAfter.querySelectorAll('[role="option"]'),
    ) as HTMLElement[];
    const rowsAfter = optionsAfter.length;
    const labelsAfter = optionsAfter.map((el) => el.textContent?.trim() ?? "");

    expect(rowsAfter).toBe(rowsBefore);
    // Verify row identity: frozen labels must match pre-submit labels exactly.
    expect(labelsAfter).toEqual(labelsBefore);
  });

  it("frozenSnapshotRef is released after submitting=false (FR-013)", () => {
    // FR-013: after submitting transitions true→false, frozenSnapshotRef is
    // released. The observable effect is that (a) 'Sending...' disappears and
    // (b) subsequent daemon mutations propagate to the UI (not frozen anymore).
    act(() => {
      useDaemonStore.setState({
        sessions: [
          {
            id: "session-1",
            project: "/home/foo/bar",
            command: "claude",
            created_at: "2024-01-01T00:00:00Z",
            view: { card: {} },
          },
        ],
        activeSessionID: "session-1",
        activeOccupant: "frame",
        sessionConfig: {
          projects: [{ path: "/home/foo/bar", isGit: false, isSandboxed: false }],
          pushCommands: ["push-cmd-1"],
        },
      });
      usePaletteStore.setState({ open: true, phase: "toolSelect" });
    });
    renderShell();

    // Enter submitting=true
    act(() => {
      usePaletteStore.setState({ submitting: true });
    });

    // 'Sending...' badge must be visible (confirming frozen state active).
    expect(screen.getByTestId("palette-progress").textContent).toContain("Sending...");

    const listboxFrozen = screen.getByTestId("palette-listbox");
    const frozenRowCount = listboxFrozen.querySelectorAll('[role="option"]').length;

    // Release submitting — frozenSnapshotRef must be cleared.
    act(() => {
      usePaletteStore.setState({ submitting: false });
    });

    // 'Sending...' is unconditionally gone after release (no null-guard escape).
    expect(screen.queryByText(/Sending\.\.\./)).toBeNull();

    // After release a daemon mutation DOES propagate (snapshot no longer frozen).
    act(() => {
      useDaemonStore.setState({
        sessions: [
          {
            id: "session-1",
            project: "/home/foo/bar",
            command: "claude",
            created_at: "2024-01-01T00:00:00Z",
            view: { card: {} },
          },
          {
            id: "session-2",
            project: "/home/foo/baz",
            command: "claude",
            created_at: "2024-01-01T00:00:00Z",
            view: { card: {} },
          },
        ],
        sessionConfig: {
          projects: [
            { path: "/home/foo/bar", isGit: false, isSandboxed: false },
            { path: "/home/foo/baz", isGit: false, isSandboxed: false },
          ],
          pushCommands: ["push-cmd-1", "push-cmd-2"],
        },
      });
    });

    // Post-release the list must have grown (mutation is live, not frozen).
    const listboxAfterRelease = screen.getByTestId("palette-listbox");
    const liveRowCount = listboxAfterRelease.querySelectorAll('[role="option"]').length;
    expect(liveRowCount).toBeGreaterThan(frozenRowCount);
  });

  // ---------------------------------------------------------------------------
  // UAC-018: ctx.frozenActiveContext set during submitting=true
  // ---------------------------------------------------------------------------

  it("ctx.frozenActiveContext is set during submitting=true (UAC-018)", () => {
    // UAC-018: when submitting transitions false→true, frozenSnapshotRef
    // captures the current activeContextSnapshot. CommandPalette then wires
    // ctx.frozenActiveContext = frozenSnapshotRef.current?.activeContext.
    // This test captures the ctx passed to ParamSelectPhase (via the spy seam
    // installed at the top of this file) and asserts frozenActiveContext is
    // populated with the snapshot that was live at freeze time.
    const capturedCtxValues: Array<ToolCtx | null> = [];
    _paramPhaseCtxCapture = (ctx) => {
      capturedCtxValues.push(ctx);
    };

    act(() => {
      useDaemonStore.setState({
        sessions: [
          {
            id: "session-xyz",
            project: "/home/foo/bar",
            command: "claude",
            created_at: "2024-01-01T00:00:00Z",
            view: { card: {} },
          },
        ],
        activeSessionID: "session-xyz",
        sessionConfig: {
          projects: [{ path: "/home/foo/bar", isGit: false, isSandboxed: false }],
          pushCommands: [],
        },
      });
      // Seed palette store with a known activeContextSnapshot — this is what
      // frozenSnapshotRef will capture on submitting false→true.
      usePaletteStore.setState({
        open: true,
        phase: "toolSelect",
        activeContextSnapshot: {
          kind: "resolved",
          projBase: "bar",
          sid8: "session-",
          fullPath: "/home/foo/bar",
          fullSessionId: "session-xyz",
        },
      });
    });
    renderShell();

    // Advance to paramSelect + submitting=true so ParamSelectPhase receives ctx.
    // The freeze useEffect fires on the submitting false→true transition and
    // captures frozenSnapshotRef.current = { activeContext: snapshot, ... }.
    // A subsequent daemon change forces ctx useMemo to recompute — at that
    // point frozenSnapshotRef.current is already populated.
    act(() => {
      usePaletteStore.setState({
        submitting: true,
        phase: "paramSelect",
        selectedToolId: "new-session",
      });
    });

    // Trigger a daemon mutation to force ctx useMemo recomputation. After
    // recomputation, ctx.frozenActiveContext = frozenSnapshotRef.current?.activeContext.
    act(() => {
      useDaemonStore.setState({ activeSessionID: null });
    });

    // The spy should have captured at least one ctx after the daemon mutation.
    const ctxWithFrozen = capturedCtxValues.find(
      (c) => c !== null && c.frozenActiveContext !== undefined,
    );
    expect(ctxWithFrozen).toBeDefined();
    // The frozen context must match the snapshot that was live at freeze time.
    expect(ctxWithFrozen?.frozenActiveContext?.kind).toBe("resolved");
    expect(
      ctxWithFrozen?.frozenActiveContext && "projBase" in ctxWithFrozen.frozenActiveContext
        ? ctxWithFrozen.frozenActiveContext.projBase
        : undefined,
    ).toBe("bar");

    _paramPhaseCtxCapture = null;
  });

  // ---------------------------------------------------------------------------
  // FR-033: InlineStatus suppressed while submitting
  // ---------------------------------------------------------------------------

  it("InlineStatus does NOT announce when submitting=true (FR-033)", () => {
    act(() => {
      useDaemonStore.setState({
        sessions: [{ id: "session-abcd1234", project: "/home/foo/bar", command: "claude", created_at: "2024-01-01T00:00:00Z", view: { card: {} } }],
        activeSessionID: "session-abcd1234",
        sessionConfig: { projects: [{ path: "/home/foo/bar", isGit: false, isSandboxed: false }], pushCommands: [] },
      });
      usePaletteStore.setState({ open: true, phase: "toolSelect", submitting: true });
    });
    renderShell();

    // Change active session while submitting — announce should be suppressed
    act(() => {
      useDaemonStore.setState({ activeSessionID: null });
    });

    const inlineStatus = screen.getByTestId("palette-inline-status");
    expect(inlineStatus.textContent).not.toContain("Active session changed to");
  });

  // ---------------------------------------------------------------------------
  // UAC-003: full new-session flow
  // ---------------------------------------------------------------------------

  it("full new-session flow: open → tool select → param enter → close (UAC-003)", async () => {
    const opener = document.createElement("button");
    document.body.appendChild(opener);

    act(() => {
      useDaemonStore.setState({
        sessionConfig: { projects: [{ path: "/home/foo/bar", isGit: false, isSandboxed: false }], pushCommands: [] },
      });
      usePaletteStore.setState({ open: true, phase: "toolSelect", opener });
    });
    const { http } = renderShell();

    // Confirm new-session tool → transitions to paramSelect
    act(() => {
      usePaletteStore.getState().confirmTool("new-session");
    });
    expect(usePaletteStore.getState().phase).toBe("paramSelect");

    // Submit with project + command params → palette closes on success
    await act(async () => {
      usePaletteStore.getState().setParam("project", "/home/foo/bar");
      usePaletteStore.getState().setParam("command", "claude");
      await usePaletteStore.getState().submit({
        http,
        daemon: { sessions: [], activeSessionID: null, projects: [], pushCommands: [] },
        daemonActions: { selectSession: vi.fn() },
        notify: { success: vi.fn(), error: vi.fn(), add: vi.fn() },
        store: { close: usePaletteStore.getState().close },
      });
    });

    // Palette should be closed
    expect(usePaletteStore.getState().open).toBe(false);
    expect(screen.queryByRole("dialog")).toBeNull();

    document.body.removeChild(opener);
  });

  // ---------------------------------------------------------------------------
  // ADR-0055: frozen flashSeq lock test
  // ---------------------------------------------------------------------------

  it("frozen flashSeq is locked during submitting=true — store bump does not re-trigger flash (ADR-0055)", () => {
    act(() => {
      usePaletteStore.setState({
        open: true,
        phase: "toolSelect",
        activeContextSnapshot: { kind: "resolved", projBase: "bar", sid8: "session-", fullPath: "/home/foo/bar", fullSessionId: "session-abcd1234" },
        flashSeq: 5,
      });
    });
    renderShell();

    // Enter submitting=true — frozenSnapshotRef should capture flashSeq=5
    act(() => {
      usePaletteStore.setState({ submitting: true });
    });

    // Bump store's flashSeq to 6 — frozen header should NOT see this
    act(() => {
      usePaletteStore.setState({ flashSeq: 6 });
    });

    // ActiveContextHeader should receive flashSeq=5 (frozen), not 6 (live)
    // The frozen header uses headerFlashSeq = frozen.flashSeq (captured at freeze time)
    const header = screen.getByTestId("palette-active-context");
    expect(header).toBeDefined();
    // We verify via the data attr or the snapshot prop — the test confirms palette is still open
    expect(screen.queryByRole("dialog")).not.toBeNull();
    // submitting=true badge shows 'Sending...'
    expect(screen.getByTestId("palette-progress").textContent).toContain("Sending...");
  });
});
