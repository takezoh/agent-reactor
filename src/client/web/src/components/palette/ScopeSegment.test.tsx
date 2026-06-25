import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../../store/daemon";
import { usePaletteStore } from "../../store/palette";
import { ScopeSegment } from "./ScopeSegment";

// resetStores wipes both daemon + palette singletons between tests so the
// previous test's scope / activeOccupant cannot bleed into the current
// render — each `it` builds the world it asserts on from scratch.
function resetStores() {
  useDaemonStore.getState().reset();
  usePaletteStore.getState().close();
}

describe("ScopeSegment", () => {
  beforeEach(() => {
    resetStores();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("FR-004: both scopes enabled when active session has frame occupant; standard is initially active", () => {
    useDaemonStore.setState({
      activeSessionID: "s1",
      activeOccupant: "frame",
    });
    // palette.scope defaults to "standard" on close()/initial state; we render
    // without opening because ScopeSegment reads scope directly off the store
    // and does not depend on `open`.
    render(<ScopeSegment />);
    const standard = screen.getByRole("tab", { name: /standard/ });
    const push = screen.getByRole("tab", { name: /push/ });
    expect(standard.hasAttribute("disabled")).toBe(false);
    expect(push.hasAttribute("disabled")).toBe(false);
    expect(standard.getAttribute("aria-pressed")).toBe("true");
    expect(push.getAttribute("aria-pressed")).toBe("false");
  });

  it("FR-005: push is disabled with 'No active session' when there is no active session", () => {
    useDaemonStore.setState({
      activeSessionID: null,
      activeOccupant: "frame",
    });
    render(<ScopeSegment />);
    const push = screen.getByRole("tab", { name: /push/ });
    expect(push.hasAttribute("disabled")).toBe(true);
    expect(push.textContent).toContain("No active session");
    // standard remains enabled — push being unavailable must not block the
    // common create/stop tools.
    const standard = screen.getByRole("tab", { name: /standard/ });
    expect(standard.hasAttribute("disabled")).toBe(false);
  });

  it("FR-006: push is disabled with 'No push-capable driver' when active session occupant is not 'frame'", () => {
    useDaemonStore.setState({
      activeSessionID: "s1",
      activeOccupant: "main",
    });
    render(<ScopeSegment />);
    const push = screen.getByRole("tab", { name: /push/ });
    expect(push.hasAttribute("disabled")).toBe(true);
    expect(push.textContent).toContain("No push-capable driver");
  });

  it("FR-006: push is disabled (fail-closed) when activeOccupant is undefined", () => {
    // The wire does not yet carry occupant; until it does, the field stays
    // undefined and push must remain disabled rather than firing against a
    // potentially-non-frame pane.
    useDaemonStore.setState({
      activeSessionID: "s1",
      activeOccupant: undefined,
    });
    render(<ScopeSegment />);
    const push = screen.getByRole("tab", { name: /push/ });
    expect(push.hasAttribute("disabled")).toBe(true);
    expect(push.textContent).toContain("No push-capable driver");
  });

  it("calls setScope('push') when push button is clicked and push is enabled", () => {
    useDaemonStore.setState({
      activeSessionID: "s1",
      activeOccupant: "frame",
    });
    const setScopeSpy = vi.spyOn(usePaletteStore.getState(), "setScope");
    render(<ScopeSegment />);
    fireEvent.click(screen.getByRole("tab", { name: /push/ }));
    expect(setScopeSpy).toHaveBeenCalledWith("push");
  });

  it("does not call setScope when a disabled scope is clicked", () => {
    useDaemonStore.setState({
      activeSessionID: null,
      activeOccupant: undefined,
    });
    const setScopeSpy = vi.spyOn(usePaletteStore.getState(), "setScope");
    render(<ScopeSegment />);
    fireEvent.click(screen.getByRole("tab", { name: /push/ }));
    // A disabled <button> swallows click events at the DOM layer, so
    // setScope must never be observed for a fail-closed push.
    expect(setScopeSpy).not.toHaveBeenCalled();
  });

  it("reflects the active scope via aria-pressed and data-active when scope is 'push'", () => {
    useDaemonStore.setState({
      activeSessionID: "s1",
      activeOccupant: "frame",
    });
    usePaletteStore.setState({ scope: "push" });
    render(<ScopeSegment />);
    const push = screen.getByRole("tab", { name: /push/ });
    const standard = screen.getByRole("tab", { name: /standard/ });
    expect(push.getAttribute("aria-pressed")).toBe("true");
    expect(push.getAttribute("data-active")).toBe("true");
    expect(standard.getAttribute("aria-pressed")).toBe("false");
    expect(standard.getAttribute("data-active")).toBeNull();
  });

  it("calls setScope('standard') when standard is clicked from push", () => {
    useDaemonStore.setState({
      activeSessionID: "s1",
      activeOccupant: "frame",
    });
    usePaletteStore.setState({ scope: "push" });
    const setScopeSpy = vi.spyOn(usePaletteStore.getState(), "setScope");
    render(<ScopeSegment />);
    fireEvent.click(screen.getByRole("tab", { name: /standard/ }));
    expect(setScopeSpy).toHaveBeenCalledWith("standard");
  });
});
