/**
 * NotificationToast — FR-TOAST-001 / FR-TOAST-002 / FR-TOAST-003 / ADR-0063
 *
 * Responsibilities:
 *  - Single aria-live='polite' role='status' on the container (not per-item).
 *  - Each toast item uses CSS class for type-based bg color (no inline style).
 *  - Mobile (<768px): position:fixed, bottom-aligned with safe-area inset.
 *  - Desktop (>=768px): position:fixed, top-right.
 *  - Max 3 visible items; auto-dismiss in 5s; tap to dismiss.
 *  - notification-toast__undosnackbar-slot: reserved for UndoSnackbar (FR-TOAST-003).
 *  - Per-level leading glyph (✓ / ! / × / i) rendered inside an aria-hidden
 *    slot — purely visual, screen readers still read the message text.
 */

import { useEffect } from "react";
import type { ReactNode } from "react";
import type { Notification } from "../store/notifications";
import { useNotificationsStore } from "../store/notifications";

// ─── types ─────────────────────────────────────────────────────────────────────

type ToastItemProps = {
  item: Notification;
  onDismiss: (id: number) => void;
};

// Per-level glyph. Plain text glyphs keep the bundle SVG-free and inherit
// font color, which lets the CSS layer style them per level without a JS
// branch. Visual-only — the parent text content carries the SR-readable
// message, so the glyph slot is aria-hidden.
const LEVEL_GLYPH: Record<Notification["level"], string> = {
  info: "i",
  warn: "!",
  error: "×",
};

// ─── ToastItem ─────────────────────────────────────────────────────────────────

function ToastItem({ item, onDismiss }: ToastItemProps): JSX.Element {
  useEffect(() => {
    const t = setTimeout(() => onDismiss(item.id), 5000);
    return () => clearTimeout(t);
  }, [item.id, onDismiss]);

  const typeClass = `notification-toast__item--${item.level}`;
  const glyph = LEVEL_GLYPH[item.level];

  return (
    // biome-ignore lint/a11y/useKeyWithClickEvents: toast is supplemental UI; keyboard users rely on auto-dismiss
    <div className={`notification-toast__item ${typeClass}`} onClick={() => onDismiss(item.id)}>
      <span className="notification-toast__item-icon" aria-hidden="true">
        {glyph}
      </span>
      <div className="notification-toast__item-content">
        {item.title != null && item.title !== "" ? (
          <>
            <div className="notification-toast__item-title">{item.title}</div>
            {item.body != null && item.body !== "" && (
              <div className="notification-toast__item-body">{item.body}</div>
            )}
          </>
        ) : (
          <div className="notification-toast__item-message">{item.message}</div>
        )}
      </div>
    </div>
  );
}

// ─── NotificationToast ─────────────────────────────────────────────────────────

export interface NotificationToastProps {
  /**
   * Additive (ADR 0063 non-breaking): when true the container is rendered as a
   * purely visual, screen-reader-silent surface — `aria-hidden=true`, no
   * `role=status` / `aria-live`, and only `children` are rendered (no passive
   * store items, no UndoSnackbar slot). PinchIndicator reuses this primitive in
   * this mode so the live fontSize is shown centre-screen without being
   * announced (a pinch is a continuous visual gesture, not a discrete event).
   * Default false preserves the existing notification behaviour exactly.
   */
  ariaHidden?: boolean;
  /** Visual content for the ariaHidden surface (e.g. the PinchIndicator readout). */
  children?: ReactNode;
}

/**
 * Renders a single aria-live container with up to 3 toast items.
 * The UndoSnackbar slot is a sibling within the same container for
 * 3-stream isolation (FR-TOAST-003): passive toasts | undo snackbar | palette.
 */
export function NotificationToast({
  ariaHidden = false,
  children,
}: NotificationToastProps = {}): JSX.Element {
  const items = useNotificationsStore((s) => s.items);
  const dismiss = useNotificationsStore((s) => s.dismiss);

  // Show only the latest 3 items
  const visible = items.slice(-3);

  // ariaHidden surface (PinchIndicator): visual-only, never announced. Renders
  // children alone so it cannot duplicate the passive toast list.
  if (ariaHidden) {
    return (
      <div className="notification-toast notification-toast--ariahidden" aria-hidden="true">
        {children}
      </div>
    );
  }

  return (
    // biome-ignore lint/a11y/useSemanticElements: spec requires explicit role='status' aria-live='polite' on a div container (FR-TOAST-001); <output> does not support child slot for UndoSnackbar
    <div className="notification-toast" aria-live="polite" role="status" aria-label="notifications">
      {/* Passive notification items — no individual aria-live (FR-TOAST-001) */}
      {visible.map((item) => (
        <ToastItem key={item.id} item={item} onDismiss={dismiss} />
      ))}

      {/* UndoSnackbar slot — independent aria-live region (FR-TOAST-003) */}
      {/* AppShell renders UndoSnackbar inside this slot via overlays prop */}
      <div className="notification-toast__undosnackbar-slot" />
    </div>
  );
}
