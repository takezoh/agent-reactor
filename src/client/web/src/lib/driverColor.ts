// driverColor — TS-side parity for the canonical driver tag colors defined
// in src/client/driver/tags.go (Go side).
//
// The Go side owns these constants — when adding a driver or changing a brand
// color, update both files together. Tests (driverColor.test.ts) lock the
// values to keep them in lockstep.

export type DriverColor = {
  bg: string;
  fg: string;
};

/** Canonical driver tag colors. Matches tags.go constants verbatim. */
const COMMAND_TAG_FG = "#FFFFFF";
const COMMAND_TAG_BG = "#D97757"; // default — also the Claude brand

const CODEX_TAG_BG = "#10A37F";
const GEMINI_TAG_BG = "#1A73E8";

const BASH_TAG_BG = "#4EAA25";
const ZSH_TAG_BG = "#2D6DB5";
const FISH_TAG_BG = "#F57900";
const POWERSHELL_TAG_BG = "#012456";
const POWERSHELL_TAG_FG = "#EEEDF0";
const NUSHELL_TAG_BG = "#3AA675";

const DRIVER_COLORS: Record<string, DriverColor> = {
  claude: { bg: COMMAND_TAG_BG, fg: COMMAND_TAG_FG },
  codex: { bg: CODEX_TAG_BG, fg: COMMAND_TAG_FG },
  gemini: { bg: GEMINI_TAG_BG, fg: COMMAND_TAG_FG },
  bash: { bg: BASH_TAG_BG, fg: COMMAND_TAG_FG },
  zsh: { bg: ZSH_TAG_BG, fg: COMMAND_TAG_FG },
  fish: { bg: FISH_TAG_BG, fg: COMMAND_TAG_FG },
  powershell: { bg: POWERSHELL_TAG_BG, fg: POWERSHELL_TAG_FG },
  pwsh: { bg: POWERSHELL_TAG_BG, fg: POWERSHELL_TAG_FG },
  nu: { bg: NUSHELL_TAG_BG, fg: COMMAND_TAG_FG },
  nushell: { bg: NUSHELL_TAG_BG, fg: COMMAND_TAG_FG },
};

const DEFAULT_DRIVER_COLOR: DriverColor = { bg: COMMAND_TAG_BG, fg: COMMAND_TAG_FG };

/** Resolve a driver name (case-insensitive) to its brand color pair.
 *  Unknown names fall back to the default command-tag color, matching
 *  Go's CommandTag() behaviour. */
export function driverColor(driver: string): DriverColor {
  const key = driver.trim().toLowerCase();
  return DRIVER_COLORS[key] ?? DEFAULT_DRIVER_COLOR;
}
