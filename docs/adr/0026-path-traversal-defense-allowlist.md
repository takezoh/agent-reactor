# ADR 0026 — Path traversal defense uses an allowlist regex on path parameters

Status: Accepted

## Context

A1-δ introduces REST endpoints with a session ID in the path:
`GET /api/sessions/{id}/{transcript|event-log}`. The handler resolves
the path to `<dataDir>/events/<frameID>.{transcript,jsonl}`. Without
sanitization, a malicious client could pass `../../../etc/passwd` or
URL-encoded equivalents to escape the data directory.

Two defense strategies:

1. **Blacklist** sanitization: filter out `..`, `/`, NUL, URL-encoded
   variants, etc. Brittle: each new encoding becomes a regression.
2. **Allowlist** validation: require the parameter to match a strict
   regex that admits only known-safe characters.

Session IDs are produced by the daemon's `Reduce(EvCreateSession)` as
`s1`, `s2`, …, so the actual charset is alphanumeric with optional
underscore / hyphen. The allowlist regex is `^[a-zA-Z0-9_-]+$` —
trivially safe, no path-segment characters admitted.

## Decision

Validate every path-parameter session ID against the regex
`^[a-zA-Z0-9_-]+$` before resolving any filesystem path. Reject with
HTTP 400 if it fails. Apply the same policy to any future REST
endpoint that takes an ID in the path.

Implementation hints:

- A single `validateSessionID(id string) bool` helper used by every
  handler.
- Filesystem paths are constructed by `filepath.Join(dataDir, "events",
  id+suffix)` only after the regex check passes.
- The regex is compiled once at package init via `regexp.MustCompile`.

## Consequences

- Path traversal is impossible by construction; the regex never admits
  a `..` or `/`.
- New ID conventions (e.g. UUIDs) would need to update the regex; that
  is a one-line, reviewable change.
- The defense is independent of the OS path-separator semantics — the
  regex blocks everything we don't recognize, not what we have learned
  to block.
- Marginal cost: one regex match per request.

## Alternatives

- **Blacklist with `..` removal** — fragile; encoded `..` / `%2e%2e`
  variants slip through. Rejected.
- **`filepath.Clean` + dataDir prefix check** — catches traversal but
  permits surprising IDs (`.`, empty string, spaces) that may confuse
  the daemon side. Rejected as redundant — allowlist is stricter and
  cheaper.

## Related requirements

- FR-δ06
