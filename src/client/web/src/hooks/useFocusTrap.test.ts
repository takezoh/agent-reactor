import { renderHook } from "@testing-library/react";
import { createRef } from "react";
import type { RefObject } from "react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { useFocusTrap } from "./useFocusTrap";

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------
// Build a real container with three buttons attached to document.body so that
// happy-dom reports a non-null offsetParent for each (matches the production
// path where the dialog is mounted into the DOM).
// ---------------------------------------------------------------------------

interface Harness {
  container: HTMLDivElement;
  buttons: HTMLButtonElement[];
  ref: RefObject<HTMLElement>;
}

function setup(buttonCount = 3): Harness {
  const container = document.createElement("div");
  const buttons: HTMLButtonElement[] = [];
  for (let i = 0; i < buttonCount; i++) {
    const b = document.createElement("button");
    b.textContent = `btn-${i}`;
    container.appendChild(b);
    buttons.push(b);
  }
  document.body.appendChild(container);
  const ref = createRef<HTMLElement>();
  // useFocusTrap reads ref.current inside useEffect; assign synchronously so
  // the effect can see the container on mount.
  (ref as { current: HTMLElement | null }).current = container;
  return { container, buttons, ref };
}

function dispatchTab(target: HTMLElement, shift = false): KeyboardEvent {
  const ev = new KeyboardEvent("keydown", {
    key: "Tab",
    shiftKey: shift,
    bubbles: true,
    cancelable: true,
  });
  target.dispatchEvent(ev);
  return ev;
}

describe("useFocusTrap", () => {
  let cleanup: (() => void) | null = null;

  beforeEach(() => {
    document.body.replaceChildren();
  });
  afterEach(() => {
    cleanup?.();
    cleanup = null;
    document.body.replaceChildren();
  });

  it("cycles Tab from last tabbable back to the first (preventDefault)", () => {
    const { container, buttons, ref } = setup(3);
    const hook = renderHook(({ enabled }) => useFocusTrap(ref, enabled), {
      initialProps: { enabled: true },
    });
    cleanup = hook.unmount;

    const last = buttons[2] as HTMLButtonElement;
    const first = buttons[0] as HTMLButtonElement;
    last.focus();
    expect(document.activeElement).toBe(last);

    const ev = dispatchTab(container);
    expect(ev.defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(first);
  });

  it("cycles Shift+Tab from first tabbable back to the last (preventDefault)", () => {
    const { container, buttons, ref } = setup(3);
    const hook = renderHook(({ enabled }) => useFocusTrap(ref, enabled), {
      initialProps: { enabled: true },
    });
    cleanup = hook.unmount;

    const first = buttons[0] as HTMLButtonElement;
    const last = buttons[2] as HTMLButtonElement;
    first.focus();
    expect(document.activeElement).toBe(first);

    const ev = dispatchTab(container, true);
    expect(ev.defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(last);
  });

  it("does not preventDefault when Tab is pressed on an interior tabbable", () => {
    const { container, buttons, ref } = setup(3);
    const hook = renderHook(({ enabled }) => useFocusTrap(ref, enabled), {
      initialProps: { enabled: true },
    });
    cleanup = hook.unmount;

    const middle = buttons[1] as HTMLButtonElement;
    middle.focus();
    expect(document.activeElement).toBe(middle);

    const evFwd = dispatchTab(container);
    expect(evFwd.defaultPrevented).toBe(false);
    // Focus is left to the browser's native handling — happy-dom does not
    // advance focus on Tab, so activeElement stays put. The contract here is
    // strictly "we did not preventDefault".
    expect(document.activeElement).toBe(middle);

    const evBack = dispatchTab(container, true);
    expect(evBack.defaultPrevented).toBe(false);
    expect(document.activeElement).toBe(middle);
  });

  it("is a true no-op when enabled=false (no listener wired)", () => {
    const { container, buttons, ref } = setup(3);
    const hook = renderHook(({ enabled }) => useFocusTrap(ref, enabled), {
      initialProps: { enabled: false },
    });
    cleanup = hook.unmount;

    const last = buttons[2] as HTMLButtonElement;
    last.focus();
    expect(document.activeElement).toBe(last);

    const ev = dispatchTab(container);
    // The hook is disabled — the keydown event must propagate untouched and
    // focus must NOT be re-routed to the first button.
    expect(ev.defaultPrevented).toBe(false);
    expect(document.activeElement).toBe(last);
  });

  it("with zero tabbables only preventDefaults (does not throw, does not focus anything)", () => {
    const container = document.createElement("div");
    document.body.appendChild(container);
    const ref = createRef<HTMLElement>();
    (ref as { current: HTMLElement | null }).current = container;

    const before = document.activeElement;
    const hook = renderHook(({ enabled }) => useFocusTrap(ref, enabled), {
      initialProps: { enabled: true },
    });
    cleanup = hook.unmount;

    const ev = dispatchTab(container);
    expect(ev.defaultPrevented).toBe(true);
    // No tabbables → no focus manipulation; whatever was active before stays.
    expect(document.activeElement).toBe(before);
  });

  it("ignores non-Tab keys (does not preventDefault)", () => {
    const { container, buttons, ref } = setup(3);
    const hook = renderHook(({ enabled }) => useFocusTrap(ref, enabled), {
      initialProps: { enabled: true },
    });
    cleanup = hook.unmount;

    const last = buttons[2] as HTMLButtonElement;
    last.focus();

    const ev = new KeyboardEvent("keydown", {
      key: "Escape",
      bubbles: true,
      cancelable: true,
    });
    container.dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(false);
    expect(document.activeElement).toBe(last);
  });

  it("removes its listener on unmount (subsequent Tab no longer cycles)", () => {
    const { container, buttons, ref } = setup(3);
    const hook = renderHook(({ enabled }) => useFocusTrap(ref, enabled), {
      initialProps: { enabled: true },
    });

    const last = buttons[2] as HTMLButtonElement;
    last.focus();

    hook.unmount();

    const ev = dispatchTab(container);
    expect(ev.defaultPrevented).toBe(false);
    expect(document.activeElement).toBe(last);
  });
});
