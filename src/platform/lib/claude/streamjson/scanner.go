package streamjson

import (
	"bufio"
	"io"
)

// maxLine caps a single NDJSON line. Claude turns carrying large diffs or file
// contents routinely exceed the bufio.Scanner default (64 KiB), which would
// silently end the read loop.
const maxLine = 64 << 20 // 64 MiB

// Scanner reads a stream of NDJSON lines from an io.Reader and decodes each
// into an Event. Empty lines and malformed JSON lines are skipped silently;
// only underlying io errors terminate the scan.
type Scanner struct {
	sc      *bufio.Scanner
	ev      Event
	skipped int
}

// NewScanner returns a Scanner that reads from r.
func NewScanner(r io.Reader) *Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)
	return &Scanner{sc: sc}
}

// Scan advances to the next parseable Event. It returns false when the stream
// ends or an io error occurs. Empty lines and malformed JSON are skipped and
// counted in Skipped.
func (s *Scanner) Scan() bool {
	for s.sc.Scan() {
		ev, err := Parse(s.sc.Bytes())
		if err != nil {
			s.skipped++
			continue
		}
		if ev == nil {
			continue
		}
		s.ev = ev
		return true
	}
	return false
}

// Event returns the most recent event decoded by Scan.
func (s *Scanner) Event() Event { return s.ev }

// Err returns the first non-EOF io error encountered by the underlying scanner.
func (s *Scanner) Err() error { return s.sc.Err() }

// Skipped returns the number of lines skipped due to empty content or parse
// errors.
func (s *Scanner) Skipped() int { return s.skipped }
