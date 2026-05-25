package devcontainer

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tailscale/hujson"
)

// spec_doc.go holds the devcontainerDoc type and its I/O helpers,
// extracted from spec.go to keep file sizes within the 500-line limit.

// buildDoc holds the build object in devcontainer.json.
// Name is a roost extension: the image tag the user assigns when building externally.
type buildDoc struct {
	Name string `json:"name"`
}

// devcontainerDoc is the subset of devcontainer.json that roost parses.
// All other keys are captured in Extra and round-tripped verbatim.
type devcontainerDoc struct {
	Image        string                     `json:"image,omitempty"`
	Build        *buildDoc                  `json:"build,omitempty"`
	Mounts       []json.RawMessage          `json:"mounts,omitempty"`
	ContainerEnv map[string]string          `json:"containerEnv,omitempty"`
	RemoteEnv    map[string]string          `json:"remoteEnv,omitempty"`
	Extra        map[string]json.RawMessage `json:"-"`
}

func (d *devcontainerDoc) UnmarshalJSON(data []byte) error {
	type plain devcontainerDoc
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*d = devcontainerDoc(p)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	d.Extra = make(map[string]json.RawMessage)
	skip := map[string]bool{"image": true, "build": true, "mounts": true, "containerEnv": true, "remoteEnv": true}
	for k, v := range raw {
		if !skip[k] {
			d.Extra[k] = v
		}
	}
	return nil
}

// resolveImage derives the image name from devcontainer.json.
// Priority: top-level image: → build.name.
// Returns ErrMissingImage when neither is set.
func resolveImage(doc *devcontainerDoc) (string, error) {
	if doc.Image != "" {
		return doc.Image, nil
	}
	if doc.Build != nil && doc.Build.Name != "" {
		return doc.Build.Name, nil
	}
	return "", ErrMissingImage
}

func readDC(path string) (*devcontainerDoc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("devcontainer merge: read %s: %w", path, err)
	}
	std, err := hujson.Standardize(data)
	if err != nil {
		return nil, fmt.Errorf("devcontainer merge: parse %s: %w", path, err)
	}
	var doc devcontainerDoc
	if err := json.Unmarshal(std, &doc); err != nil {
		return nil, fmt.Errorf("devcontainer merge: unmarshal %s: %w", path, err)
	}
	return &doc, nil
}

func extractString(extra map[string]json.RawMessage, key string) string {
	raw, ok := extra[key]
	if !ok {
		return ""
	}
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}

func extractStrings(extra map[string]json.RawMessage, key string) []string {
	raw, ok := extra[key]
	if !ok {
		return nil
	}
	var ss []string
	if err := json.Unmarshal(raw, &ss); err != nil {
		return nil
	}
	return ss
}

// extractPostCreate parses postCreateCommand (string or string array) into exec argv.
// String form is wrapped as ["bash", "-lc", "<cmd>"].
func extractPostCreate(extra map[string]json.RawMessage) []string {
	raw, ok := extra["postCreateCommand"]
	if !ok {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "" {
			return nil
		}
		return []string{"bash", "-lc", s}
	}
	var ss []string
	if err := json.Unmarshal(raw, &ss); err == nil {
		return ss
	}
	return nil
}

func cloneEnv(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
