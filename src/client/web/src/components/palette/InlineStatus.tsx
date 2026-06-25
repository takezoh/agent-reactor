// InlineStatus — single aria-live='polite' announce slot for the command palette.
//
// ADR-0057: exactly one element with aria-live exists inside the palette.
//           ActiveContextHeader and StatusBadge must NOT add aria-live.
// FR-029: icon + color for kind='warning' (color-independence).
// FR-031: key={reannounceKey} drives remount so same-message re-emits still
//         reach the screen reader.
// FR-033: store.inlineStatus.seq change replaces the DOM text child.
// ADR-0036: no document/window/HTMLElement refs; useEffect is React-only.

import { useEffect, useState } from "react";
import { usePaletteStore } from "../../store/palette";

export interface InlineStatusProps {
  // CommandPalette flows 'Active session changed to ...' announce through here
  // (ADR-0057 single slot). seq drives value-change detection.
  announce?: { message: string; seq: number };
}

export function InlineStatus(props: InlineStatusProps = {}): JSX.Element {
  const inlineStatus = usePaletteStore((s) => s.inlineStatus);

  // Sentinel -1 ensures any first-mount announce (seq >= 0) always triggers the
  // effect and is displayed, even if announce.seq happens to equal 0.
  const [lastAnnounceSeq, setLastAnnounceSeq] = useState(-1);
  const [showingAnnounce, setShowingAnnounce] = useState(props.announce != null);

  // eslint-disable-next-line react-hooks/exhaustive-deps
  // biome-ignore lint/correctness/useExhaustiveDependencies: intentional — only seq drives re-announce, not the full object
  useEffect(() => {
    if (!props.announce) return;
    if (props.announce.seq === lastAnnounceSeq) return;
    setLastAnnounceSeq(props.announce.seq);
    setShowingAnnounce(true);
    // announce auto-expires after 4 s to align with inlineStatus auto-clear
    const id = setTimeout(() => setShowingAnnounce(false), 4000);
    return () => clearTimeout(id);
  }, [props.announce?.seq]);

  const text = showingAnnounce && props.announce ? props.announce.message : inlineStatus.message;

  // key={reannounceKey} causes React to remount the inner span whenever the
  // source sequence number changes — even for identical text.  The DOM diff
  // sees an empty node replaced by a node with content, which triggers a fresh
  // screen-reader announce (FR-031).
  // Guard mirrors the `text` ternary: if announce is undefined (e.g. parent
  // transitions it away while the 4 s window is still open), fall back to the
  // inlineStatus seq so we never dereference a possibly-undefined announce.seq.
  const reannounceKey =
    showingAnnounce && props.announce ? `a-${props.announce.seq}` : `s-${inlineStatus.seq}`;

  // Derive the CSS kind modifier from whichever source is currently shown.
  // When announce is active we default to 'info' because the announce channel
  // carries informational messages (e.g. 'Active session changed to …'), not
  // warnings — so the warning icon and orange colour should not appear.
  const activeKind = showingAnnounce && props.announce ? "info" : inlineStatus.kind;

  return (
    <output
      className="palette-inline-status"
      aria-live="polite"
      aria-atomic="true"
      data-testid="palette-inline-status"
    >
      {text === "" ? null : (
        <span
          key={reannounceKey}
          data-seq={reannounceKey}
          className={`palette-inline-status__text palette-inline-status__text--${activeKind}`}
        >
          {inlineStatus.kind === "warning" && !showingAnnounce && (
            <span aria-hidden="true" className="palette-inline-status__icon">
              !{" "}
            </span>
          )}
          {text}
        </span>
      )}
    </output>
  );
}
