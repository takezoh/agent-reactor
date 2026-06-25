// Mac platform detection — single source of truth for the web client.
//
// Centralises the Cmd/Ctrl-key decision that previously lived inline in
// `hooks/useGlobalHotkey.ts` and `App.tsx`. Callers will be migrated in
// follow-up tasks (platform-hotkey-rewire / header-integration); this module
// only provides the implementation.
//
// Evaluation order (short-circuiting):
//   1. `typeof navigator === "undefined"` → false (SSR / test safety, FR-D2).
//   2. `navigator.userAgentData.platform` contains "macOS" → true.
//      Modern Chromium exposes UA Client Hints synchronously for low-entropy
//      values like `platform`; prefer it over the deprecated `navigator.platform`.
//   3. `navigator.platform.toUpperCase()` contains "MAC" → true.
//      Deprecated but still the most widely supported signal across browsers.
//   4. `navigator.userAgent.toUpperCase()` contains "MAC" → true.
//      Final fallback for engines that hide `platform` (e.g. some
//      privacy-hardened builds) but still emit "Macintosh" in the UA string.
//   5. Otherwise → false.
//
// The function is intentionally synchronous and does not call
// `userAgentData.getHighEntropyValues()` — keyboard-modifier decisions need a
// blocking answer on the first keypress.

// UA Client Hints' low-entropy surface. `userAgentData` is optional because it
// is unsupported on Safari/Firefox at time of writing; the inner `platform`
// string is also optional because some embedders ship the object but elide
// fields. Declared globally so we never reach for `any` to read it.
declare global {
  interface NavigatorUAData {
    platform?: string;
  }

  interface Navigator {
    userAgentData?: NavigatorUAData;
  }
}

export function isMacPlatform(): boolean {
  if (typeof navigator === "undefined") return false;

  const uaDataPlatform = navigator.userAgentData?.platform;
  if (typeof uaDataPlatform === "string" && uaDataPlatform.includes("macOS")) {
    return true;
  }

  const platform = navigator.platform;
  if (typeof platform === "string" && platform.toUpperCase().includes("MAC")) {
    return true;
  }

  const userAgent = navigator.userAgent;
  if (typeof userAgent === "string" && userAgent.toUpperCase().includes("MAC")) {
    return true;
  }

  return false;
}
