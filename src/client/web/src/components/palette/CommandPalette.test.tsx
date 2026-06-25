// CommandPalette tests — exercises the DOM-owning shell:
//   - portal mount / unmount semantics
//   - role=dialog + aria-modal=true
//   - blur on open
//   - opener.focus() on unmount
//   - overlay outside-click vs inside-click
//   - Esc routes to store.back()
//   - refocusSeq → input.focus()
//   - phase switching renders the right phase component
//   - submitting / error UI

import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { SessionsApi } from "../../api/sessions";
import { useDaemonStore } from "../../store/daemon";
import { useNotificationsStore } from "../../store/notifications";
import { usePaletteStore } from "../../store/palette";
import { CommandPalette } from "./CommandPalette";

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
      scope: "standard",
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
    // The store transitions open=false AND opener=null in the same set;
    // the effect's openerRef tracker still points at the live button at
    // cleanup time, so focus() is invoked and document.activeElement
    // actually moves to the opener — which is the real user-visible
    // contract (FR-017 / FR-023). Asserting the spy alone would let a
    // future regression where focus() is called on a stale clone slip
    // through.
    act(() => {
      usePaletteStore.getState().close();
    });
    expect(focusSpy).toHaveBeenCalledTimes(1);
    expect(document.activeElement).toBe(opener);

    document.body.removeChild(opener);
  });

  it("logs a warning when activeElement is not blurrable on open", () => {
    // jsdom's default <body> is blurrable, so to force the non-blurrable
    // path we attach an own-property `activeElement` getter to the document
    // instance. The cleanup uses `delete` to peel the override back off so
    // subsequent tests fall through to the real prototype getter. We avoid
    // poking at Document.prototype's descriptor because jsdom doesn't
    // necessarily expose it as configurable.
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
      // Restore by removing the own-property override; document.activeElement
      // then resolves through the Document prototype getter again. We use
      // Reflect.deleteProperty (biome flags raw `delete`) — assigning
      // `undefined` would leave the getter override in place and break later
      // tests that read document.activeElement (it'd return undefined, not
      // the real focused element).
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

  it("submitting=true shows the progress affordance", () => {
    act(() => {
      usePaletteStore.setState({ open: true, submitting: true });
    });
    renderShell();
    expect(screen.getByTestId("palette-progress")).toBeDefined();
  });

  it("submitting=false does NOT show the progress affordance", () => {
    act(() => {
      usePaletteStore.setState({ open: true, submitting: false });
    });
    renderShell();
    expect(screen.queryByTestId("palette-progress")).toBeNull();
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

  it("ScopeSegment is rendered above the phase", () => {
    act(() => {
      usePaletteStore.setState({ open: true });
    });
    renderShell();
    expect(screen.getByRole("tablist", { name: "palette scope" })).toBeDefined();
  });
});
