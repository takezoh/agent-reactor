package devcontainer

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ErrMissingImage is returned when devcontainer.json has neither image nor build.name.
var ErrMissingImage = errors.New("devcontainer.json: image or build.name is required (roost does not build images)")

// Isolation controls container sharing behaviour for a DevcontainerSpec.
type Isolation int

const (
	// IsolationProject gives each project its own container (default).
	IsolationProject Isolation = iota
	// IsolationShared uses a single shared container for all projects.
	IsolationShared
)

var localEnvRe = regexp.MustCompile(`\$\{localEnv:([A-Za-z_][A-Za-z0-9_]*)\}`)

// envVarRe matches $VAR, ${VAR}, and ${containerEnv:VAR} for env layered expansion.
// Capture groups: [1]=containerEnv form, [2]=${VAR} form, [3]=$VAR form.
var envVarRe = regexp.MustCompile(`\$(?:\{containerEnv:([A-Za-z_][A-Za-z0-9_]*)\}|\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))`)

// DevcontainerSpec holds the resolved container configuration for a project,
// derived from devcontainer.json with roost overlay applied.
type DevcontainerSpec struct {
	ProjectPath             string
	Image                   string // resolved from devcontainer.json image: or build.name
	Isolation               Isolation
	ContainerEnv            map[string]string
	RemoteEnv               map[string]string // applied via docker exec -e (like VS Code remote processes)
	Mounts                  []string          // docker --mount or -v format
	ExtraWorkspaces         []BindMount       // shared-mode: additional workspace bind-mounts
	WorkspaceMount          string            // custom workspace mount (empty = default)
	WorkspaceFolder         string            // container-side workspace path
	WorkspaceFolderFallback string            // overlay-supplied fallback when WorkspaceFolder is unset
	ContainerUser           string            // docker create -u
	RemoteUser              string            // docker exec -u (fallback: RemoteUser → ContainerUser → "")
	RunArgs                 []string          // extra docker create args from runArgs field
	ExtraCreateArgs         []string          // extra docker create args from settings (injected before image)
	PostCreate              []string          // nil = no postCreateCommand; else exec argv
	ExtraPostCreate         [][]string        // roost-injected extra postCreateCommands, run after PostCreate
	PreExec                 string            // roost extension: shell command run before each docker exec (preExecCommand)
}

// SpecOverlay carries roost-injected env/mounts merged on top of base devcontainer.json.
type SpecOverlay struct {
	Env                     map[string]string
	Mounts                  []string
	ExtraWorkspaces         []BindMount // shared-mode: additional workspace bind-mounts
	ExtraCreateArgs         []string    // docker create options; injected before image name
	PreExec                 string      // fallback preExecCommand if not set in devcontainer.json
	PostCreate              []string    // extra postCreateCommand argv; run after devcontainer.json's postCreateCommand
	WorkspaceFolderFallback string      // fallback container workspace path when devcontainer.json omits workspaceFolder/workspaceMount
}

// hashShort returns the first 12 hex characters of sha256(s),
// used for short stable identifiers in container names and labels.
func hashShort(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:6])
}

func projectHash(projectPath string) string {
	return hashShort(projectPath)
}

// ExtraWorkspacesHash returns a deterministic short hash of ExtraWorkspaces,
// used as the roost-mount-hash container label to detect mount drift between
// the live container and the current spec. Returns "none" when empty.
func (s *DevcontainerSpec) ExtraWorkspacesHash() string {
	if len(s.ExtraWorkspaces) == 0 {
		return "none"
	}
	entries := make([]string, len(s.ExtraWorkspaces))
	for i, w := range s.ExtraWorkspaces {
		entries[i] = w.Source + "\t" + w.Target
	}
	sort.Strings(entries)
	return hashShort(strings.Join(entries, "\n"))
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

	image, err := resolveImage(doc)
	if err != nil {
		return nil, fmt.Errorf("devcontainer spec: %w", err)
	}

	spec := &DevcontainerSpec{
		ProjectPath:     projectPath,
		Image:           image,
		ContainerEnv:    cloneEnv(doc.ContainerEnv),
		RemoteEnv:       cloneEnv(doc.RemoteEnv),
		ContainerUser:   extractString(doc.Extra, "containerUser"),
		RemoteUser:      extractString(doc.Extra, "remoteUser"),
		WorkspaceFolder: extractString(doc.Extra, "workspaceFolder"),
		WorkspaceMount:  extractString(doc.Extra, "workspaceMount"),
		RunArgs:         extractStrings(doc.Extra, "runArgs"),
		PostCreate:      extractPostCreate(doc.Extra),
		PreExec:         extractString(doc.Extra, "preExecCommand"),
	}

	ws := spec.WorkspaceTarget()
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
// Overlay env is written to both ContainerEnv (docker create -e, visible from PID 1) and
// RemoteEnv (docker exec -e, re-injected on every frame launch). The RemoteEnv path ensures
// that overlay env reaches exec'd processes even when an existing container is reused,
// since Config.Env is frozen at container creation time.
func (s *DevcontainerSpec) Apply(overlay SpecOverlay) {
	if s.ContainerEnv == nil {
		s.ContainerEnv = make(map[string]string)
	}
	if s.RemoteEnv == nil {
		s.RemoteEnv = make(map[string]string)
	}
	for k, v := range overlay.Env {
		s.ContainerEnv[k] = prependEnvVal(v, s.ContainerEnv[k])
		s.RemoteEnv[k] = prependEnvVal(v, s.RemoteEnv[k])
	}
	s.Mounts = append(s.Mounts, overlay.Mounts...)
	s.ExtraWorkspaces = append(s.ExtraWorkspaces, overlay.ExtraWorkspaces...)
	s.ExtraCreateArgs = append(s.ExtraCreateArgs, overlay.ExtraCreateArgs...)
	if s.PreExec == "" {
		s.PreExec = overlay.PreExec
	}
	if len(overlay.PostCreate) > 0 {
		s.ExtraPostCreate = append(s.ExtraPostCreate, overlay.PostCreate)
	}
	if overlay.WorkspaceFolderFallback != "" {
		s.WorkspaceFolderFallback = overlay.WorkspaceFolderFallback
	}
}

// envSuffixRe matches a trailing ":<var-ref>" (e.g. ":$PATH", ":${containerEnv:PATH}")
// so the self-reference can be stripped before prepending to an existing value.
var envSuffixRe = regexp.MustCompile(`:\$(?:[A-Za-z_][A-Za-z0-9_]*|\{[^}]+\})$`)

// prependEnvVal prepends the overlay prefix to existing, stripping any trailing
// ":<var-ref>" suffix to avoid double-expansion. No suffix means plain override.
func prependEnvVal(overlay, existing string) string {
	if existing == "" {
		return overlay
	}
	prefix := envSuffixRe.ReplaceAllString(overlay, "")
	if prefix == overlay {
		return overlay
	}
	if prefix == "" {
		return existing
	}
	return prefix + ":" + existing
}

// WorkspaceTarget returns the container-side workspace path used for -w and the
// default workspace mount.
// Priority: devcontainer.json workspaceFolder > overlay WorkspaceFolderFallback > ProjectPath (host-mirrored).
func (s *DevcontainerSpec) WorkspaceTarget() string {
	if s.WorkspaceFolder != "" {
		return s.WorkspaceFolder
	}
	if s.WorkspaceFolderFallback != "" {
		return s.WorkspaceFolderFallback
	}
	return s.ProjectPath
}

// BindMount is a parsed (source, target) pair for a docker bind mount.
type BindMount struct {
	Source string
	Target string
}

// BindMounts returns every bind mount this spec materialises at `docker create`
// time. Used by the runtime to register host↔container path mappings with
// pathmap so hook payload translation covers user-declared mounts. Sources
// scanned: WorkspaceMount (with default fallback), ExtraWorkspaces (shared mode),
// Mounts (devcontainer.json), RunArgs, ExtraCreateArgs. Non-bind mounts are
// skipped; the read-only flag is ignored.
func (s *DevcontainerSpec) BindMounts() []BindMount {
	var out []BindMount
	add := func(src, tgt string) {
		if src != "" && tgt != "" {
			out = append(out, BindMount{Source: src, Target: tgt})
		}
	}

	if s.Isolation != IsolationShared {
		if s.WorkspaceMount == "" {
			add(s.ProjectPath, s.WorkspaceTarget())
		} else if src, tgt, ok := parseMountSpec(s.WorkspaceMount); ok {
			add(src, tgt)
		}
	}
	for _, ws := range s.ExtraWorkspaces {
		add(ws.Source, ws.Target)
	}
	for _, m := range s.Mounts {
		if src, tgt, ok := parseMountSpec(m); ok {
			add(src, tgt)
		}
	}
	scan := func(args []string) {
		for i := 0; i < len(args); i++ {
			a := args[i]
			switch {
			case a == "--mount" && i+1 < len(args):
				if src, tgt, ok := parseMountSpec(args[i+1]); ok {
					add(src, tgt)
				}
				i++
			case strings.HasPrefix(a, "--mount="):
				if src, tgt, ok := parseMountSpec(strings.TrimPrefix(a, "--mount=")); ok {
					add(src, tgt)
				}
			case a == "-v" && i+1 < len(args):
				if src, tgt, ok := parseShortMount(args[i+1]); ok {
					add(src, tgt)
				}
				i++
			}
		}
	}
	scan(s.RunArgs)
	scan(s.ExtraCreateArgs)
	return out
}

// parseMountSpec parses a `--mount` style spec ("type=bind,source=X,target=Y[,...]").
// Falls back to the short "host:container[:ro]" form when no "=" is present.
// Returns ok=false for non-bind types or missing fields.
func parseMountSpec(spec string) (src, tgt string, ok bool) {
	if !strings.Contains(spec, "=") {
		return parseShortMount(spec)
	}
	typ := "bind"
	for _, kv := range strings.Split(spec, ",") {
		k, v, found := strings.Cut(kv, "=")
		if !found {
			continue
		}
		switch strings.TrimSpace(k) {
		case "type":
			typ = strings.TrimSpace(v)
		case "source", "src":
			src = v
		case "target", "destination", "dst":
			tgt = v
		}
	}
	if typ != "bind" {
		return "", "", false
	}
	return src, tgt, src != "" && tgt != ""
}

// parseShortMount parses the short "-v host:container[:ro]" form.
func parseShortMount(spec string) (src, tgt string, ok bool) {
	parts := strings.SplitN(spec, ":", 3)
	if len(parts) < 2 {
		return "", "", false
	}
	return parts[0], parts[1], parts[0] != "" && parts[1] != ""
}

// EffectiveUser returns the user for docker exec (remoteUser → containerUser → "").
func (s *DevcontainerSpec) EffectiveUser() string {
	if s.RemoteUser != "" {
		return s.RemoteUser
	}
	return s.ContainerUser
}

// ContainerName returns the stable docker container name.
// In shared mode returns the fixed name "roost-shared"; otherwise a per-project hash.
func (s *DevcontainerSpec) ContainerName() string {
	if s.Isolation == IsolationShared {
		return "roost-shared"
	}
	return "roost-" + projectHash(s.ProjectPath)
}

// BuildCreateArgs returns the argument list for "docker create <args>".
// The returned slice does NOT include "create" itself.
func (s *DevcontainerSpec) BuildCreateArgs(image string) []string {
	args := []string{
		"--name", s.ContainerName(),
		"--label", "roost-managed=1",
	}
	if s.Isolation == IsolationShared {
		args = append(args, "--label", "roost-isolation=shared")
		args = append(args, "--label", "roost-mount-hash="+s.ExtraWorkspacesHash())
	} else {
		args = append(args, "--label", "roost-project="+s.ProjectPath)
	}
	if s.ContainerUser != "" {
		args = append(args, "-u", s.ContainerUser)
	}
	args = append(args, "-w", s.WorkspaceTarget())
	for k, v := range s.ContainerEnv {
		args = append(args, "-e", k+"="+v)
	}
	if s.Isolation != IsolationShared {
		wsMount := s.WorkspaceMount
		if wsMount == "" {
			wsMount = fmt.Sprintf("type=bind,source=%s,target=%s,consistency=cached", s.ProjectPath, s.WorkspaceTarget())
		}
		args = append(args, "--mount", wsMount)
	}
	for _, ws := range s.ExtraWorkspaces {
		args = append(args, "--mount", fmt.Sprintf("type=bind,source=%s,target=%s,consistency=cached", ws.Source, ws.Target))
	}
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
	args = append(args, s.ExtraCreateArgs...)
	args = append(args, image)
	// Keep the container alive for `docker exec` to attach later.
	// Equivalent to @devcontainers/cli's default overrideCommand behavior.
	args = append(args, "/bin/sh", "-c", `trap "exit 0" 15; while sleep 1 & wait $!; do :; done`)
	return args
}

func (s *DevcontainerSpec) applySubstitution() {
	ws := s.WorkspaceTarget()
	s.ContainerEnv = substituteEnvMap(s.ContainerEnv, s.ProjectPath, ws)
	s.RemoteEnv = substituteEnvMap(s.RemoteEnv, s.ProjectPath, ws)
	s.WorkspaceMount = substituteVarsInStr(s.WorkspaceMount, s.ProjectPath, ws)
	s.WorkspaceFolder = substituteVarsInStr(s.WorkspaceFolder, s.ProjectPath, ws)
}

func substituteEnvMap(src map[string]string, projectPath, ws string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[substituteVarsInStr(k, projectPath, ws)] = substituteVarsInStr(v, projectPath, ws)
	}
	return dst
}

// ResolveContainerEnvPlaceholders expands env variable references in ContainerEnv and RemoteEnv
// using a three-layer model:
//   - L1: imageEnv (image baseline from docker inspect)
//   - L2: ContainerEnv — $VAR / ${VAR} / ${containerEnv:VAR} resolved against L1
//   - L3: RemoteEnv  — same forms resolved against L1 ∪ resolved-L2
//
// Undefined variables expand to empty string.
func (s *DevcontainerSpec) ResolveContainerEnvPlaceholders(imageEnv map[string]string) {
	resolve := func(v string, lookup map[string]string) string {
		return envVarRe.ReplaceAllStringFunc(v, func(m string) string {
			subs := envVarRe.FindStringSubmatch(m)
			for _, name := range subs[1:] {
				if name != "" {
					return lookup[name]
				}
			}
			return ""
		})
	}
	for k, v := range s.ContainerEnv {
		s.ContainerEnv[k] = resolve(v, imageEnv)
	}
	// L3 sees L1 ∪ resolved-L2 (containerEnv overrides image baseline)
	merged := make(map[string]string, len(imageEnv)+len(s.ContainerEnv))
	for k, v := range imageEnv {
		merged[k] = v
	}
	for k, v := range s.ContainerEnv {
		merged[k] = v
	}
	for k, v := range s.RemoteEnv {
		s.RemoteEnv[k] = resolve(v, merged)
	}
	if p, ok := s.ContainerEnv["PATH"]; ok {
		s.ContainerEnv["PATH"] = deduplicateColonList(p)
	}
	if p, ok := s.RemoteEnv["PATH"]; ok {
		s.RemoteEnv["PATH"] = deduplicateColonList(p)
	}
}

func deduplicateColonList(s string) string {
	segs := strings.Split(s, ":")
	if len(segs) == 1 {
		return s
	}
	seen := make(map[string]bool, len(segs))
	out := make([]string, 0, len(segs))
	for _, seg := range segs {
		if seg == "" || seen[seg] {
			continue
		}
		seen[seg] = true
		out = append(out, seg)
	}
	return strings.Join(out, ":")
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
