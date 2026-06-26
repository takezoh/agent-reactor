// driverColor.test.ts — lock the TS color values to the Go source so the two
// stay in lockstep. When updating src/client/driver/tags.go, update both this
// test and the lib in the same commit.

import * as fs from "node:fs";
import * as path from "node:path";
import { describe, expect, it } from "vitest";
import { driverColor } from "./driverColor";

describe("driverColor — canonical brand mapping", () => {
  it("claude uses the default command tag color (#D97757 / #FFFFFF)", () => {
    expect(driverColor("claude")).toEqual({ bg: "#D97757", fg: "#FFFFFF" });
  });

  it("codex uses OpenAI green", () => {
    expect(driverColor("codex")).toEqual({ bg: "#10A37F", fg: "#FFFFFF" });
  });

  it("gemini uses Google blue", () => {
    expect(driverColor("gemini")).toEqual({ bg: "#1A73E8", fg: "#FFFFFF" });
  });

  it("bash uses GNU green", () => {
    expect(driverColor("bash")).toEqual({ bg: "#4EAA25", fg: "#FFFFFF" });
  });

  it("zsh uses Z Shell blue", () => {
    expect(driverColor("zsh")).toEqual({ bg: "#2D6DB5", fg: "#FFFFFF" });
  });

  it("fish uses fish orange", () => {
    expect(driverColor("fish")).toEqual({ bg: "#F57900", fg: "#FFFFFF" });
  });

  it("powershell and pwsh share the PowerShell navy/off-white pair", () => {
    expect(driverColor("powershell")).toEqual({ bg: "#012456", fg: "#EEEDF0" });
    expect(driverColor("pwsh")).toEqual({ bg: "#012456", fg: "#EEEDF0" });
  });

  it("nu and nushell share the Nushell green", () => {
    expect(driverColor("nu")).toEqual({ bg: "#3AA675", fg: "#FFFFFF" });
    expect(driverColor("nushell")).toEqual({ bg: "#3AA675", fg: "#FFFFFF" });
  });

  it("unknown driver names fall back to the default command tag color", () => {
    expect(driverColor("something-unknown")).toEqual({ bg: "#D97757", fg: "#FFFFFF" });
    expect(driverColor("")).toEqual({ bg: "#D97757", fg: "#FFFFFF" });
  });

  it("is case-insensitive", () => {
    expect(driverColor("CLAUDE")).toEqual(driverColor("claude"));
    expect(driverColor("  Codex  ")).toEqual(driverColor("codex"));
  });

  it("matches the Go constants in src/client/driver/tags.go (regression gate)", () => {
    // Read the Go source and assert each constant we depend on is still
    // defined verbatim. If the Go side renames or recolors a driver, this
    // test catches the drift before the UI ships a stale colour.
    const goPath = path.resolve(__dirname, "../../../../client/driver/tags.go");
    const src = fs.readFileSync(goPath, "utf-8");
    expect(src).toContain('commandTagBg = "#D97757"');
    expect(src).toContain('commandTagFg = "#FFFFFF"');
    expect(src).toContain('codexTagBg   = "#10A37F"');
    expect(src).toContain('geminiTagBg  = "#1A73E8"');
    expect(src).toContain('bashTagBg       = "#4EAA25"');
    expect(src).toContain('zshTagBg        = "#2D6DB5"');
    expect(src).toContain('fishTagBg       = "#F57900"');
    expect(src).toContain('powershellTagBg = "#012456"');
    expect(src).toContain('powershellTagFg = "#EEEDF0"');
    expect(src).toContain('nushellTagBg    = "#3AA675"');
  });
});
