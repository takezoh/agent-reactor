package stream

import (
	"strings"

	"github.com/takezoh/agent-roost/client/driver"
)

// Re-exported from driver/ so callers need not import both packages.
const (
	DriverName   = driver.CodexDriverName
	SockPrefix   = driver.CodexAppServerSockPrefix
	SockSuffix   = driver.CodexAppServerSockSuffix
	LoopbackPort = driver.CodexAppServerLoopbackPort
	// RunDirName is the subdirectory under the daemon data dir that holds
	// per-session codex app-server UDS files: <dataDir>/run/<RunDirName>/.
	RunDirName = driver.CodexDriverName
)

// prefixWriter is an io.Writer that captures up to max bytes into dst.
type prefixWriter struct {
	dst *strings.Builder
	max int
}

func newPrefixWriter(dst *strings.Builder, max int) *prefixWriter {
	return &prefixWriter{dst: dst, max: max}
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	if p.dst.Len() < p.max {
		room := p.max - p.dst.Len()
		if room > len(b) {
			room = len(b)
		}
		p.dst.Write(b[:room])
	}
	return len(b), nil
}
