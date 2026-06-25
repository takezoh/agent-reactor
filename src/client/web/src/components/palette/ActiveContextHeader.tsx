// ActiveContextHeader — displays the active context in the palette header.
// UAC-001 / UAC-013 / FR-009 / FR-010 / FR-028 / FR-029 / FR-032
// ADR-0055 (lift-state frozen props) / ADR-0057 (role=status, no aria-live)

import { useEffect, useRef, useState } from "react";
import { usePaletteStore } from "../../store/palette";
import type { ActiveContextSnapshot } from "../../store/palette_active_context";

export interface ActiveContextHeaderProps {
  // frozen snapshot props (ADR-0055). undefined means subscribe from store.
  snapshot?: ActiveContextSnapshot;
  // flashSeq is undefined in frozen mode. In store mode it may be omitted.
  flashSeq?: number;
}

const FLASH_MS = 600;

export function ActiveContextHeader(props: ActiveContextHeaderProps = {}): JSX.Element {
  const storeSnapshot = usePaletteStore((s) => s.activeContextSnapshot);
  const storeFlashSeq = usePaletteStore((s) => s.flashSeq);
  const snap = props.snapshot ?? storeSnapshot;
  const flashSeq = props.flashSeq ?? storeFlashSeq;

  const [flashing, setFlashing] = useState(false);
  const lastSeqRef = useRef(flashSeq);

  useEffect(() => {
    if (lastSeqRef.current === flashSeq) return;
    lastSeqRef.current = flashSeq;
    setFlashing(true);
    const id = setTimeout(() => setFlashing(false), FLASH_MS);
    return () => clearTimeout(id);
  }, [flashSeq]);

  const className = `palette-active-context${flashing ? " palette-active-context--flash" : ""}`;

  if (snap.kind === "none") {
    return (
      <output className={className} data-testid="palette-active-context">
        <span aria-hidden="true" className="palette-active-context__icon">
          —
        </span>
        <span className="palette-active-context__text">No active session</span>
      </output>
    );
  }

  if (snap.kind === "unknown") {
    return (
      <output className={className} title={snap.fullSessionId} data-testid="palette-active-context">
        <span className="palette-active-context__label">Active:</span>{" "}
        <span className="palette-active-context__proj">???</span>{" "}
        <span className="palette-active-context__sep">/</span>{" "}
        <span className="palette-active-context__sid">{snap.sid8}</span>
      </output>
    );
  }

  // resolved
  return (
    <output
      className={className}
      title={`${snap.fullPath}\n${snap.fullSessionId}`}
      data-testid="palette-active-context"
    >
      <span className="palette-active-context__label">Active:</span>{" "}
      <span className="palette-active-context__proj">{snap.projBase}</span>{" "}
      <span className="palette-active-context__sep">/</span>{" "}
      <span className="palette-active-context__sid">{snap.sid8}</span>
    </output>
  );
}
