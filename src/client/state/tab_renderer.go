package state

import "encoding/json"

// TabRenderer is the interface clients use to render tab content without
// knowing the driver-specific format. Drivers register a factory via
// RegisterTabRenderer at init time; callers create renderers via
// NewTabRenderer using the LogTab's Kind and RendererCfg.
type TabRenderer interface {
	Append(data []byte) string
	Reset()
}

var rendererFactories = map[TabKind]func(json.RawMessage) TabRenderer{}

// RegisterTabRenderer registers a typed factory for a TabKind. The
// generic type parameter C is the driver-specific config struct that
// is serialized into LogTab.RendererCfg by the driver. Same pattern
// as worker.Submit[In, Out].
func RegisterTabRenderer[C any](kind TabKind, factory func(C) TabRenderer) {
	rendererFactories[kind] = func(raw json.RawMessage) TabRenderer {
		var cfg C
		if len(raw) > 0 {
			// Unmarshal errors are silently ignored; factory receives
			// the zero-value config and the renderer falls back to its
			// default behaviour. Logging in state/ would violate the
			// pure-functional-core contract.
			_ = json.Unmarshal(raw, &cfg)
		}
		return factory(cfg)
	}
}

// HasTabRenderer reports whether a factory is registered for the kind.
func HasTabRenderer(kind TabKind) bool {
	_, ok := rendererFactories[kind]
	return ok
}

// NewTabRenderer creates a TabRenderer for the given kind using the
// registered factory. Returns nil if no factory is registered.
func NewTabRenderer(kind TabKind, cfg json.RawMessage) TabRenderer {
	f, ok := rendererFactories[kind]
	if !ok {
		return nil
	}
	return f(cfg)
}
