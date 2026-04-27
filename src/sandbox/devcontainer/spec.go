package devcontainer

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tailscale/hujson"
)

var localEnvRe = regexp.MustCompile(`\$\{localEnv:([A-Za-z_][A-Za-z0-9_]*)\}`)

// DevcontainerSpec holds the resolved container configuration for a project,
// derived from devcontainer.json with roost overlay applied.
type DevcontainerSpec struct {
	ProjectPath     string
	ContainerEnv    map[string]string
	Mounts          []string // docker --mount or -v format
	WorkspaceMount  string   // custom workspace mount (empty = default)
	WorkspaceFolder string   // container-side workspace path
	ContainerUser   string   // docker create -u
	RemoteUser      string   // docker exec -u (fallback: RemoteUser → ContainerUser → "")
	RunArgs         []string // extra docker create args from runArgs field
	PostCreate      []string // nil = no postCreateCommand; else exec argv
}

// SpecOverlay carries roost-injected env/mounts merged on top of base devcontainer.json.
type SpecOverlay struct {
	Env    map[string]string
	Mounts []string // docker --mount format
}

// ProjectScopeImage returns the project-scope image name for the given hash.
func ProjectScopeImage(hash string) string {
	return fmt.Sprintf("roost-proj-%s:latest", hash)
}

// ProjectScopeImageForPath returns the project-scope image name for projectPath.
func ProjectScopeImageForPath(projectPath string) string {
	return ProjectScopeImage(projectHash(projectPath))
}

// UserScopeImage returns the shared user-scope image name.
func UserScopeImage() string {
	return "roost-user:latest"
}

func projectHash(projectPath string) string {
	h := sha256.Sum256([]byte(projectPath))
	return fmt.Sprintf("%x", h[:6])
}

// LoadSpec reads devcontainer.json from dcDir for projectPath.
// dcDir is <project>/.devcontainer or ~/.devcontainer.
// Call Apply to merge roost-specific overlay before using the spec.
func LoadSpec(projectPath, dcDir string) (*DevcontainerSpec, error) {
	dcPath := filepath.Join(dcDir, "devcontainer.json")
	doc, err := readDC(dcPath)
	if err != nil {
		return nil, fmt.Errorf("devcontainer spec: %w", err)
	}

	spec := &DevcontainerSpec{
		ProjectPath:     projectPath,
		ContainerEnv:    cloneEnv(doc.ContainerEnv),
		ContainerUser:   extractString(doc.Extra, "containerUser"),
		RemoteUser:      extractString(doc.Extra, "remoteUser"),
		WorkspaceFolder: extractString(doc.Extra, "workspaceFolder"),
		WorkspaceMount:  extractString(doc.Extra, "workspaceMount"),
		RunArgs:         extractStrings(doc.Extra, "runArgs"),
		PostCreate:      extractPostCreate(doc.Extra),
	}

	ws := spec.workspaceTarget()
	for _, raw := range doc.Mounts {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		spec.Mounts = append(spec.Mounts, substituteVarsInStr(s, projectPath, ws))
	}

	spec.applySubstitution()
	return spec, nil
}

// Apply merges roost overlay env and mounts on top of the base spec.
func (s *DevcontainerSpec) Apply(overlay SpecOverlay) {
	if s.ContainerEnv == nil {
		s.ContainerEnv = make(map[string]string)
	}
	for k, v := range overlay.Env {
		s.ContainerEnv[k] = v
	}
	s.Mounts = append(s.Mounts, overlay.Mounts...)
}

// workspaceTarget returns the container-side workspace path.
func (s *DevcontainerSpec) workspaceTarget() string {
	if s.WorkspaceFolder != "" {
		return s.WorkspaceFolder
	}
	return "/workspaces/" + filepath.Base(s.ProjectPath)
}

// EffectiveUser returns the user for docker exec (remoteUser → containerUser → "").
func (s *DevcontainerSpec) EffectiveUser() string {
	if s.RemoteUser != "" {
		return s.RemoteUser
	}
	return s.ContainerUser
}

// ContainerName returns the stable docker container name for this project.
func (s *DevcontainerSpec) ContainerName() string {
	return "roost-" + projectHash(s.ProjectPath)
}

// BuildCreateArgs returns the argument list for "docker create <args>".
// The returned slice does NOT include "create" itself.
func (s *DevcontainerSpec) BuildCreateArgs(image string) []string {
	args := []string{
		"--name", s.ContainerName(),
		"--label", "roost-managed=1",
		"--label", "roost-project=" + s.ProjectPath,
	}
	if s.ContainerUser != "" {
		args = append(args, "-u", s.ContainerUser)
	}
	args = append(args, "-w", s.workspaceTarget())
	for k, v := range s.ContainerEnv {
		args = append(args, "-e", k+"="+v)
	}
	// workspace mount (default: bind project → /workspaces/<basename>)
	wsMount := s.WorkspaceMount
	if wsMount == "" {
		base := filepath.Base(s.ProjectPath)
		wsMount = fmt.Sprintf("type=bind,source=%s,target=/workspaces/%s,consistency=cached", s.ProjectPath, base)
	}
	args = append(args, "--mount", wsMount)
	for _, m := range s.Mounts {
		// devcontainer.json `mounts` values use --mount syntax (key=value pairs joined by ",").
		// Short "host:container[:ro]" form has no "=" — route those to -v.
		if strings.Contains(m, "=") {
			args = append(args, "--mount", m)
		} else {
			args = append(args, "-v", m)
		}
	}
	args = append(args, s.RunArgs...)
	args = append(args, image)
	// Keep the container alive for `docker exec` to attach later.
	// Equivalent to @devcontainers/cli's default overrideCommand behavior.
	args = append(args, "/bin/sh", "-c", `trap "exit 0" 15; while sleep 1 & wait $!; do :; done`)
	return args
}

func (s *DevcontainerSpec) applySubstitution() {
	ws := s.workspaceTarget()
	env := make(map[string]string, len(s.ContainerEnv))
	for k, v := range s.ContainerEnv {
		env[substituteVarsInStr(k, s.ProjectPath, ws)] = substituteVarsInStr(v, s.ProjectPath, ws)
	}
	s.ContainerEnv = env
	s.WorkspaceMount = substituteVarsInStr(s.WorkspaceMount, s.ProjectPath, ws)
	s.WorkspaceFolder = substituteVarsInStr(s.WorkspaceFolder, s.ProjectPath, ws)
}

// substituteVarsInStr replaces devcontainer variable references.
// Supports: ${localWorkspaceFolder}, ${localWorkspaceFolderBasename},
// ${containerWorkspaceFolder}, ${localEnv:VAR}.
func substituteVarsInStr(s, projectPath, containerWS string) string {
	s = strings.ReplaceAll(s, "${localWorkspaceFolder}", projectPath)
	s = strings.ReplaceAll(s, "${localWorkspaceFolderBasename}", filepath.Base(projectPath))
	s = strings.ReplaceAll(s, "${containerWorkspaceFolder}", containerWS)
	s = localEnvRe.ReplaceAllStringFunc(s, func(match string) string {
		name := localEnvRe.FindStringSubmatch(match)[1]
		return os.Getenv(name)
	})
	return s
}

// devcontainerDoc is the subset of devcontainer.json that roost parses.
// All other keys are captured in Extra and round-tripped verbatim.
type devcontainerDoc struct {
	Mounts       []json.RawMessage          `json:"mounts,omitempty"`
	ContainerEnv map[string]string          `json:"containerEnv,omitempty"`
	Extra        map[string]json.RawMessage `json:"-"`
}

func (d *devcontainerDoc) UnmarshalJSON(data []byte) error {
	type plain struct {
		Mounts       []json.RawMessage `json:"mounts"`
		ContainerEnv map[string]string `json:"containerEnv"`
	}
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	d.Mounts = p.Mounts
	d.ContainerEnv = p.ContainerEnv

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	d.Extra = make(map[string]json.RawMessage)
	skip := map[string]bool{"mounts": true, "containerEnv": true}
	for k, v := range raw {
		if !skip[k] {
			d.Extra[k] = v
		}
	}
	return nil
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
