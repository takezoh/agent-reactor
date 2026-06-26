// UnifiedListbox.test.tsx
//
// FR-TOKEN-002 / FR-PALETTE-NAV-001 / FR-PALETTE-IME-001 / FR-A11Y-001
// UAC-009 / UAC-011
//
// Tests for the UnifiedListbox primitive: disabled skip-navigation,
// aria-activedescendant sync, IME suppression, and DOM visibility of
// disabled rows.

import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { UnifiedListbox, type UnifiedListboxItem } from "./UnifiedListbox";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

/** 5 items: item1(enabled), item2(disabled), item3(enabled), item4(disabled), item5(enabled). */
const FIVE_ITEMS: Array<UnifiedListboxItem> = [
  { id: "item1", label: "Item 1" },
  { id: "item2", label: "Item 2", disabled: true, disabledReason: "Not available" },
  { id: "item3", label: "Item 3" },
  { id: "item4", label: "Item 4", disabled: true, disabledReason: "Requires session" },
  { id: "item5", label: "Item 5" },
];

// ---------------------------------------------------------------------------
// Render helper
// ---------------------------------------------------------------------------

interface RenderArgs {
  items?: Array<UnifiedListboxItem>;
  activeId?: string | null;
  onActiveChange?: (id: string) => void;
  onActivate?: (id: string) => void;
  onCompositionChange?: (composing: boolean) => void;
}

function renderListbox({
  items = FIVE_ITEMS,
  activeId = "item1",
  onActiveChange = vi.fn(),
  onActivate = vi.fn(),
  onCompositionChange,
}: RenderArgs = {}) {
  const onActiveChangeFn = onActiveChange;
  const onActivateFn = onActivate;
  const utils = render(
    <UnifiedListbox
      ariaLabel="Test listbox"
      items={items}
      activeId={activeId}
      onActiveChange={onActiveChangeFn}
      onActivate={onActivateFn}
      onCompositionChange={onCompositionChange}
    />,
  );
  const listbox = screen.getByRole("listbox");
  const options = () => Array.from(screen.queryAllByRole("option")) as HTMLElement[];
  return { listbox, options, ...utils };
}

// ---------------------------------------------------------------------------
// Setup / teardown
// ---------------------------------------------------------------------------

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// FR-TOKEN-002 / UAC-009: disabled rows visible, aria-activedescendant only
// points to enabled rows
// ---------------------------------------------------------------------------

describe("UnifiedListbox — FR-TOKEN-002: disabled row visibility", () => {
  it("renders all 5 items including disabled ones in the DOM", () => {
    const { options } = renderListbox();
    expect(options()).toHaveLength(5);
  });

  it("disabled rows are DOM-visible (not display:none)", () => {
    const { options } = renderListbox();
    const disabledOpts = options().filter((o) => o.getAttribute("aria-disabled") === "true");
    expect(disabledOpts).toHaveLength(2);
    for (const opt of disabledOpts) {
      // happy-dom returns '' for display when not explicitly set; check it's not 'none'.
      const computed = getComputedStyle(opt).display;
      expect(computed).not.toBe("none");
    }
  });

  it("disabled rows carry aria-disabled='true' (FR-TOKEN-002)", () => {
    const { options } = renderListbox();
    const item2 = options().find((o) => o.id === "item2");
    const item4 = options().find((o) => o.id === "item4");
    expect(item2?.getAttribute("aria-disabled")).toBe("true");
    expect(item4?.getAttribute("aria-disabled")).toBe("true");
  });

  it("enabled rows do NOT carry aria-disabled (FR-TOKEN-002)", () => {
    const { options } = renderListbox();
    const enabled = options().filter((o) => o.getAttribute("aria-disabled") !== "true");
    expect(enabled).toHaveLength(3);
    for (const opt of enabled) {
      expect(opt.getAttribute("aria-disabled")).toBeNull();
    }
  });

  it("disabled rows contain reason text node child (UAC-009 / UAC-011)", () => {
    const { options } = renderListbox();
    const item2 = options().find((o) => o.id === "item2");
    const item4 = options().find((o) => o.id === "item4");
    const reason2 = item2?.querySelector(".unified-listbox__option-reason");
    const reason4 = item4?.querySelector(".unified-listbox__option-reason");
    expect(reason2).not.toBeNull();
    expect(reason2?.textContent).toBe("Not available");
    expect(reason4).not.toBeNull();
    expect(reason4?.textContent).toBe("Requires session");
  });

  it("aria-activedescendant equals activeId (enabled item only)", () => {
    const { listbox } = renderListbox({ activeId: "item1" });
    expect(listbox.getAttribute("aria-activedescendant")).toBe("item1");
  });

  it("aria-activedescendant does not point to disabled item when activeId is null", () => {
    const { listbox } = renderListbox({ activeId: null });
    expect(listbox.getAttribute("aria-activedescendant")).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// FR-PALETTE-NAV-001: ArrowDown / ArrowUp / Ctrl-P / Ctrl-N skip disabled
// ---------------------------------------------------------------------------

describe("UnifiedListbox — FR-PALETTE-NAV-001: keyboard disabled skip", () => {
  it("ArrowDown from item1 skips item2(disabled) and lands on item3", () => {
    const onActiveChange = vi.fn();
    const { listbox } = renderListbox({ activeId: "item1", onActiveChange });

    fireEvent.keyDown(listbox, { key: "ArrowDown" });

    expect(onActiveChange).toHaveBeenCalledTimes(1);
    expect(onActiveChange).toHaveBeenCalledWith("item3");
  });

  it("ArrowDown from item3 skips item4(disabled) and lands on item5", () => {
    const onActiveChange = vi.fn();
    const { listbox } = renderListbox({ activeId: "item3", onActiveChange });

    fireEvent.keyDown(listbox, { key: "ArrowDown" });

    expect(onActiveChange).toHaveBeenCalledTimes(1);
    expect(onActiveChange).toHaveBeenCalledWith("item5");
  });

  it("ArrowDown from item5 (last enabled) stays at item5 (clamp-not-wrap)", () => {
    const onActiveChange = vi.fn();
    const { listbox } = renderListbox({ activeId: "item5", onActiveChange });

    fireEvent.keyDown(listbox, { key: "ArrowDown" });

    // No further enabled item — should call with same last enabled item (item5).
    expect(onActiveChange).toHaveBeenCalledWith("item5");
  });

  it("ArrowUp from item5 skips item4(disabled) and lands on item3", () => {
    const onActiveChange = vi.fn();
    const { listbox } = renderListbox({ activeId: "item5", onActiveChange });

    fireEvent.keyDown(listbox, { key: "ArrowUp" });

    expect(onActiveChange).toHaveBeenCalledTimes(1);
    expect(onActiveChange).toHaveBeenCalledWith("item3");
  });

  it("ArrowUp from item3 skips item2(disabled) and lands on item1", () => {
    const onActiveChange = vi.fn();
    const { listbox } = renderListbox({ activeId: "item3", onActiveChange });

    fireEvent.keyDown(listbox, { key: "ArrowUp" });

    expect(onActiveChange).toHaveBeenCalledTimes(1);
    expect(onActiveChange).toHaveBeenCalledWith("item1");
  });

  it("ArrowUp from item1 (first enabled) stays at item1 (clamp-not-wrap)", () => {
    const onActiveChange = vi.fn();
    const { listbox } = renderListbox({ activeId: "item1", onActiveChange });

    fireEvent.keyDown(listbox, { key: "ArrowUp" });

    // Falls back to first enabled item, which is still item1.
    expect(onActiveChange).toHaveBeenCalledWith("item1");
  });

  it("Ctrl+N from item1 skips item2 and lands on item3", () => {
    const onActiveChange = vi.fn();
    const { listbox } = renderListbox({ activeId: "item1", onActiveChange });

    fireEvent.keyDown(listbox, { key: "n", ctrlKey: true });

    expect(onActiveChange).toHaveBeenCalledWith("item3");
  });

  it("Ctrl+N from item3 skips item4 and lands on item5", () => {
    const onActiveChange = vi.fn();
    const { listbox } = renderListbox({ activeId: "item3", onActiveChange });

    fireEvent.keyDown(listbox, { key: "n", ctrlKey: true });

    expect(onActiveChange).toHaveBeenCalledWith("item5");
  });

  it("Ctrl+P from item5 skips item4 and lands on item3", () => {
    const onActiveChange = vi.fn();
    const { listbox } = renderListbox({ activeId: "item5", onActiveChange });

    fireEvent.keyDown(listbox, { key: "p", ctrlKey: true });

    expect(onActiveChange).toHaveBeenCalledWith("item3");
  });

  it("Ctrl+P from item3 skips item2 and lands on item1", () => {
    const onActiveChange = vi.fn();
    const { listbox } = renderListbox({ activeId: "item3", onActiveChange });

    fireEvent.keyDown(listbox, { key: "p", ctrlKey: true });

    expect(onActiveChange).toHaveBeenCalledWith("item1");
  });
});

// ---------------------------------------------------------------------------
// Enter key calls onActivate
// ---------------------------------------------------------------------------

describe("UnifiedListbox — Enter activates active item", () => {
  it("Enter calls onActivate with activeId when activeId points to enabled item", () => {
    const onActivate = vi.fn();
    const { listbox } = renderListbox({ activeId: "item1", onActivate });

    fireEvent.keyDown(listbox, { key: "Enter" });

    expect(onActivate).toHaveBeenCalledTimes(1);
    expect(onActivate).toHaveBeenCalledWith("item1");
  });

  it("Enter does NOT call onActivate when activeId points to disabled item", () => {
    const onActivate = vi.fn();
    const { listbox } = renderListbox({ activeId: "item2", onActivate });

    fireEvent.keyDown(listbox, { key: "Enter" });

    expect(onActivate).not.toHaveBeenCalled();
  });

  it("Enter does nothing when activeId is null", () => {
    const onActivate = vi.fn();
    const { listbox } = renderListbox({ activeId: null, onActivate });

    fireEvent.keyDown(listbox, { key: "Enter" });

    expect(onActivate).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// FR-PALETTE-IME-001: IME composition suppresses Enter
// ---------------------------------------------------------------------------

describe("UnifiedListbox — FR-PALETTE-IME-001: IME suppression", () => {
  it("Enter during compositionstart does NOT call onActivate", () => {
    const onActivate = vi.fn();
    const onCompositionChange = vi.fn();
    const { listbox } = renderListbox({ activeId: "item1", onActivate, onCompositionChange });

    fireEvent.compositionStart(listbox);
    expect(onCompositionChange).toHaveBeenCalledWith(true);

    fireEvent.keyDown(listbox, { key: "Enter" });

    expect(onActivate).not.toHaveBeenCalled();
  });

  it("ArrowDown during composition does NOT trigger onActiveChange", () => {
    const onActiveChange = vi.fn();
    const { listbox } = renderListbox({ activeId: "item1", onActiveChange });

    fireEvent.compositionStart(listbox);
    fireEvent.keyDown(listbox, { key: "ArrowDown" });

    expect(onActiveChange).not.toHaveBeenCalled();
  });

  it("Enter after compositionend calls onActivate (IME resolved)", () => {
    const onActivate = vi.fn();
    const onCompositionChange = vi.fn();
    const { listbox } = renderListbox({ activeId: "item1", onActivate, onCompositionChange });

    fireEvent.compositionStart(listbox);
    fireEvent.compositionEnd(listbox);
    expect(onCompositionChange).toHaveBeenLastCalledWith(false);

    fireEvent.keyDown(listbox, { key: "Enter" });

    expect(onActivate).toHaveBeenCalledTimes(1);
    expect(onActivate).toHaveBeenCalledWith("item1");
  });

  it("onCompositionChange is called with true on compositionstart and false on compositionend", () => {
    const onCompositionChange = vi.fn();
    const { listbox } = renderListbox({ onCompositionChange });

    fireEvent.compositionStart(listbox);
    expect(onCompositionChange).toHaveBeenCalledWith(true);

    fireEvent.compositionEnd(listbox);
    expect(onCompositionChange).toHaveBeenCalledWith(false);
  });
});

// ---------------------------------------------------------------------------
// ARIA sync: aria-activedescendant / aria-selected
// ---------------------------------------------------------------------------

describe("UnifiedListbox — ARIA sync", () => {
  it("aria-activedescendant on listbox matches activeId", () => {
    const { listbox } = renderListbox({ activeId: "item3" });
    expect(listbox.getAttribute("aria-activedescendant")).toBe("item3");
  });

  it("each option's aria-selected is true only for activeId", () => {
    const { options } = renderListbox({ activeId: "item3" });
    for (const opt of options()) {
      const expected = opt.id === "item3" ? "true" : "false";
      expect(opt.getAttribute("aria-selected")).toBe(expected);
    }
  });

  it("option id attribute matches item.id", () => {
    const { options } = renderListbox();
    const ids = options().map((o) => o.id);
    expect(ids).toEqual(["item1", "item2", "item3", "item4", "item5"]);
  });
});

// ---------------------------------------------------------------------------
// CSS class / token structure
// ---------------------------------------------------------------------------

describe("UnifiedListbox — CSS token classes", () => {
  it("root element has class 'unified-listbox'", () => {
    const { listbox } = renderListbox();
    expect(listbox.classList.contains("unified-listbox")).toBe(true);
  });

  it("each option has class 'unified-listbox__option'", () => {
    const { options } = renderListbox();
    for (const opt of options()) {
      expect(opt.classList.contains("unified-listbox__option")).toBe(true);
    }
  });

  it("disabled options also have class 'unified-listbox__option--disabled'", () => {
    const { options } = renderListbox();
    const disabled = options().filter((o) => o.getAttribute("aria-disabled") === "true");
    expect(disabled).toHaveLength(2);
    for (const opt of disabled) {
      expect(opt.classList.contains("unified-listbox__option--disabled")).toBe(true);
    }
  });

  it("enabled options do NOT have 'unified-listbox__option--disabled'", () => {
    const { options } = renderListbox();
    const enabled = options().filter((o) => o.getAttribute("aria-disabled") !== "true");
    for (const opt of enabled) {
      expect(opt.classList.contains("unified-listbox__option--disabled")).toBe(false);
    }
  });

  // m1 / FR-A11Y-001: palette listbox option must meet WCAG 2.5.5 (44x44px).
  // happy-dom does not resolve --row-min-height / token cascades, so we read
  // app.css and assert the CSS contract directly:
  //   - .unified-listbox__option has min-height: var(--row-min-height)
  //   - tokens.css sets --row-min-height >= 44px when scoped to the touch
  //     target context (.session-list .unified-listbox__option: 44px floor).
  // We assert both rules so the listbox sidebar (44px) and the palette
  // listbox (--row-min-height fallback) both stay observable.
  it("FR-A11Y-001 (m1): listbox option CSS contract — 44x44 minimum target", () => {
    const appCssPath = resolve(__dirname, "..", "..", "css", "app.css");
    const appCss = readFileSync(appCssPath, "utf-8");
    // Base .unified-listbox__option declaration uses --row-min-height var
    // (still owned by app.css).
    const baseMatch = appCss.match(/\.unified-listbox__option \{([^}]+)\}/);
    expect(baseMatch).not.toBeNull();
    expect(baseMatch?.[1] ?? "").toMatch(/min-height:\s*var\(--row-min-height\)/);
    // SessionList override locks to 44px (moved to session-list.css).
    const slCssPath = resolve(__dirname, "..", "..", "css", "session-list.css");
    const slCss = readFileSync(slCssPath, "utf-8");
    expect(slCss).toMatch(/\.session-list \.unified-listbox__option \{[^}]*min-height:\s*44px/);
  });
});
