package devcontainer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/takezoh/agent-reactor/platform/sandbox"
	"github.com/takezoh/agent-reactor/platform/shellalias"
)

// SharedContainerKey is the containers map key used for shared-mode instances.
// It aliases sandbox.SharedInstanceKey, the canonical reserved key produced by
// IsolationPlan.ContainerKey, so this package and the generic key derivation
// agree by construction.
const SharedContainerKey = sandbox.SharedInstanceKey

// Docker call indirections. Tests override these to drive Manager scenarios
// that would otherwise require a real docker daemon (the stale-bind-mount
// recreate path, the shared-vs-project destroy split, …).
//
// findContainerFn / findSharedContainerFn take the Manager's NamePrefix as
// first argument so peer daemons running under a different prefix never see
// each other's containers in `docker ps --filter`.
var (
	startContainerFn      = StartContainer
	stopContainerFn       = StopContainer
	removeContainerFn     = RemoveContainer
	createContainerFn     = CreateContainer
	findContainerFn       = FindContainer
	findSharedContainerFn = FindSharedContainer
	imageEnvFn            = ImageEnv
	runPostCreateFn       = RunPostCreate
)

// ContainerState holds runtime data for one project's devcontainer.
type ContainerState struct {
	mu          sync.Mutex
	containerID string
	spec        *DevcontainerSpec
	refCount    int
}

// WorkspaceFolder returns the container-absolute workspace path from the spec,
// or "" if the spec is not available. spec is immutable post-construction.
func (cs *ContainerState) WorkspaceFolder() string {
	if cs == nil || cs.spec == nil {
		return ""
	}
	return cs.spec.WorkspaceFolder
}

// WorkspaceTarget returns the effective container-side workspace path, falling
// back to /workspaces/<basename> when workspaceFolder is absent from devcontainer.json.
// Use this for pathmap registration so the mount covers the actual container cwd.
func (cs *ContainerState) WorkspaceTarget() string {
	if cs == nil || cs.spec == nil {
		return ""
	}
	return cs.spec.WorkspaceTarget()
}

func (cs *ContainerState) BindMounts() []BindMount {
	if cs == nil || cs.spec == nil {
		return nil
	}
	return cs.spec.BindMounts()
}

// IsShared reports whether this container state belongs to the shared container.
func (cs *ContainerState) IsShared() bool {
	return cs != nil && cs.spec != nil && cs.spec.Isolation == IsolationShared
}

// isolation reconstructs the IsolationPlan from the materialized spec, whose
// Isolation is stamped from opts.Isolation.Kind at create time. It lets teardown
// route its containers-map key through the same IsolationPlan.ContainerKey the
// create path used, instead of re-deriving the shared sentinel by hand.
func (cs *ContainerState) isolation() sandbox.IsolationPlan {
	if cs == nil || cs.spec == nil {
		return sandbox.IsolationPlan{}
	}
	return sandbox.IsolationPlan{Kind: cs.spec.Isolation}
}

func (cs *ContainerState) ContainerID() string {
	if cs == nil {
		return ""
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.containerID
}

// PreExec returns the devcontainer.json `preExecCommand` (or the spec
// fallback). Callers that build their own docker exec command (bypassing
// BuildLaunchCommand) need this to wrap their command with the same
// `<login-shell> -lc 'preExec; exec ...'` envelope the pane uses, otherwise
// binaries that depend on the shell init (mise shims, tool-version managers,
// env loaders) won't see the expected setup.
func (cs *ContainerState) PreExec() string {
	if cs == nil || cs.spec == nil {
		return ""
	}
	return cs.spec.PreExec
}

func (cs *ContainerState) EffectiveUser() string {
	if cs == nil || cs.spec == nil {
		return ""
	}
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.spec.EffectiveUser()
}

// Manager implements sandbox.Manager[*ContainerState] using direct docker commands.
// Reactor does not build images; the image name is read from devcontainer.json (image: or build.name).
type Manager struct {
	overlayFn  OverlayFunc
	namePrefix string // injected into every spec; "" → DefaultNamePrefix
	mu         sync.Mutex
	inflight   singleflight.Group
	containers map[string]*ContainerState // key = projectPath
}

// New returns a Manager whose containers and labels use the default prefix.
// overlayFn may be nil. Equivalent to NewWithPrefix(overlayFn, "").
func New(overlayFn OverlayFunc) *Manager {
	return NewWithPrefix(overlayFn, "")
}

// NewWithPrefix returns a Manager that uses `namePrefix` for container names
// and label keys. An empty prefix falls back to DefaultNamePrefix. A daemon
// running side-by-side with another arc daemon (e.g. scripts/run-dev.sh next
// to the user's TUI daemon) MUST configure a distinct prefix here; otherwise
// the docker container namespace collides across data dirs and mount-hash
// drift detection silently rm's the peer's containers.
func NewWithPrefix(overlayFn OverlayFunc, namePrefix string) *Manager {
	return &Manager{
		overlayFn:  overlayFn,
		namePrefix: namePrefix,
		containers: make(map[string]*ContainerState),
	}
}

// NamePrefix returns the configured prefix, falling back to DefaultNamePrefix.
func (m *Manager) NamePrefix() string {
	if m == nil || m.namePrefix == "" {
		return DefaultNamePrefix
	}
	return m.namePrefix
}

// EnsureInstance ensures the devcontainer for projectPath is running.
// When opts.Isolation is shared, a single shared container is used across projects.
// Returns an error if the image declared in devcontainer.json does not exist locally.
func (m *Manager) EnsureInstance(ctx context.Context, projectPath, _ string, opts sandbox.StartOptions) (*sandbox.Instance[*ContainerState], error) {
	instanceKey := opts.Isolation.ContainerKey(projectPath)
	_, err, _ := m.inflight.Do(instanceKey, func() (any, error) {
		return nil, m.ensureContainer(ctx, instanceKey, projectPath, opts)
	})
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	cs := m.containers[instanceKey]
	m.mu.Unlock()

	image := ""
	if cs != nil && cs.spec != nil {
		image = cs.spec.Image
	}
	return &sandbox.Instance[*ContainerState]{
		ProjectPath: projectPath,
		Image:       image,
		Internal:    cs,
	}, nil
}

func (m *Manager) ensureContainer(ctx context.Context, instanceKey, projectPath string, opts sandbox.StartOptions) error {
	m.mu.Lock()
	_, exists := m.containers[instanceKey]
	m.mu.Unlock()
	if exists {
		return nil
	}

	var (
		ctr *ContainerInfo
		err error
	)

	t := time.Now()
	findCtx, findCancel := context.WithTimeout(ctx, 5*time.Second)
	prefix := m.NamePrefix()
	if opts.Isolation.IsShared() {
		ctr, err = findSharedContainerFn(findCtx, prefix)
	} else {
		ctr, err = findContainerFn(findCtx, prefix, projectPath)
	}
	findCancel()
	slog.Info("devcontainer: stage", "name", "find", "elapsed", time.Since(t), "project", projectPath, "shared", opts.Isolation.IsShared(), "prefix", prefix)
	if err != nil {
		return fmt.Errorf("devcontainer: find container: %w", err)
	}

	dcPath, err := resolveDCPath(projectPath, opts)
	if err != nil {
		return err
	}

	t = time.Now()
	spec, err := m.loadSpec(opts.Isolation, projectPath, filepath.Dir(dcPath))
	if err != nil {
		return err
	}
	spec.Isolation = opts.Isolation.Kind
	slog.Info("devcontainer: stage", "name", "load_spec", "image", spec.Image, "elapsed", time.Since(t), "project", projectPath)

	image := spec.Image
	imgEnv, err := imageEnvFn(ctx, image)
	if err != nil {
		slog.Warn("devcontainer: image env probe failed; resolving without image baseline",
			"image", image, "err", err)
		imgEnv = map[string]string{}
	}
	spec.ResolveContainerEnvPlaceholders(imgEnv)

	ctr, err = discardContainerIfStale(ctx, ctr, instanceKey, spec, opts)
	if err != nil {
		return err
	}

	if ctr != nil {
		recovered, err := m.tryReuseElseRecreate(ctx, instanceKey, ctr, spec)
		if err != nil {
			return err
		}
		if !recovered {
			return nil
		}
	}
	return m.createContainer(ctx, instanceKey, image, spec)
}

// discardContainerIfStale removes an existing container when a cold-start or
// shared-mode mount-hash mismatch requires a fresh container, returning nil
// so the caller proceeds to createContainer.
func discardContainerIfStale(ctx context.Context, ctr *ContainerInfo, instanceKey string, spec *DevcontainerSpec, opts sandbox.StartOptions) (*ContainerInfo, error) {
	if ctr != nil && opts.ColdStart {
		slog.Info("devcontainer: cold start discarding existing container",
			"id", shortID(ctr.ID), "state", ctr.State, "key", instanceKey)
		rmCtx, rmCancel := context.WithTimeout(ctx, 30*time.Second)
		rmErr := removeContainerFn(rmCtx, ctr.ID)
		rmCancel()
		if rmErr != nil {
			return nil, fmt.Errorf("devcontainer: cold-start remove: %w", rmErr)
		}
		return nil, nil
	}
	// Mount drift recreate applies to BOTH isolation kinds. A project container
	// created before a project-scope sandbox change (e.g. a host_exec overlay
	// mount added when switching shared→project) carries a stale mount-hash label,
	// so reusing it would silently drop the new mounts. A pre-label container
	// reports MountHash "" which mismatches any real hash and recreates once.
	if ctr != nil {
		expected := spec.MountConfigurationHash()
		if ctr.MountHash != expected {
			slog.Info("devcontainer: mount mismatch, recreating container",
				"id", shortID(ctr.ID), "old", ctr.MountHash, "new", expected,
				"key", instanceKey, "shared", opts.Isolation.IsShared())
			rmCtx, rmCancel := context.WithTimeout(ctx, 30*time.Second)
			rmErr := removeContainerFn(rmCtx, ctr.ID)
			rmCancel()
			if rmErr != nil {
				return nil, fmt.Errorf("devcontainer: remove stale container: %w", rmErr)
			}
			return nil, nil
		}
	}
	return ctr, nil
}

// resolveDCPath returns the path to devcontainer.json for the given project and options.
func resolveDCPath(projectPath string, opts sandbox.StartOptions) (string, error) {
	dcDir := opts.Isolation.DevcontainerDir
	if opts.Isolation.IsShared() {
		if dcDir != "" {
			p := filepath.Join(dcDir, "devcontainer.json")
			if _, statErr := os.Stat(p); statErr != nil {
				return "", fmt.Errorf("devcontainer: shared devcontainer path %q: devcontainer.json not found", dcDir)
			}
			return p, nil
		}
		dcPath, err := UserBaseDC()
		if err != nil {
			return "", fmt.Errorf("devcontainer: %w", err)
		}
		return dcPath, nil
	}
	dcPath, err := FindDevcontainerPath(projectPath, dcDir)
	if err != nil {
		return "", fmt.Errorf("devcontainer: %w", err)
	}
	return dcPath, nil
}

// tryReuseElseRecreate attempts to reuse the existing container. If reuse
// fails because of a stale Docker Desktop bind-mount cache (file mounts like
// ~/.claude.json silently dropping their source path), it removes the broken
// container so the caller can recreate from scratch. Returns:
//   - (false, nil): reuse succeeded, no recreate needed
//   - (true,  nil): container was removed; caller must call createContainer
//   - (_,   error): reuse failed for an unrelated reason — propagate
//
// Image layers and the mount-hash label survive the remove; the only state
// lost is what already lived in the dead container layer. Host bind-mounts
// (~/.claude/sessions, ~/.codex/sessions) are untouched, so claude/codex
// session resume still works after the recreate.
func (m *Manager) tryReuseElseRecreate(ctx context.Context, instanceKey string, ctr *ContainerInfo, spec *DevcontainerSpec) (recreate bool, err error) {
	if reuseErr := m.reuseContainer(ctx, instanceKey, ctr, spec); reuseErr != nil {
		if !isStaleBindMountError(reuseErr) {
			return false, reuseErr
		}
		slog.Warn("devcontainer: stale bind-mount cache, recreating container",
			"id", shortID(ctr.ID), "key", instanceKey)
		rmCtx, rmCancel := context.WithTimeout(ctx, 30*time.Second)
		rmErr := removeContainerFn(rmCtx, ctr.ID)
		rmCancel()
		if rmErr != nil {
			return false, fmt.Errorf("devcontainer: recover after stale bind-mount: %w (original: %v)", rmErr, reuseErr)
		}
		m.mu.Lock()
		delete(m.containers, instanceKey)
		m.mu.Unlock()
		return true, nil
	}
	return false, nil
}

// isStaleBindMountError detects the specific Docker Desktop failure mode where
// the WSL bind-mount cache has lost a file mount's source path. We only retry
// on this exact pattern so unrelated start failures (image missing, permission
// problems, network) still surface to the user untouched.
func isStaleBindMountError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "OCI runtime create failed") &&
		strings.Contains(msg, "error mounting") &&
		strings.Contains(msg, "no such file or directory")
}

func (m *Manager) loadSpec(plan sandbox.IsolationPlan, projectPath, dcDir string) (*DevcontainerSpec, error) {
	spec, err := LoadSpec(projectPath, dcDir)
	if err != nil {
		return nil, fmt.Errorf("devcontainer: load spec: %w", err)
	}

	if m.overlayFn != nil {
		overlay, err := m.overlayFn(plan, projectPath, dcDir)
		if err != nil {
			return nil, fmt.Errorf("devcontainer: overlay: %w", err)
		}
		spec.Apply(overlay)
	}
	// Inject the Manager's prefix so ContainerName / BuildCreateArgs and the
	// label keys stamped on the new container all come out under this daemon's
	// namespace, never the legacy default.
	spec.NamePrefix = m.namePrefix
	return spec, nil
}

func (m *Manager) reuseContainer(ctx context.Context, instanceKey string, ctr *ContainerInfo, spec *DevcontainerSpec) error {
	if ctr.State != "running" {
		slog.Info("devcontainer: starting existing container", "id", shortID(ctr.ID), "state", ctr.State, "key", instanceKey)
		t := time.Now()
		startCtx, startCancel := context.WithTimeout(ctx, 30*time.Second)
		err := startContainerFn(startCtx, ctr.ID)
		startCancel()
		slog.Info("devcontainer: stage", "name", "start_existing", "elapsed", time.Since(t), "key", instanceKey)
		if err != nil {
			slog.Error("devcontainer: container start failed, manual recovery required",
				"id", shortID(ctr.ID), "key", instanceKey,
				"hint", "if Docker Desktop bind-mount cache is stale, run `docker rm -f "+ctr.ID+"` and restart arc",
				"err", err)
			return fmt.Errorf("devcontainer: %w", err)
		}
	} else {
		slog.Info("devcontainer: reusing running container", "id", shortID(ctr.ID), "key", instanceKey)
	}

	m.mu.Lock()
	m.containers[instanceKey] = &ContainerState{containerID: ctr.ID, spec: spec}
	m.mu.Unlock()
	return nil
}

func (m *Manager) createContainer(ctx context.Context, instanceKey, image string, spec *DevcontainerSpec) error {
	containerID, err := m.createAndStart(ctx, instanceKey, image, spec)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.containers[instanceKey] = &ContainerState{containerID: containerID, spec: spec}
	m.mu.Unlock()

	m.runPostCreate(containerID, spec)
	slog.Info("devcontainer: container ready", "id", shortID(containerID), "key", instanceKey)
	return nil
}

func (m *Manager) createAndStart(ctx context.Context, instanceKey, image string, spec *DevcontainerSpec) (string, error) {
	createArgs := spec.BuildCreateArgs(image)
	slog.Info("devcontainer: creating container", "image", image, "key", instanceKey)
	t := time.Now()
	createCtx, createCancel := context.WithTimeout(ctx, 30*time.Second)
	containerID, err := createContainerFn(createCtx, createArgs)
	createCancel()
	slog.Info("devcontainer: stage", "name", "create", "elapsed", time.Since(t), "key", instanceKey)
	if err != nil {
		return "", fmt.Errorf("devcontainer: %w", err)
	}

	slog.Info("devcontainer: starting container", "id", shortID(containerID), "key", instanceKey)
	t = time.Now()
	startCtx, startCancel := context.WithTimeout(ctx, 30*time.Second)
	err = startContainerFn(startCtx, containerID)
	startCancel()
	slog.Info("devcontainer: stage", "name", "start_new", "elapsed", time.Since(t), "key", instanceKey)
	if err != nil {
		return "", fmt.Errorf("devcontainer: %w", err)
	}
	return containerID, nil
}

func (m *Manager) runPostCreate(containerID string, spec *DevcontainerSpec) {
	if len(spec.PostCreate) == 0 && len(spec.ExtraPostCreate) == 0 {
		return
	}
	readyCtx, readyCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer readyCancel()
	if err := waitForContainer(readyCtx, containerID); err != nil {
		slog.Warn("devcontainer: container not ready for exec, skipping postCreate", "err", err)
		return
	}
	pcCtx, pcCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer pcCancel()
	user := spec.EffectiveUser()
	runPostCreateFn(pcCtx, containerID, user, spec.PostCreate)
	for _, argv := range spec.ExtraPostCreate {
		RunPostCreate(pcCtx, containerID, user, argv)
	}
}

// BuildLaunchCommand generates a "docker exec" command to run plan inside inst.
// frameCtx carries per-frame values (workDir, env) the launcher resolved at
// launch time; in shared mode this is the only path by which per-frame state
// reaches docker exec (the container-scoped spec stays user-scope only).
func (m *Manager) BuildLaunchCommand(inst *sandbox.Instance[*ContainerState], launchSpec sandbox.LaunchSpec, frameCtx sandbox.FrameContext, env map[string]string) (string, map[string]string, error) {
	cs := inst.Internal
	if cs == nil {
		return "", nil, fmt.Errorf("devcontainer: nil ContainerState for %s", inst.ProjectPath)
	}

	cs.mu.Lock()
	containerID := cs.containerID
	spec := cs.spec
	cs.mu.Unlock()

	workDir := resolveWorkDir(spec, frameCtx.WorkDir, launchSpec.StartDir, inst.ProjectPath)

	command := launchSpec.Command
	if command == "shell" {
		command = "sh -c " + shellEscape("exec "+shellalias.LoginShellCommand+" -l")
	}
	if spec.PreExec != "" {
		inner := shellEscape(spec.PreExec + "; exec " + command)
		command = "sh -c " + shellEscape("exec "+shellalias.LoginShellCommand+" -lc "+inner)
	}

	var sb strings.Builder
	// -t (pseudo-TTY) only when the consumer drives an interactive terminal
	// (backend pane). Piped/headless consumers (orchestrator JSON-RPC stdio) must
	// omit it: `docker exec -t` aborts when stdin is not a terminal.
	sb.WriteString("docker exec -i")
	if launchSpec.TTY {
		sb.WriteString("t")
	}
	if u := spec.EffectiveUser(); u != "" {
		sb.WriteString(" -u ")
		sb.WriteString(shellEscape(u))
	}
	sb.WriteString(" -w ")
	sb.WriteString(shellEscape(workDir))
	// docker exec -e accepts repeated KEY=VAL; the last occurrence wins.
	// Emit in priority order: spec (container default) → frameCtx (per-frame
	// credential) → env (driver-specific token). Keys are sorted within each
	// source to keep test output deterministic.
	writeEnv := func(m map[string]string) {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(" -e ")
			sb.WriteString(shellEscape(k + "=" + m[k]))
		}
	}
	writeEnv(spec.RemoteEnv)
	writeEnv(frameCtx.Env)
	writeEnv(env)
	sb.WriteString(" ")
	sb.WriteString(containerID)
	sb.WriteString(" ")
	sb.WriteString(command)

	return sb.String(), map[string]string{}, nil
}

// AcquireFrame increments the ref-count for the instance.
func (m *Manager) AcquireFrame(inst *sandbox.Instance[*ContainerState]) {
	cs := inst.Internal
	cs.mu.Lock()
	cs.refCount++
	cs.mu.Unlock()
}

// ReleaseFrame decrements the ref-count. Returns true when count drops to zero.
func (m *Manager) ReleaseFrame(inst *sandbox.Instance[*ContainerState]) bool {
	cs := inst.Internal
	cs.mu.Lock()
	cs.refCount--
	zero := cs.refCount <= 0
	cs.mu.Unlock()
	return zero
}

// DestroyInstance handles end-of-life for an instance. Shutdown semantics:
// all sandbox resources are released, so containers (shared and project alike)
// are removed (docker rm -f). Cold start then always provisions a fresh
// container — image layer cache survives, but the running container, mounts,
// and in-container daemons (sockbridge, etc.) are gone. Detach uses a
// different code path (EffDetachClient) and never reaches this function, so
// warm restart keeps the container intact.
func (m *Manager) DestroyInstance(ctx context.Context, inst *sandbox.Instance[*ContainerState]) error {
	cs := inst.Internal

	instanceKey := cs.isolation().ContainerKey(inst.ProjectPath)

	m.mu.Lock()
	delete(m.containers, instanceKey)
	m.mu.Unlock()

	cs.mu.Lock()
	id := cs.containerID
	cs.mu.Unlock()
	if id == "" {
		return nil
	}

	slog.Info("devcontainer: removing container",
		"id", shortID(id), "project", inst.ProjectPath, "shared", cs.IsShared())
	rmCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return removeContainerFn(rmCtx, id)
}

// resolveWorkDir returns the container-side working directory for a launch.
// Shared containers have no canonical project root, so the caller is expected
// to have already translated plan.StartDir to a container path via pathmap;
// it is used as-is. Project containers fall back to translateWorkDir, which
// maps a host path under projectPath into the container workspace.
// resolveWorkDir returns the container-side cwd for docker exec.
// Priority: frameCtx WorkDir (already container-side, resolved via pathmap by
// the launcher) → shared-mode plan.StartDir → translateWorkDir (project mode).
func resolveWorkDir(spec *DevcontainerSpec, ctxWorkDir, planStartDir, projectPath string) string {
	if ctxWorkDir != "" {
		return ctxWorkDir
	}
	if spec.Isolation == IsolationShared {
		if planStartDir != "" {
			return planStartDir
		}
		return spec.WorkspaceTarget()
	}
	return translateWorkDir(planStartDir, projectPath, spec.WorkspaceTarget())
}

// translateWorkDir maps a host-side launch directory to its container-side
// equivalent under remoteWorkspace. Paths outside projectPath fall back to remoteWorkspace.
func translateWorkDir(hostStartDir, projectPath, remoteWorkspace string) string {
	if hostStartDir == "" || hostStartDir == projectPath {
		return remoteWorkspace
	}
	rel, err := filepath.Rel(projectPath, hostStartDir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return remoteWorkspace
	}
	return path.Join(remoteWorkspace, rel)
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
