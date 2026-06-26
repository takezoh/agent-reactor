// TagPill — accessible color pill for driver-supplied Card.tags.
//
// Extracted from DriverViewPanel so SessionList (session card row) can render
// the same WCAG AA fg/bg auto-resolution as the driver detail panel.
// Spec ref: FR-TAGPILL-001.

import { contrastRatio, parseColor } from "../../util/contrast";
import type { Tag as TagType } from "../../wire/server";

/** Default token fg/bg used when driver provides an invalid color string. */
const TOKEN_DEFAULT_FG = { r: 230, g: 230, b: 230 }; // #e6e6e6
const TOKEN_DEFAULT_FG_STR = `rgb(${TOKEN_DEFAULT_FG.r},${TOKEN_DEFAULT_FG.g},${TOKEN_DEFAULT_FG.b})`;
const TOKEN_DEFAULT_BG = { r: 51, g: 51, b: 51 }; // #333 (--status-unknown)
const TOKEN_DEFAULT_BG_STR = `rgb(${TOKEN_DEFAULT_BG.r},${TOKEN_DEFAULT_BG.g},${TOKEN_DEFAULT_BG.b})`;

const BLACK = { r: 0, g: 0, b: 0 };
const WHITE = { r: 255, g: 255, b: 255 };

const WCAG_AA = 4.5;

/**
 * FR-TAGPILL-001: Compute accessible fg/bg for a driver-supplied tag.
 * When contrast ratio < 4.5, replace fg with whichever of black/white
 * gives higher contrast against bg, and add a border indicator.
 *
 * Invalid color inputs are replaced by token defaults for BOTH the ratio
 * calculation AND the emitted CSS value so they cannot diverge (FR-WIRE-001).
 */
export function resolveTagPillStyle(
  fgInput: string | undefined,
  bgInput: string | undefined,
): { color: string; backgroundColor: string; border?: string } {
  const fgParsed = fgInput ? parseColor(fgInput) : null;
  const bgParsed = bgInput ? parseColor(bgInput) : null;

  if (fgParsed === null && fgInput) {
    console.warn("[TagPill] Invalid driver fg color (FR-WIRE-001):", fgInput);
  }
  if (bgParsed === null && bgInput) {
    console.warn("[TagPill] Invalid driver bg color (FR-WIRE-001):", bgInput);
  }

  const fg = fgParsed ?? TOKEN_DEFAULT_FG;
  const bg = bgParsed ?? TOKEN_DEFAULT_BG;
  const fgStr = fgParsed !== null ? (fgInput ?? TOKEN_DEFAULT_FG_STR) : TOKEN_DEFAULT_FG_STR;
  const bgStr = bgParsed !== null ? (bgInput ?? TOKEN_DEFAULT_BG_STR) : TOKEN_DEFAULT_BG_STR;

  const ratio = contrastRatio(fg, bg);

  if (ratio < WCAG_AA) {
    const blackRatio = contrastRatio(BLACK, bg);
    const whiteRatio = contrastRatio(WHITE, bg);
    const newFg = blackRatio >= whiteRatio ? "#000000" : "#ffffff";
    return {
      color: newFg,
      backgroundColor: bgStr,
      border: "1px solid currentColor",
    };
  }

  return {
    color: fgStr,
    backgroundColor: bgStr,
  };
}

export interface TagPillProps {
  tag: TagType;
  className?: string;
}

export function TagPill({ tag, className }: TagPillProps) {
  const style = resolveTagPillStyle(tag.fg, tag.bg);
  const cls = ["driver-tag", "driver-tag-pill", className].filter(Boolean).join(" ");
  return (
    <span className={cls} style={style}>
      {tag.text}
    </span>
  );
}
