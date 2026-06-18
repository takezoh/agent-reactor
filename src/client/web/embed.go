// Package web holds the embedded browser client (xterm.js) served by the
// server. It is a client implementation; the server serializes the wire format
// and this package only ships the static assets that render it.
package web

import "embed"

// Assets is the embedded web client: the page, its script, and the vendored
// xterm.js bundle. Everything is served same-origin so a strict
// Content-Security-Policy (script-src 'self') applies — nothing is loaded from
// a CDN, eliminating the third-party-script supply-chain risk.
//
//go:embed index.html app.js vendor
var Assets embed.FS
