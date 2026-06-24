package devcontainer

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/takezoh/agent-reactor/platform/sandbox"
)

// ErrMissingImage is returned when devcontainer.json has neither image nor build.name.
var ErrMissingImage = errors.New("devcontainer.json: image or build.name is required (Reactor does not build images)")

// Isolation controls container sharing behaviour for a DevcontainerSpec.
// It aliases sandbox.IsolationKind so the spec and the launch-time IsolationPlan
// share one underlying type — there is no second representation of shared vs
// project to keep in sync.
type Isolation = sandbox.IsolationKind

const (
	// IsolationProject gives each project its own container (default).
	IsolationProject = sandbox.IsolationProject
	// IsolationShared uses a single shared container for all projects.
	IsolationShared = sandbox.IsolationShared
)

var localEnvRe = regexp.MustCompile(`\$\{localEnv:([A-Za-z_][A-Za-z0-9_]*)\}`)

// envVarRe matches $VAR, ${VAR}, and ${containerEnv:VAR} for env layered expansion.
// Capture groups: [1]=containerEnv form, [2]=${VAR} form, [3]=$VAR form.
var envVarRe = regexp.MustCompile(`\$(?:\{containerEnv:([A-Za-z_][A-Za-z0-9_]*)\}|\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))`)

// DefaultNamePrefix is the legacy/backwards-compatible container & label prefix
// used when NamePrefix is empty. Daemons that want process-isolation from a
// peer arc daemon (e.g. scripts/run-dev.sh side-by-side with the user's TUI
// daemon) MUST configure a distinct prefix — otherwise both daemons compete
// for the same container name and the mount-hash recreate path will rm each
// other's containers (see web-gateway-isolation memory).
const DefaultNamePrefix = "reactor"

// DevcontainerSpec holds the resolved container configuration for a project,
// derived from devcontainer.json with roost overlay applied.
type DevcontainerSpec struct {
	ProjectPath             string
	Image                   string // resolved from devcontainer.json image: or build.name
	Isolation               Isolation
	NamePrefix              string // container-name & label prefix; empty → DefaultNamePrefix
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
	ExtraPostCreate         [][]string        // reactor-injected extra postCreateCommands, run after PostCreate
	PreExec                 string            // roost extension: shell command run before each docker exec (preExecCommand)
}

// EffectiveNamePrefix returns NamePrefix or DefaultNamePrefix when empty.
// Use this in code paths that build container names, --label keys, or docker
// ps --filter expressions, so a Manager configured with a custom prefix is
// invisible to a peer daemon using the default.
func (s *DevcontainerSpec) EffectiveNamePrefix() string {
	if s == nil || s.NamePrefix == "" {
		return DefaultNamePrefix
	}
	return s.NamePrefix
}

// SpecOverlay carries reactor-injected env/mounts merged on top of base devcontainer.json.
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

// MountConfigurationHash is the deterministic short hash stamped onto the
// reactor-mount-hash container label of EVERY reactor container (both shared and
// per-project). It covers every bind-mount the container is created with —
// workspace mounts plus spec.Mounts (run-dir bind, proxy sockets, host_exec
// overlays, devcontainer.json `mounts`) — so any drift between the spec and a
// live container forces an auto-recreate in ensureContainer.
//
// Earlier this only hashed ExtraWorkspaces, which missed run-dir source path
// changes after the SharedContainerKey unification: a container created with
// /opt/agent-reactor/run -> /home/take/.agent-reactor/run/<random-hash> was happily reused
// by a binary that now writes the codex socket under
// /home/take/.agent-reactor/run/__shared__, and codex frames failed to reach the
// in-container sockbridge.
func (s *DevcontainerSpec) MountConfigurationHash() string {
	entries := make([]string, 0, len(s.ExtraWorkspaces)+len(s.Mounts))
	for _, w := range s.ExtraWorkspaces {
		entries = append(entries, "ws\t"+w.Source+"\t"+w.Target)
	}
	for _, m := range s.Mounts {
		entries = append(entries, "mount\t"+m)
	}
	if len(entries) == 0 {
		return "none"
	}
	sort.Strings(entries)
	return hashShort(strings.Join(entries, "\n"))
}

// LoadSpec reads devcontainer.json from dcDir for projectPath.
// dcDir is <project>/.devcontainer or ~/.devcontainer.
// Call Apply to merge reactor-specific overlay before using the spec.
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
	// Drop devcontainer.json `mounts` whose bind source does not exist on this
	// host. A single devcontainer.json is shared across machines (e.g. WSL and
	// native Linux) via dotfiles, so OS-specific sources like /mnt/c/Obsidian or
	// ${localEnv:USERPROFILE}/Downloads are absent on one side. docker --mount
	// type=bind fails the whole create on any missing source (unlike -v, which
	// auto-creates), so prune here — before MountConfigurationHash and
	// BuildCreateArgs observe spec.Mounts. Scope is intentionally limited to the
	// devcontainer.json mounts: roost's own socket/run-dir mounts are added later
	// by Apply and must still fail loud if their source is missing.
	spec.Mounts = pruneMissingBindSources(spec.Mounts)

	spec.applySubstitution()
	return spec, nil
}

// mountSourceExistsFn reports whether a bind-mount source path exists. It is a
// package var so tests can stub host filesystem checks.
var mountSourceExistsFn = func(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// pruneMissingBindSources returns mounts with --mount-style bind entries whose
// source path is absent removed (logged at WARN). Only long-form `--mount` specs
// (see isLongFormMount) are eligible: short "host:container" specs go to
// `docker -v`, which auto-creates a missing source, so pruning them would wrongly
// drop a mount docker would have honored. Non-bind and unparseable mounts are
// also kept — an unrecognised spec is never silently dropped.
func pruneMissingBindSources(mounts []string) []string {
	out := mounts[:0:0]
	for _, m := range mounts {
		if isLongFormMount(m) {
			if src, _, ok := parseMountSpec(m); ok && !mountSourceExistsFn(src) {
				slog.Warn("devcontainer: skip mount, source missing", "src", src, "mount", m)
				continue
			}
		}
		out = append(out, m)
	}
	return out
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

// isLongFormMount reports whether a mount spec is docker's long "--mount" form
// (comma-separated key=value pairs) rather than the short "-v host:container[:ro]"
// form. The presence of "=" is docker's own discriminator, and it drives three
// coupled decisions that must agree: which flag BuildCreateArgs emits, whether
// parseMountSpec reads key=value or host:container, and whether
// pruneMissingBindSources may drop a missing source — only --mount sources are
// fatal at create time, while -v auto-creates them.
func isLongFormMount(m string) bool {
	return strings.Contains(m, "=")
}

// parseMountSpec parses a `--mount` style spec ("type=bind,source=X,target=Y[,...]").
// Falls back to the short "host:container[:ro]" form when no "=" is present.
// Returns ok=false for non-bind types or missing fields.
func parseMountSpec(spec string) (src, tgt string, ok bool) {
	if !isLongFormMount(spec) {
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
// In shared mode returns "<prefix>-shared"; otherwise "<prefix>-<projectHash>".
func (s *DevcontainerSpec) ContainerName() string {
	p := s.EffectiveNamePrefix()
	if s.Isolation == IsolationShared {
		return p + "-shared"
	}
	return p + "-" + projectHash(s.ProjectPath)
}

// BuildCreateArgs returns the argument list for "docker create <args>".
// The returned slice does NOT include "create" itself.
func (s *DevcontainerSpec) BuildCreateArgs(image string) []string {
	p := s.EffectiveNamePrefix()
	args := []string{
		"--name", s.ContainerName(),
		"--label", p + "-managed=1",
		// <prefix>-mount-hash is stamped for BOTH isolation kinds so ensureContainer
		// can detect mount drift and auto-recreate. Project containers historically
		// omitted it, which silently stranded project-scope sandbox changes (e.g. a
		// newly added host_exec overlay mount) on a reused pre-change container.
		"--label", p + "-mount-hash=" + s.MountConfigurationHash(),
	}
	if s.Isolation == IsolationShared {
		args = append(args, "--label", p+"-isolation=shared")
	} else {
		args = append(args, "--label", p+"-project="+s.ProjectPath)
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
		if isLongFormMount(m) {
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
