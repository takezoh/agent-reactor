// UAC-005 / UAC-013 / UAC-017 / FR-031 / FR-033 / ADR-0057
//
// InlineStatus — single aria-live='polite' announce slot for the palette.
//
// Coverage:
//   - empty inlineStatus.message → slot is empty but aria-live container exists
//   - emitDisabledFeedback → text appears with warning icon
//   - same-message re-emit → DOM remounts (data-seq attribute changes)
//   - 4 s auto-clear via fake timers
//   - props.announce overrides inlineStatus (higher priority)
//   - aria-live='polite' / role='status' present exactly once
//   - kind='warning' renders '!' icon

import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { usePaletteStore } from "../../store/palette";
import { INLINE_STATUS_AUTO_CLEAR_MS } from "../../store/palette_inline_status";
import { InlineStatus } from "./InlineStatus";

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

function renderStatus(props: Parameters<typeof InlineStatus>[0] = {}) {
  return render(<InlineStatus {...props} />);
}

// Reset the palette store to a clean baseline before every test.
beforeEach(() => {
  usePaletteStore.setState({
    inlineStatus: {
      message: "",
      kind: "warning",
      seq: 0,
      timerId: null,
    },
  });
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

describe("InlineStatus — empty state (FR-033 / ADR-0057)", () => {
  // UAC-005
  it("renders the aria-live container even when message is empty", () => {
    renderStatus();
    const slot = screen.getByTestId("palette-inline-status");
    expect(slot).toBeTruthy();
    expect(slot.getAttribute("aria-live")).toBe("polite");
    // <output> has implicit role="status"; getAttribute returns null for implicit roles
    expect(slot.tagName.toLowerCase()).toBe("output");
    // No text child when message is empty
    expect(slot.textContent?.trim()).toBe("");
  });
});

describe("InlineStatus — emitDisabledFeedback (FR-031 / FR-033)", () => {
  // UAC-013
  it("shows warning text after emitDisabledFeedback", () => {
    renderStatus();
    act(() => {
      usePaletteStore.getState().emitDisabledFeedback("save", "No active session");
    });
    const slot = screen.getByTestId("palette-inline-status");
    expect(slot.textContent).toContain('"save" is unavailable: No active session');
  });

  it("renders a warning icon ('!') alongside the message", () => {
    renderStatus();
    act(() => {
      usePaletteStore.getState().emitDisabledFeedback("run", "No active session");
    });
    const slot = screen.getByTestId("palette-inline-status");
    const icon = slot.querySelector(".palette-inline-status__icon");
    expect(icon).toBeTruthy();
    expect(icon?.getAttribute("aria-hidden")).toBe("true");
  });

  // UAC-017 / FR-031 — same-message re-emit causes DOM remount (data-seq changes)
  it("increments data-seq on each re-emit, even for identical messages", () => {
    renderStatus();

    act(() => {
      usePaletteStore.getState().emitDisabledFeedback("save", "No active session");
    });
    const slot = screen.getByTestId("palette-inline-status");
    const firstSpan = slot.querySelector("span[data-seq]");
    const firstSeq = firstSpan?.getAttribute("data-seq");

    act(() => {
      // Advance time a bit so timer does not clear between the two emits
      vi.advanceTimersByTime(100);
      usePaletteStore.getState().emitDisabledFeedback("save", "No active session");
    });

    const secondSpan = slot.querySelector("span[data-seq]");
    const secondSeq = secondSpan?.getAttribute("data-seq");

    expect(secondSeq).not.toBe(firstSeq);
  });

  it("auto-clears after 4 s (INLINE_STATUS_AUTO_CLEAR_MS)", () => {
    renderStatus();
    act(() => {
      usePaletteStore.getState().emitDisabledFeedback("run", "No active session");
    });
    // Confirm text is visible
    expect(screen.getByTestId("palette-inline-status").textContent).toContain("unavailable");

    act(() => {
      vi.advanceTimersByTime(INLINE_STATUS_AUTO_CLEAR_MS);
    });
    // After auto-clear the slot text should be empty
    expect(screen.getByTestId("palette-inline-status").textContent?.trim()).toBe("");
  });
});

describe("InlineStatus — props.announce priority (ADR-0057)", () => {
  it("shows announce message when seq is higher than lastAnnounceSeq", () => {
    const { rerender } = render(<InlineStatus announce={undefined} />);
    // seq=1 is new (initialised to 0) → triggers useEffect → showingAnnounce=true
    act(() => {
      rerender(<InlineStatus announce={{ message: "Active session changed to foo", seq: 1 }} />);
    });
    const slot = screen.getByTestId("palette-inline-status");
    expect(slot.textContent).toContain("Active session changed to foo");
  });

  it("announce expires after 4 s and falls back to inlineStatus", () => {
    const { rerender } = render(<InlineStatus announce={undefined} />);
    act(() => {
      rerender(<InlineStatus announce={{ message: "Session changed", seq: 1 }} />);
    });
    act(() => {
      vi.advanceTimersByTime(INLINE_STATUS_AUTO_CLEAR_MS);
    });
    // After expiry and no inlineStatus.message, slot should be empty
    const slot = screen.getByTestId("palette-inline-status");
    expect(slot.textContent?.trim()).toBe("");
  });

  it("announce seq increment triggers re-display even with same message", () => {
    const { rerender } = render(<InlineStatus announce={undefined} />);
    act(() => {
      rerender(<InlineStatus announce={{ message: "Session changed", seq: 1 }} />);
    });
    act(() => {
      vi.advanceTimersByTime(INLINE_STATUS_AUTO_CLEAR_MS);
    });
    // seq 2 re-triggers showingAnnounce
    act(() => {
      rerender(<InlineStatus announce={{ message: "Session changed", seq: 2 }} />);
    });
    const slot = screen.getByTestId("palette-inline-status");
    expect(slot.textContent).toContain("Session changed");
  });

  // UAC-017 / ADR-0057 — announce MUST override a non-empty inlineStatus.message
  it("announce overrides a non-empty inlineStatus.message", () => {
    // First populate inlineStatus so the slot has a non-empty message.
    const { rerender } = render(<InlineStatus announce={undefined} />);
    act(() => {
      usePaletteStore.getState().emitDisabledFeedback("save", "No active session");
    });
    const slot = screen.getByTestId("palette-inline-status");
    // Confirm inlineStatus message is visible before the announce arrives.
    expect(slot.textContent).toContain('"save" is unavailable: No active session');

    // Now provide an announce with seq=1 (higher than lastAnnounceSeq=-1) — it
    // must override the non-empty inlineStatus message.
    act(() => {
      rerender(<InlineStatus announce={{ message: "Active session changed to bar", seq: 1 }} />);
    });
    // The announce message must be present and the inlineStatus message must not.
    expect(slot.textContent).toContain("Active session changed to bar");
    expect(slot.textContent).not.toContain('"save" is unavailable');
  });

  // ADR-0057 — announce is shown immediately when present at first mount
  it("shows announce message when mounted with an announce prop directly (no prior rerender)", () => {
    render(<InlineStatus announce={{ message: "Session started", seq: 1 }} />);
    const slot = screen.getByTestId("palette-inline-status");
    expect(slot.textContent).toContain("Session started");
  });
});

describe("InlineStatus — single aria-live slot (ADR-0057)", () => {
  it("renders exactly one element with role=status and aria-live=polite", () => {
    renderStatus();
    const slots = screen.getAllByRole("status");
    expect(slots).toHaveLength(1);
    const first = slots[0];
    expect(first?.getAttribute("aria-live")).toBe("polite");
  });
});
