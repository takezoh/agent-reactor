package metrics_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/takezoh/agent-roost/platform/metrics"
)

func TestAccumulator_SingleThread_NoDoubleCount(t *testing.T) {
	acc := metrics.NewAccumulator()

	acc = acc.Observe(metrics.Usage{ThreadID: "t1", Input: 100, Output: 50, Total: 150})
	require.Equal(t, metrics.Totals{Input: 100, Output: 50, Total: 150}, acc.Totals())

	// Cumulative absolute 250 — only the delta (150, 70, 220) is added, not 250 again.
	acc = acc.Observe(metrics.Usage{ThreadID: "t1", Input: 250, Output: 120, Total: 370})
	require.Equal(t, metrics.Totals{Input: 250, Output: 120, Total: 370}, acc.Totals())
}

func TestAccumulator_MultiThread_SumsIndependently(t *testing.T) {
	acc := metrics.NewAccumulator()
	acc = acc.Observe(metrics.Usage{ThreadID: "t1", Input: 100, Output: 50, Total: 150})
	acc = acc.Observe(metrics.Usage{ThreadID: "t2", Input: 200, Output: 80, Total: 280})
	require.Equal(t, metrics.Totals{Input: 300, Output: 130, Total: 430}, acc.Totals())
}

func TestAccumulator_DoesNotMutateReceiver(t *testing.T) {
	base := metrics.NewAccumulator().Observe(metrics.Usage{ThreadID: "t1", Input: 100, Output: 50, Total: 150})

	// Observing on base must not change base — value semantics.
	_ = base.Observe(metrics.Usage{ThreadID: "t1", Input: 250, Output: 120, Total: 370})
	require.Equal(t, metrics.Totals{Input: 100, Output: 50, Total: 150}, base.Totals())
}

func TestAccumulator_SameValueNoChange(t *testing.T) {
	acc := metrics.NewAccumulator()
	acc = acc.Observe(metrics.Usage{ThreadID: "t1", Input: 100, Output: 50, Total: 150})
	acc = acc.Observe(metrics.Usage{ThreadID: "t1", Input: 100, Output: 50, Total: 150})
	require.Equal(t, metrics.Totals{Input: 100, Output: 50, Total: 150}, acc.Totals())
}

func TestAccumulator_NegativeDeltaClamped(t *testing.T) {
	acc := metrics.NewAccumulator()
	acc = acc.Observe(metrics.Usage{ThreadID: "t1", Input: 100, Output: 50, Total: 150})

	// Decrease (monotonic violation) must not subtract from the total.
	acc = acc.Observe(metrics.Usage{ThreadID: "t1", Input: 80, Output: 50, Total: 130})
	require.Equal(t, metrics.Totals{Input: 100, Output: 50, Total: 150}, acc.Totals())
}

func TestRuntimeAccumulator_SumsTurns(t *testing.T) {
	var r metrics.RuntimeAccumulator
	r.AddTurn(2 * time.Second)
	r.AddTurn(3 * time.Second)
	require.Equal(t, 5*time.Second, r.Total())
}

func TestRuntimeAccumulator_NegativeIgnored(t *testing.T) {
	var r metrics.RuntimeAccumulator
	r.AddTurn(-1 * time.Second)
	require.Equal(t, time.Duration(0), r.Total())
}
