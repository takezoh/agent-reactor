package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorker_WasKilledGracefully verifies that only "terminal" and "non-active" reasons
// set the graceful flag, leaving "stall" (and any other reason) as non-graceful.
func TestWorker_WasKilledGracefully(t *testing.T) {
	tests := []struct {
		reason   string
		graceful bool
	}{
		{"terminal", true},
		{"non-active", true},
		{"stall", false},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			done := make(chan struct{})
			ctx, cancel := context.WithCancel(context.Background())
			w := &Worker{
				cancel: cancel,
				done:   done,
			}
			go func() {
				<-ctx.Done()
				close(done)
			}()
			err := w.Kill(tt.reason)
			require.NoError(t, err)
			assert.Equal(t, tt.graceful, w.WasKilledGracefully())
		})
	}
}
