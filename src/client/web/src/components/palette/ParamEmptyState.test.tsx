import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ParamEmptyState } from "./ParamEmptyState";

describe("ParamEmptyState", () => {
  it("renders the message inside a role='status' live region", () => {
    render(<ParamEmptyState message="No projects available - add a project first" />);
    const status = screen.getByRole("status");
    expect(status.textContent).toBe("No projects available - add a project first");
  });

  it("renders the message string verbatim (no i18n / transform)", () => {
    // Pin transparent props: whatever the caller passes is what the DOM
    // receives. This guards against a future regression where someone adds
    // a hidden prefix / suffix / wrapper-text inside the component.
    const message = "Custom!! 文字列 — 123";
    render(<ParamEmptyState message={message} />);
    expect(screen.getByRole("status").textContent).toBe(message);
  });

  it("does not swallow Enter keydown (no preventDefault, no key handler)", () => {
    // The component must not register onKeyDown/onKeyUp — the palette shell
    // owns Enter / Esc / Tab semantics. fireEvent.keyDown returns true when
    // no listener called preventDefault, and the dispatched event's
    // defaultPrevented stays false. We assert both signals so a future
    // accidental e.preventDefault() inside the component is caught.
    render(<ParamEmptyState message="empty" />);
    const status = screen.getByRole("status");
    const event = new KeyboardEvent("keydown", {
      key: "Enter",
      bubbles: true,
      cancelable: true,
    });
    const notPrevented = status.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(false);
    expect(notPrevented).toBe(true);
  });

  it("is not in the tab order (no tabIndex)", () => {
    // Empty-state is presentational; focus must stay on the palette shell.
    // Asserting the attribute is absent (rather than checking a numeric
    // value) catches both `tabIndex={0}` and `tabIndex={-1}` regressions —
    // either would change the focus contract the parent relies on.
    render(<ParamEmptyState message="empty" />);
    const status = screen.getByRole("status");
    expect(status.hasAttribute("tabindex")).toBe(false);
  });
});
