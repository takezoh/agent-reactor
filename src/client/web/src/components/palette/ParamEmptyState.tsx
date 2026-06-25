// ParamEmptyState — generic empty-state body for ParamSelectPhase.
//
// Spec: docs/specs/2026-06-24-web-ui-fixes (palette-bugfix)
// Role: presentational only. Receives a message string and renders it as a
// polite live region (role="status" carries implicit aria-live="polite", so
// we don't repeat the attribute). The component is intentionally generic —
// the caller (ParamSelectPhase) owns the message wording (e.g. "No projects
// available - add a project first" vs a future "No commands available").
//
// What this component deliberately does NOT do:
//   - No key handlers (onKeyDown/onKeyUp/onKeyPress). Keystrokes (Enter,
//     Tab, Esc, arrows) must bubble unchanged so the palette shell's focus
//     trap / Esc-back / submit gating keeps working. The acceptance test
//     pins this by asserting event.defaultPrevented === false after a
//     keyDown Enter through the rendered node.
//   - No tabIndex. Empty-state is not a focus target; focus stays on the
//     palette shell (combobox input or fieldset) per FR-016.
//   - No i18n / transform. The message string is rendered verbatim so the
//     caller has full control over wording (and tests can pin exact text).

import type { JSX } from "react";

export interface ParamEmptyStateProps {
  message: string;
}

export function ParamEmptyState({ message }: ParamEmptyStateProps): JSX.Element {
  // className mirrors the existing palette convention (see app.css —
  // `.palette-param-empty` is given the same subdued treatment as
  // `.palette-progress`: font-size 0.85em / opacity 0.8). A dedicated
  // class (rather than reusing `.palette-progress`) lets the empty-state
  // styling diverge later without touching call sites or the progress
  // live region.
  return (
    <div
      // biome-ignore lint/a11y/useSemanticElements: the task contract pins role="status" on a generic <div>. The component's only prop is `message` — there is no form/output value being produced, so an <output> element (which implies a form association via its `for`/`form` semantics) would mis-describe the surface. Keeping role="status" on a <div> is intentional and the test pins it via getByRole('status').
      role="status"
      className="palette-param-empty"
    >
      {message}
    </div>
  );
}
