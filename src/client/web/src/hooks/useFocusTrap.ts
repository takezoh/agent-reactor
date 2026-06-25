import { useEffect } from "react";
import type { RefObject } from "react";

/**
 * useFocusTrap — minimal Tab / Shift+Tab cycle inside a container ref.
 *
 * Responsibilities (ADR-0039: "palette-focus-trap-minimal"):
 *   - Cycle focus between the first and last tabbable descendants of `ref`.
 *   - Do *not* handle Escape or restore focus to the opener — those belong to
 *     the palette store / CommandPalette unmount.
 *   - Do *not* attach listeners to `document`; only the ref'd element.
 *
 * Disabled state (`enabled === false`) is a true no-op: no listener is wired,
 * so mount / unmount cost stays at zero when the dialog is closed.
 */
const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

export function useFocusTrap(ref: RefObject<HTMLElement>, enabled: boolean): void {
  useEffect(() => {
    if (!enabled) return;
    const root = ref.current;
    if (!root) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Tab") return;
      const focusables = Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
        (el) => !el.hasAttribute("inert") && el.offsetParent !== null,
      );
      if (focusables.length === 0) {
        e.preventDefault();
        return;
      }
      const first = focusables[0] as HTMLElement;
      const last = focusables[focusables.length - 1] as HTMLElement;
      const active = document.activeElement as HTMLElement | null;
      if (e.shiftKey && active === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && active === last) {
        e.preventDefault();
        first.focus();
      }
    };
    root.addEventListener("keydown", onKey);
    return () => root.removeEventListener("keydown", onKey);
  }, [ref, enabled]);
}
