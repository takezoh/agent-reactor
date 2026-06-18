package web

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// ticketTTL bounds how long a minted WebSocket ticket stays valid. Tickets are
// single-use; the TTL only caps the window during which an unused one is
// honoured.
const ticketTTL = 30 * time.Second

// ticketStore mints and validates short-lived, single-use tickets that let a
// browser authenticate a WebSocket connection without putting the bearer token
// in the URL. A ticket is minted over the header-authenticated API and consumed
// (removed) on the first /ws connection, so a ticket that leaks into a log is
// useless after one use or ticketTTL, whichever comes first.
type ticketStore struct {
	now func() time.Time // injectable clock for tests
	mu  sync.Mutex
	m   map[string]time.Time // ticket → expiry
}

func newTicketStore() *ticketStore {
	return &ticketStore{now: time.Now, m: make(map[string]time.Time)}
}

// mint returns a fresh single-use ticket valid for ticketTTL.
func (s *ticketStore) mint() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	tok := base64.RawURLEncoding.EncodeToString(b)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictExpiredLocked()
	s.m[tok] = s.now().Add(ticketTTL)
	return tok
}

// consume validates and removes a ticket, returning true exactly once per
// minted, unexpired ticket. An empty, unknown, already-used, or expired ticket
// returns false.
func (s *ticketStore) consume(tok string) bool {
	if tok == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.m[tok]
	if !ok {
		return false
	}
	delete(s.m, tok)
	return s.now().Before(exp)
}

// evictExpiredLocked drops expired tickets so the map cannot grow without bound
// from minted-but-never-used tickets. The caller holds s.mu.
func (s *ticketStore) evictExpiredLocked() {
	now := s.now()
	for tok, exp := range s.m {
		if !now.Before(exp) {
			delete(s.m, tok)
		}
	}
}
