// useAnnounceMessage — builds announce messages for InlineStatus from announceSeq.
//
// FR-010: announce on active session change.
// FR-033: announce suppressed while submitting.
// ADR-0057: CommandPalette flows active-context announce through InlineStatus props.

import { useRef } from "react";
import type { ActiveContextSnapshot } from "../../../store/palette_active_context";

/**
 * Returns a ref holding the latest announce message (or undefined).
 * Updates synchronously during render when announceSeq increments and
 * submitting is false (FR-033 suppress during submit).
 */
export function useAnnounceMessage(
  announceSeq: number,
  submitting: boolean,
  activeContextSnapshot: ActiveContextSnapshot,
): React.MutableRefObject<{ message: string; seq: number } | undefined> {
  const announceRef = useRef<{ message: string; seq: number } | undefined>(undefined);
  const prevAnnounceSeqRef = useRef(announceSeq);

  if (prevAnnounceSeqRef.current !== announceSeq && !submitting) {
    prevAnnounceSeqRef.current = announceSeq;
    const snap = activeContextSnapshot;
    let message: string;
    if (snap.kind === "none") {
      message = "Active session cleared";
    } else if (snap.kind === "unknown") {
      message = `Active session changed to unknown / ${snap.sid8}`;
    } else {
      message = `Active session changed to ${snap.projBase} / ${snap.sid8}`;
    }
    announceRef.current = { message, seq: announceSeq };
  }

  return announceRef;
}
