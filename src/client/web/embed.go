// Package web is the web-client host for the arc server: it embeds the
// browser client (Vite/React UI built into dist/) and provides Handler, which
// serves that UI under a strict Content-Security-Policy and reverse-proxies the
// data plane (/api, /ws) to the headless backend (cmd/server). The browser
// talks only to this origin; the backend serves no HTML. Wired up by cmd/web.
package web

import (
	"embed"
	"io/fs"
)

// Assets is the embedded Vite build output. Everything under dist/ is served
// same-origin so a strict Content-Security-Policy (script-src 'self') applies —
// no inline scripts, no CDN dependencies, eliminating third-party-script
// supply-chain risk (FR-β03 / FR-β13).
//
//go:embed dist
var Assets embed.FS

// DistFS returns a sub-FS rooted at dist/ so that http.FileServer serves
// dist/index.html at / (rather than at /dist/index.html).
func DistFS() (fs.FS, error) {
	return fs.Sub(Assets, "dist")
}
