package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestStartGatewayNotifiesReadiness(t *testing.T) {
	prev := notifyReady
	defer func() { notifyReady = prev }()

	called := make(chan struct{}, 1)
	notifyReady = func() error {
		select {
		case called <- struct{}{}:
		default:
		}
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	df := &daemonFlagSet{
		addr:     "127.0.0.1:0",
		insecure: true,
		noAuth:   true,
	}
	h, err := startGateway(ctx, cancel, filepath.Join(t.TempDir(), "server.sock"), df)
	if err != nil {
		t.Fatalf("startGateway() error = %v", err)
	}
	defer h.Close()

	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for readiness notify")
	}
}

func TestStartGatewayKeepsRunningWhenNotifyFails(t *testing.T) {
	prev := notifyReady
	defer func() { notifyReady = prev }()

	notifyReady = func() error {
		return context.Canceled
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	df := &daemonFlagSet{
		addr:     "127.0.0.1:0",
		insecure: true,
		noAuth:   true,
	}
	h, err := startGateway(ctx, cancel, filepath.Join(t.TempDir(), "server.sock"), df)
	if err != nil {
		t.Fatalf("startGateway() error = %v", err)
	}
	h.Close()
}
