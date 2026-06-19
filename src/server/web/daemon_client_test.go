package web

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/takezoh/agent-reactor/client/proto"
)

const (
	testMinDelay = time.Millisecond
	testMaxDelay = 2 * time.Millisecond
)

// errDialer returns a dialFunc that always returns an error.
func errDialer(err error) dialFunc {
	return func() (*proto.Client, error) { return nil, err }
}

// waitHealth polls Health() up to deadline; returns whether the target value
// was observed.
func waitHealth(d *DaemonClient, want bool, deadline time.Duration) bool { //nolint:unparam
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if d.Health() == want {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return d.Health() == want
}

// TestDaemonClient_DialSuccess verifies that a successful initial dial sets
// Health to true.
func TestDaemonClient_DialSuccess(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	d := NewDaemonClientWithDialer(func() (*proto.Client, error) {
		return proto.DialConn(clientConn), nil
	}, testMinDelay, testMaxDelay)
	defer d.Close()

	if !d.Health() {
		t.Fatal("expected Health() == true after successful dial")
	}
	if d.LastError() != nil {
		t.Fatalf("expected LastError() == nil, got %v", d.LastError())
	}
	if d.LastAttemptAt().IsZero() {
		t.Fatal("expected LastAttemptAt() to be set")
	}
}

// TestDaemonClient_DialFailureRetries verifies that a dialer that fails
// the first 3 calls then succeeds on the 4th eventually brings Health to true.
func TestDaemonClient_DialFailureRetries(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	defer serverConn.Close()

	var callCount atomic.Int32
	dialer := func() (*proto.Client, error) {
		n := callCount.Add(1)
		if n < 4 {
			return nil, errors.New("temporary failure")
		}
		return proto.DialConn(clientConn), nil
	}

	d := NewDaemonClientWithDialer(dialer, testMinDelay, testMaxDelay)
	defer d.Close()

	if !waitHealth(d, true, 2*time.Second) {
		t.Fatalf("Health() never became true; callCount=%d, lastErr=%v",
			callCount.Load(), d.LastError())
	}

	if d.LastAttemptAt().IsZero() {
		t.Fatal("LastAttemptAt() should be set")
	}
}

// TestDaemonClient_DisconnectAndReconnect verifies that when the server side
// closes the connection the events channel is closed and a fresh one is
// available after reconnection.
func TestDaemonClient_DisconnectAndReconnect(t *testing.T) {
	t.Parallel()

	// First connection (will be closed to trigger reconnect).
	c1, s1 := net.Pipe()
	// Second connection (reconnect target).
	c2, s2 := net.Pipe()
	defer s2.Close()

	var callCount atomic.Int32
	dialer := func() (*proto.Client, error) {
		n := callCount.Add(1)
		switch n {
		case 1:
			return proto.DialConn(c1), nil
		case 2:
			return proto.DialConn(c2), nil
		default:
			return nil, errors.New("no more conns")
		}
	}

	d := NewDaemonClientWithDialer(dialer, testMinDelay, testMaxDelay)
	defer d.Close()

	if !waitHealth(d, true, time.Second) {
		t.Fatal("Health() never became true after initial dial")
	}

	// Snapshot the first events channel.
	firstCh := d.SubscribeEvents()

	// Close the server side to simulate a daemon disconnect.
	s1.Close()

	// The first events channel must eventually be closed.
	drainDone := make(chan struct{})
	go func() {
		defer close(drainDone)
		for range firstCh {
		}
	}()
	select {
	case <-drainDone:
	case <-time.After(2 * time.Second):
		t.Error("first events channel was not closed after disconnect")
		return
	}

	// Wait for reconnection.
	if !waitHealth(d, true, 2*time.Second) {
		t.Fatal("Health() never became true after reconnect")
	}

	// The new events channel must be different from the first.
	secondCh := d.SubscribeEvents()
	if secondCh == firstCh {
		t.Fatal("expected a fresh events channel after reconnect")
	}
}

// TestDaemonClient_SendWhileDown verifies that SendCommand returns
// ErrDaemonUnavailable when no connection is established.
func TestDaemonClient_SendWhileDown(t *testing.T) {
	t.Parallel()

	d := NewDaemonClientWithDialer(errDialer(errors.New("always down")), testMinDelay, testMaxDelay)
	defer d.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := d.SendCommand(ctx, proto.CmdSubscribe{})
	if !errors.Is(err, ErrDaemonUnavailable) {
		t.Fatalf("expected ErrDaemonUnavailable, got %v", err)
	}
}

// TestDaemonClient_InFlightCancellation verifies that cancelling the context
// while SendCommand is waiting returns context.Canceled.
func TestDaemonClient_InFlightCancellation(t *testing.T) {
	t.Parallel()

	// Use net.Pipe: writes block until read, so we need a goroutine on the
	// server side that reads (but never replies) to allow Send to write its
	// frame. Otherwise writeFrame itself blocks.
	clientConn, serverConn := net.Pipe()

	// Server goroutine: consume bytes so the client can write its frame,
	// but never send a reply so Send blocks in its select.
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		buf := make([]byte, 4096)
		for {
			_, err := serverConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()
	defer func() {
		serverConn.Close()
		<-serverDone
	}()

	d := NewDaemonClientWithDialer(func() (*proto.Client, error) {
		return proto.DialConn(clientConn), nil
	}, testMinDelay, testMaxDelay)
	defer d.Close()

	if !waitHealth(d, true, time.Second) {
		t.Fatal("Health() never became true")
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := d.SendCommand(ctx, proto.CmdSubscribe{})
		errCh <- err
	}()

	// Give the goroutine time to enter Send, then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SendCommand did not return after context cancellation")
	}
}

// TestDaemonClient_HealthInitiallyFalse verifies that a DaemonClient with no
// dial attempt yet returns Health()==false without panicking.
func TestDaemonClient_HealthInitiallyFalse(t *testing.T) {
	t.Parallel()

	d := &DaemonClient{
		sockPath: "<test>",
		dial:     errDialer(errors.New("no daemon")),
		events:   make(chan proto.ServerEvent, 64),
		stop:     make(chan struct{}),
		minDelay: testMinDelay,
		maxDelay: testMaxDelay,
	}
	// Do NOT call tryDialOnce or supervise — test the zero-value state only.

	if d.Health() {
		t.Fatal("expected Health() == false before any dial")
	}
	if d.LastError() != nil {
		t.Fatal("expected LastError() == nil before any dial")
	}
	if !d.LastAttemptAt().IsZero() {
		t.Fatal("expected LastAttemptAt() zero before any dial")
	}
}
