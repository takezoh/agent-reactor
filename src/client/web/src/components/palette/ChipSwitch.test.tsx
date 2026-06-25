// ChipSwitch.test.tsx — UAC-008 / UAC-009 / UAC-010 / FR-016 / FR-017 / FR-019 / FR-020 / FR-023 / FR-029
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ChipSwitch } from "./ChipSwitch";

function renderChip(overrides: Partial<Parameters<typeof ChipSwitch>[0]> = {}) {
  const onToggle = vi.fn();
  const defaults = {
    hintKey: "W" as const,
    label: "Worktree",
    checked: false,
    onToggle,
    composing: false,
    testId: "worktree",
  };
  const props = { ...defaults, ...overrides, onToggle: overrides.onToggle ?? onToggle };
  render(<ChipSwitch {...props} />);
  const button = screen.getByRole("switch");
  return { button, onToggle: props.onToggle };
}

describe("ChipSwitch", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // UAC-009 / FR-016 — renders role='switch' + aria-checked + hint + state text
  it("renders with aria-checked=false and data-on=off when checked=false (FR-016 / UAC-009)", () => {
    const { button } = renderChip({ checked: false });
    expect(button).not.toBeNull();
    expect(button.getAttribute("role")).toBe("switch");
    expect(button.getAttribute("aria-checked")).toBe("false");
    expect(button.getAttribute("data-on")).toBe("off");
    expect(button.textContent).toContain("OFF");
  });

  // FR-029 / WCAG 1.4.1 — state conveyed by text, not only color
  it("shows ON text when checked=true (FR-029 / WCAG 1.4.1)", () => {
    const { button } = renderChip({ checked: true });
    expect(button.getAttribute("aria-checked")).toBe("true");
    expect(button.getAttribute("data-on")).toBe("on");
    expect(button.textContent).toContain("ON");
  });

  // UAC-009 / FR-016 — hintKey='W' shows '[W]' icon hint
  it("shows [W] hint when hintKey=W (FR-016 / UAC-009)", () => {
    const { button } = renderChip({ hintKey: "W" });
    expect(button.textContent).toContain("[W]");
  });

  // UAC-009 / FR-016 — hintKey='H' shows '[H]' icon hint
  it("shows [H] hint when hintKey=H (FR-016 / UAC-009)", () => {
    const { button } = renderChip({ hintKey: "H", label: "Host (sandbox)" });
    expect(button.textContent).toContain("[H]");
  });

  // UAC-008 / FR-017 — pointerdown calls onToggle and calls preventDefault
  it("pointerdown fires onToggle and calls preventDefault (FR-017 / UAC-008)", () => {
    const { button, onToggle } = renderChip();
    // fireEvent.pointerDown returns false when preventDefault was called
    const notCanceled = fireEvent.pointerDown(button);
    expect(onToggle).toHaveBeenCalledTimes(1);
    expect(notCanceled).toBe(false); // false = preventDefault was called
  });

  // UAC-010 / FR-019 — Space key triggers onToggle + preventDefault
  it("Space on focused chip calls onToggle and preventDefault (FR-019 / UAC-010)", () => {
    const { button, onToggle } = renderChip();
    const notCanceled = fireEvent.keyDown(button, { key: " " });
    expect(onToggle).toHaveBeenCalledTimes(1);
    expect(notCanceled).toBe(false); // false = preventDefault was called
  });

  // UAC-010 / FR-020 — Enter key triggers onToggle + preventDefault (no form submit)
  it("Enter on focused chip calls onToggle and preventDefault (FR-020 / UAC-010)", () => {
    const { button, onToggle } = renderChip();
    const notCanceled = fireEvent.keyDown(button, { key: "Enter" });
    expect(onToggle).toHaveBeenCalledTimes(1);
    expect(notCanceled).toBe(false); // false = preventDefault was called
  });

  // FR-023 — composing=true blocks pointerdown
  it("pointerdown is no-op when composing=true (FR-023)", () => {
    const { button, onToggle } = renderChip({ composing: true });
    fireEvent.pointerDown(button);
    expect(onToggle).not.toHaveBeenCalled();
  });

  // FR-023 — composing=true blocks Space
  it("Space is no-op when composing=true (FR-023)", () => {
    const { button, onToggle } = renderChip({ composing: true });
    fireEvent.keyDown(button, { key: " " });
    expect(onToggle).not.toHaveBeenCalled();
  });

  // FR-023 — composing=true blocks Enter
  it("Enter is no-op when composing=true (FR-023)", () => {
    const { button, onToggle } = renderChip({ composing: true });
    fireEvent.keyDown(button, { key: "Enter" });
    expect(onToggle).not.toHaveBeenCalled();
  });

  // disabled=true blocks pointerdown
  it("pointerdown is no-op when disabled=true", () => {
    const { button, onToggle } = renderChip({ disabled: true });
    fireEvent.pointerDown(button);
    expect(onToggle).not.toHaveBeenCalled();
  });

  // disabled=true blocks Space
  it("Space is no-op when disabled=true", () => {
    const { button, onToggle } = renderChip({ disabled: true });
    fireEvent.keyDown(button, { key: " " });
    expect(onToggle).not.toHaveBeenCalled();
  });

  // disabled=true blocks Enter
  it("Enter is no-op when disabled=true", () => {
    const { button, onToggle } = renderChip({ disabled: true });
    fireEvent.keyDown(button, { key: "Enter" });
    expect(onToggle).not.toHaveBeenCalled();
  });

  // testId is forwarded as data-toggle attribute
  it("data-toggle attribute is set from testId prop", () => {
    const { button } = renderChip({ testId: "host" });
    expect(button.getAttribute("data-toggle")).toBe("host");
  });
});
