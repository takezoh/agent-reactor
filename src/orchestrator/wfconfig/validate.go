package wfconfig

import "fmt"

func validate(c *Config) error {
	if c.Hooks.TimeoutMS <= 0 {
		return fmt.Errorf("%w: hooks.timeout_ms must be > 0, got %d", ErrConfigValidation, c.Hooks.TimeoutMS)
	}
	if c.Agent.MaxTurns <= 0 {
		return fmt.Errorf("%w: agent.max_turns must be > 0, got %d", ErrConfigValidation, c.Agent.MaxTurns)
	}
	if c.Polling.IntervalMS <= 0 {
		return fmt.Errorf("%w: polling.interval_ms must be > 0, got %d", ErrConfigValidation, c.Polling.IntervalMS)
	}
	if c.Codex.TurnTimeoutMS <= 0 {
		return fmt.Errorf("%w: codex.turn_timeout_ms must be > 0, got %d", ErrConfigValidation, c.Codex.TurnTimeoutMS)
	}
	if c.Codex.ReadTimeoutMS <= 0 {
		return fmt.Errorf("%w: codex.read_timeout_ms must be > 0, got %d", ErrConfigValidation, c.Codex.ReadTimeoutMS)
	}
	if c.Codex.StallTimeoutMS <= 0 {
		return fmt.Errorf("%w: codex.stall_timeout_ms must be > 0, got %d", ErrConfigValidation, c.Codex.StallTimeoutMS)
	}
	return nil
}
