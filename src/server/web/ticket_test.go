package web

import (
	"testing"
	"time"
)

func TestTicketSingleUse(t *testing.T) {
	s := newTicketStore()
	tok := s.mint()
	if tok == "" {
		t.Fatal("mint returned empty ticket")
	}
	if !s.consume(tok) {
		t.Fatal("first consume should succeed")
	}
	if s.consume(tok) {
		t.Fatal("second consume must fail: ticket is single-use")
	}
}

func TestTicketUnknownAndEmpty(t *testing.T) {
	s := newTicketStore()
	if s.consume("") {
		t.Fatal("empty ticket must be rejected")
	}
	if s.consume("never-minted") {
		t.Fatal("unknown ticket must be rejected")
	}
}

func TestTicketDistinct(t *testing.T) {
	s := newTicketStore()
	a, b := s.mint(), s.mint()
	if a == b {
		t.Fatal("mint must return distinct tickets")
	}
}

func TestTicketExpires(t *testing.T) {
	now := time.Unix(1_000, 0)
	s := newTicketStore()
	s.now = func() time.Time { return now }

	tok := s.mint()
	now = now.Add(ticketTTL + time.Second) // advance past expiry
	if s.consume(tok) {
		t.Fatal("expired ticket must be rejected")
	}
}

func TestTicketEvictsExpired(t *testing.T) {
	now := time.Unix(1_000, 0)
	s := newTicketStore()
	s.now = func() time.Time { return now }

	stale := s.mint()
	now = now.Add(ticketTTL + time.Second)
	s.mint() // minting evicts expired entries

	s.mu.Lock()
	_, present := s.m[stale]
	count := len(s.m)
	s.mu.Unlock()
	if present {
		t.Fatal("expired ticket should have been evicted on mint")
	}
	if count != 1 {
		t.Fatalf("store should hold only the fresh ticket, got %d", count)
	}
}
