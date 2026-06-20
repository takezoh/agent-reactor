import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { LogTab } from "../wire/server";
import { LogTabSelector } from "./LogTabSelector";

function makeTab(label: string, path: string): LogTab {
  return { label, path, kind: "text" };
}

describe("LogTabSelector", () => {
  it("renders nothing when tabs is empty", () => {
    const { container } = render(<LogTabSelector tabs={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders each tab label and selects first by default", () => {
    const tabs = [
      makeTab("Alpha", "/logs/alpha"),
      makeTab("Beta", "/logs/beta"),
      makeTab("Gamma", "/logs/gamma"),
    ];
    render(<LogTabSelector tabs={tabs} />);
    const buttons = screen.getAllByRole("tab");
    expect(buttons).toHaveLength(3);
    expect(buttons[0]?.textContent).toBe("Alpha");
    expect(buttons[1]?.textContent).toBe("Beta");
    expect(buttons[2]?.textContent).toBe("Gamma");
    expect(buttons[0]?.getAttribute("aria-selected")).toBe("true");
    expect(buttons[1]?.getAttribute("aria-selected")).toBe("false");
    expect(buttons[2]?.getAttribute("aria-selected")).toBe("false");
  });

  it("switches selection on click", () => {
    const tabs = [makeTab("One", "/logs/one"), makeTab("Two", "/logs/two")];
    render(<LogTabSelector tabs={tabs} />);
    const buttons = screen.getAllByRole("tab");
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    const btn1 = buttons[1];
    if (!btn1) throw new Error("button 1 not found");
    fireEvent.click(btn1);
    expect(buttons[0]?.getAttribute("aria-selected")).toBe("false");
    expect(buttons[1]?.getAttribute("aria-selected")).toBe("true");
  });

  it("shows placeholder text referencing active tab path", () => {
    const tabs = [makeTab("Main", "/logs/main.log"), makeTab("Aux", "/logs/aux.log")];
    render(<LogTabSelector tabs={tabs} />);
    const panel = screen.getByRole("tabpanel");
    expect(panel.textContent).toContain("/logs/main.log");
  });
});
