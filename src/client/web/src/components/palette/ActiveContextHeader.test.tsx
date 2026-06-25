// UAC-001 / UAC-002 / UAC-013 / FR-009 / FR-010 / FR-029 / FR-032 / ADR-0055 / ADR-0057
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { usePaletteStore } from "../../store/palette";
import type { ActiveContextSnapshot } from "../../store/palette_active_context";
import { ActiveContextHeader } from "./ActiveContextHeader";

function resetStore() {
  usePaletteStore.getState().close();
}

describe("ActiveContextHeader", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    resetStore();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  // UAC-001 / FR-009 / FR-029
  it("renders '— No active session' for kind=none", () => {
    const snap: ActiveContextSnapshot = { kind: "none" };
    render(<ActiveContextHeader snapshot={snap} flashSeq={0} />);
    const el = screen.getByRole("status");
    expect(el).toBeDefined();
    expect(el.textContent).toContain("No active session");
    // icon character present
    expect(el.textContent).toContain("—");
  });

  // UAC-001 / FR-009 / FR-028
  it("renders 'Active: bar / abcd1234' with correct title for kind=resolved", () => {
    const snap: ActiveContextSnapshot = {
      kind: "resolved",
      projBase: "bar",
      sid8: "abcd1234",
      fullPath: "/home/foo/bar",
      fullSessionId: "abcd1234efgh",
    };
    render(<ActiveContextHeader snapshot={snap} flashSeq={0} />);
    const el = screen.getByRole("status");
    expect(el.textContent).toContain("Active:");
    expect(el.textContent).toContain("bar");
    expect(el.textContent).toContain("abcd1234");
    expect(el.getAttribute("title")).toBe("/home/foo/bar\nabcd1234efgh");
  });

  // UAC-013 / FR-009 fallback
  it("renders 'Active: ??? / abcd1234' with title=fullSessionId for kind=unknown", () => {
    const snap: ActiveContextSnapshot = {
      kind: "unknown",
      sid8: "abcd1234",
      fullSessionId: "abcd1234efgh",
    };
    render(<ActiveContextHeader snapshot={snap} flashSeq={0} />);
    const el = screen.getByRole("status");
    expect(el.textContent).toContain("Active:");
    expect(el.textContent).toContain("???");
    expect(el.textContent).toContain("abcd1234");
    expect(el.getAttribute("title")).toBe("abcd1234efgh");
  });

  // FR-010 / FR-032: same flashSeq does not trigger flash
  it("does not add flash class when flashSeq remains the same", () => {
    const snap: ActiveContextSnapshot = { kind: "none" };
    const { rerender } = render(<ActiveContextHeader snapshot={snap} flashSeq={1} />);
    rerender(<ActiveContextHeader snapshot={snap} flashSeq={1} />);
    const el = screen.getByRole("status");
    expect(el.className).not.toContain("palette-active-context--flash");
  });

  // FR-010 / FR-032: changing flashSeq adds flash class for 600ms
  it("adds flash class when flashSeq changes and removes it after 600ms", () => {
    const snap: ActiveContextSnapshot = { kind: "none" };
    const { rerender } = render(<ActiveContextHeader snapshot={snap} flashSeq={1} />);
    // Change flashSeq from 1 to 2
    rerender(<ActiveContextHeader snapshot={snap} flashSeq={2} />);
    const el = screen.getByRole("status");
    expect(el.className).toContain("palette-active-context--flash");
    // Advance timers to 600ms and flush React state updates
    act(() => {
      vi.advanceTimersByTime(600);
    });
    expect(el.className).not.toContain("palette-active-context--flash");
  });

  // FR-032: consecutive flashSeq changes cancel previous timer
  it("cancels previous timer when flashSeq changes twice in rapid succession", () => {
    const snap: ActiveContextSnapshot = { kind: "none" };
    const { rerender } = render(<ActiveContextHeader snapshot={snap} flashSeq={1} />);
    // First change
    rerender(<ActiveContextHeader snapshot={snap} flashSeq={2} />);
    const el = screen.getByRole("status");
    // Advance partially (not full 600ms)
    act(() => {
      vi.advanceTimersByTime(300);
    });
    // Second change before first timer fires
    rerender(<ActiveContextHeader snapshot={snap} flashSeq={3} />);
    expect(el.className).toContain("palette-active-context--flash");
    // Advance 600ms from second change — flash should now be cleared
    act(() => {
      vi.advanceTimersByTime(600);
    });
    expect(el.className).not.toContain("palette-active-context--flash");
  });

  // ADR-0055: frozen props mode — store value is ignored when props.snapshot provided
  it("renders props.snapshot independently from store state (frozen mode)", () => {
    // Set store to resolved state
    usePaletteStore.getState().setActiveContextSnapshot({
      kind: "resolved",
      projBase: "store-proj",
      sid8: "storeid1",
      fullPath: "/store/proj",
      fullSessionId: "storeid1full",
    });
    // Provide a different frozen snapshot via props
    const frozenSnap: ActiveContextSnapshot = {
      kind: "unknown",
      sid8: "frozen01",
      fullSessionId: "frozen01full",
    };
    render(<ActiveContextHeader snapshot={frozenSnap} flashSeq={0} />);
    const el = screen.getByRole("status");
    // Should render the frozen snapshot, not the store one
    expect(el.textContent).toContain("???");
    expect(el.textContent).toContain("frozen01");
    expect(el.textContent).not.toContain("store-proj");
  });

  // ADR-0057: role=status must be present but aria-live must NOT be present
  it("has role=status but no aria-live attribute (ADR-0057)", () => {
    const snap: ActiveContextSnapshot = { kind: "none" };
    render(<ActiveContextHeader snapshot={snap} flashSeq={0} />);
    const el = screen.getByRole("status");
    expect(el.getAttribute("aria-live")).toBeNull();
  });
});
