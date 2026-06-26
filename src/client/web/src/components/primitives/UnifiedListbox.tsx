// UnifiedListbox — generic accessible listbox primitive.
//
// Extracts the a11y patterns that were refined in the command palette
// (ToolSelectPhase / ParamListbox) into a standalone, reusable component.
//
// Spec: docs/specs/2026-06-25-web-ui-redesign
// FR:
//   - FR-TOKEN-002 (disabled rows visible; aria-activedescendant only points
//     to enabled rows)
//   - FR-PALETTE-NAV-001 (ArrowDown/Up/Ctrl-N/P skip disabled rows)
//   - FR-PALETTE-IME-001 (compositionstart/end suppress Enter)
//   - FR-A11Y-001 (44px / 2rem minimum row height)
//   - UAC-009 / UAC-011 (disabled rows stay DOM-visible with reason text)
//
// Token classes applied to each option row use the --row-* CSS custom
// properties from tokens.css (m1-tokens-css).
//
// NOTE: palette internal state machines (ADR-0036/0039/0050/0055/0057) are
// NOT modified by this file. The palette components remain unchanged; this
// primitive is consumed by SessionList (m5-session-list-listbox).

import { type KeyboardEvent, type ReactNode, useId, useRef, useState } from "react";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface UnifiedListboxItem {
  id: string;
  label: ReactNode;
  disabled?: boolean;
  disabledReason?: string;
}

export interface UnifiedListboxProps {
  ariaLabel: string;
  items: Array<UnifiedListboxItem>;
  activeId: string | null;
  onActiveChange: (id: string) => void;
  onActivate: (id: string) => void;
  onCompositionChange?: (composing: boolean) => void;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Returns the index of item with `id` in `items`, or -1. */
function indexById(items: Array<UnifiedListboxItem>, id: string | null): number {
  if (id === null) return -1;
  return items.findIndex((item) => item.id === id);
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function UnifiedListbox(props: UnifiedListboxProps) {
  const { ariaLabel, items, activeId, onActiveChange, onActivate, onCompositionChange } = props;

  // Per-instance DOM-id prefix. Without this, mounting the same listbox twice
  // in one document (e.g. SessionList in the desktop sidebar AND inside
  // SessionDrawer at the same time on a narrow viewport) emits duplicate
  // element ids — browsers then break `aria-activedescendant` lookup and the
  // first hidden copy can swallow taps targeted at the visible one.
  const idScope = useId();
  const optionDomId = (itemId: string): string => `${idScope}-${itemId}`;

  // IME composition state — local; mirrored to caller via onCompositionChange.
  const [isComposing, setIsComposing] = useState(false);

  // Keep a ref for use inside the keydown handler without stale-closure issues.
  const isComposingRef = useRef(false);

  function setComposing(v: boolean) {
    isComposingRef.current = v;
    setIsComposing(v);
    onCompositionChange?.(v);
  }

  // Ordered list of enabled item ids — used for skip navigation.
  const enabledIds = items.filter((item) => !item.disabled).map((item) => item.id);

  function onKeyDown(e: KeyboardEvent<HTMLDivElement>) {
    // FR-PALETTE-IME-001: while IME is composing, suppress navigation/commit.
    if (isComposingRef.current) return;

    const k = e.key;
    const lower = k.toLowerCase();

    // ArrowDown / Ctrl+N — move to next enabled item (skip disabled).
    if (k === "ArrowDown" || (e.ctrlKey && lower === "n")) {
      e.preventDefault();
      const currentIdx = indexById(items, activeId);
      // Find the first enabled item after the current position.
      const nextId =
        enabledIds.find((id) => {
          const idx = indexById(items, id);
          return idx > currentIdx;
        }) ?? enabledIds[enabledIds.length - 1];
      if (nextId !== undefined) onActiveChange(nextId);
      return;
    }

    // ArrowUp / Ctrl+P — move to previous enabled item (skip disabled).
    if (k === "ArrowUp" || (e.ctrlKey && lower === "p")) {
      e.preventDefault();
      const currentIdx = indexById(items, activeId);
      // Walk backward through enabledIds to find the nearest entry before cursor.
      let prevId: string | undefined;
      for (let i = enabledIds.length - 1; i >= 0; i--) {
        const id = enabledIds[i];
        if (id !== undefined) {
          const idx = indexById(items, id);
          if (idx < currentIdx) {
            prevId = id;
            break;
          }
        }
      }
      if (prevId === undefined) prevId = enabledIds[0];
      if (prevId !== undefined) onActiveChange(prevId);
      return;
    }

    // Enter / Space — activate the currently active item.
    if (k === "Enter" || k === " ") {
      e.preventDefault();
      if (activeId !== null) {
        const activeItem = items.find((item) => item.id === activeId);
        if (activeItem !== undefined && !activeItem.disabled) {
          onActivate(activeId);
        }
      }
      return;
    }
  }

  return (
    // biome-ignore lint/a11y/useSemanticElements: ARIA listbox pattern requires div+role=listbox; <select> cannot host rich ReactNode labels or custom keyboard navigation
    <div
      role="listbox"
      aria-label={ariaLabel}
      aria-activedescendant={activeId !== null ? optionDomId(activeId) : undefined}
      tabIndex={0}
      className="unified-listbox"
      onKeyDown={onKeyDown}
      onCompositionStart={() => setComposing(true)}
      onCompositionEnd={() => setComposing(false)}
      data-composing={isComposing ? "true" : undefined}
    >
      {items.map((item) => {
        const isActive = item.id === activeId;
        return (
          <div
            key={item.id}
            id={optionDomId(item.id)}
            data-item-id={item.id}
            // biome-ignore lint/a11y/useSemanticElements: ARIA listbox uses div+role=option; <option> only works inside <select>
            // biome-ignore lint/a11y/useFocusableInteractive: focus stays on parent listbox via aria-activedescendant; options are not individually tabbable
            role="option"
            tabIndex={-1}
            aria-selected={isActive}
            aria-disabled={item.disabled ? "true" : undefined}
            className={[
              "unified-listbox__option",
              item.disabled ? "unified-listbox__option--disabled" : "",
            ]
              .filter(Boolean)
              .join(" ")}
            onPointerDown={(e) => {
              // Prevent focus leaving the listbox on click.
              e.preventDefault();
              if (!item.disabled) {
                // Move cursor (preview) then commit (activate) in one gesture.
                onActiveChange(item.id);
                onActivate(item.id);
              }
            }}
          >
            {item.label}
            {item.disabled && item.disabledReason !== undefined && (
              <span className="unified-listbox__option-reason">{item.disabledReason}</span>
            )}
          </div>
        );
      })}
    </div>
  );
}
