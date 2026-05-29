package runtime

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	rsubsystem "github.com/takezoh/agent-roost/client/runtime/subsystem"
	"github.com/takezoh/agent-roost/client/state"
	"github.com/takezoh/agent-roost/platform/agentlaunch"
	platformconfig "github.com/takezoh/agent-roost/platform/config"
	"github.com/takezoh/agent-roost/platform/sandbox"
	sandboxdc "github.com/takezoh/agent-roost/platform/sandbox/devcontainer"
)

// Frame-launch matrix driven through the REAL launcher stack
// (NewDispatcherAdapter → SandboxDispatcher → DirectDispatcher /
// DevcontainerLauncher), with only the docker-backed sandbox.Manager faked.
// This pins the launch wiring touched by e41ab1c across:
//
//	env       : host / per-project container / shared container
//	lifecycle : new-session / cold-start / warm-start
//
// What each cell proves:
//   - host    → DirectDispatcher injects ROOST_SOCKET, strips the container
//     token, registers no frame token / mounts / endpoint.
//   - container → wrapLaunchForSpawn generates a bearer token, the launcher
//     hands it to the Manager's BuildLaunchCommand (it rides the docker-exec
//     command, not the tmux spawn env), and the runtime registers token +
//     mounts + endpoint. per-project vs shared differ in the run-dir key
//     (projectPath hash vs SharedContainerKey hash).
//   - the command-execution ORDER is subsystem.Ensure → subsystem.BindFrame →
//     mgr.EnsureInstance → mgr.AcquireFrame → mgr.BuildLaunchCommand →
//     tmux.SpawnWindow.
//
// The subsystem backends are faked (recSubsysFactory) so no real codex
// app-server starts here; the codex command/socket rewrite is covered at the
// stream-package altitude. The roost-bridge binary copy and the container
// workspace bind-mount are covered by platform/agentlaunch tests — here the
// faked Manager returns a nil ContainerState (all its methods are nil-safe).

type envKind int

const (
	envHost envKind = iota
	envProject
	envShared
)

// orderRecorder collects an ordered trace of cross-layer launch calls.
type orderRecorder struct {
	mu sync.Mutex
	ev []string
}

func (o *orderRecorder) add(s string) {
	o.mu.Lock()
	o.ev = append(o.ev, s)
	o.mu.Unlock()
}

func (o *orderRecorder) snapshot() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.ev...)
}

// kindCounter records which subsystem kind was Ensure'd, so a test can assert
// codex→stream / generic→cli selection without running a backend.
type kindCounter struct {
	mu sync.Mutex
	m  map[state.LaunchSubsystem]int
}

func (k *kindCounter) inc(kind state.LaunchSubsystem) {
	k.mu.Lock()
	k.m[kind]++
	k.mu.Unlock()
}

func (k *kindCounter) count(kind state.LaunchSubsystem) int {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.m[kind]
}

// fakeSandboxManager satisfies sandbox.Manager[*sandboxdc.ContainerState] with
// no docker. mockMgr (platform/agentlaunch) is package-local and cannot be
// imported here, so this is a runtime-scope equivalent with a call recorder.
type fakeSandboxManager struct {
	rec          *orderRecorder
	mu           sync.Mutex
	lastBuildEnv map[string]string
	ensureN      int
	acquireN     int
	releaseN     int
	destroyN     int
	inst         *sandbox.Instance[*sandboxdc.ContainerState]
}

func (m *fakeSandboxManager) EnsureInstance(_ context.Context, project, _ string, _ sandbox.StartOptions) (*sandbox.Instance[*sandboxdc.ContainerState], error) {
	m.rec.add("mgr.EnsureInstance")
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureN++
	if m.inst == nil {
		m.inst = &sandbox.Instance[*sandboxdc.ContainerState]{ProjectPath: project, Internal: nil}
	}
	return m.inst, nil
}

func (m *fakeSandboxManager) BuildLaunchCommand(_ *sandbox.Instance[*sandboxdc.ContainerState], spec sandbox.LaunchSpec, _ sandbox.FrameContext, env map[string]string) (string, map[string]string, error) {
	m.rec.add("mgr.BuildLaunchCommand")
	m.mu.Lock()
	m.lastBuildEnv = cloneEnvMap(env, 0)
	m.mu.Unlock()
	// The real Manager bakes env into a `docker exec -e ...` command and returns
	// a separate container-exec env; mirror that shape with a marker env.
	return "docker exec fake " + spec.Command, map[string]string{"CONTAINER": "1"}, nil
}

func (m *fakeSandboxManager) AcquireFrame(_ *sandbox.Instance[*sandboxdc.ContainerState]) {
	m.rec.add("mgr.AcquireFrame")
	m.mu.Lock()
	m.acquireN++
	m.mu.Unlock()
}

func (m *fakeSandboxManager) ReleaseFrame(_ *sandbox.Instance[*sandboxdc.ContainerState]) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.releaseN++
	return true
}

func (m *fakeSandboxManager) DestroyInstance(_ context.Context, _ *sandbox.Instance[*sandboxdc.ContainerState]) error {
	m.mu.Lock()
	m.destroyN++
	m.mu.Unlock()
	return nil
}

func (m *fakeSandboxManager) buildEnv() map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneEnvMap(m.lastBuildEnv, 0)
}

var _ sandbox.Manager[*sandboxdc.ContainerState] = (*fakeSandboxManager)(nil)

// recSubsystem echoes the bind plan and records lifecycle calls.
type recSubsystem struct {
	id   state.SubsystemID
	kind state.LaunchSubsystem
	rec  *orderRecorder
}

func (s *recSubsystem) Kind() state.LaunchSubsystem { return s.kind }
func (s *recSubsystem) Start(context.Context) error { return nil }
func (s *recSubsystem) BindFrame(_ context.Context, req rsubsystem.BindRequest) (rsubsystem.BindResult, error) {
	s.rec.add("subsystem.BindFrame")
	return rsubsystem.BindResult{Plan: req.Plan}, nil
}
func (s *recSubsystem) ReleaseFrame(state.FrameID) {}
func (s *recSubsystem) Stop(context.Context)       {}

type recSubsysFactory struct {
	kind  state.LaunchSubsystem
	rec   *orderRecorder
	kinds *kindCounter
}

func (f *recSubsysFactory) Ensure(_ context.Context, sid state.SessionID, _ string, _ state.LaunchPlan) (rsubsystem.Subsystem, state.SubsystemID, error) {
	f.rec.add("subsystem.Ensure")
	f.kinds.inc(f.kind)
	id := state.SubsystemID(string(f.kind) + ":" + string(sid))
	return &recSubsystem{id: id, kind: f.kind, rec: f.rec}, id, nil
}

// recordingTmux wraps fakeTmuxBackend, appending "tmux.SpawnWindow" to the
// shared order trace before delegating. All other methods are promoted.
type recordingTmux struct {
	*fakeTmuxBackend
	rec *orderRecorder
}

func (t *recordingTmux) SpawnWindow(name, command, startDir string, env map[string]string) (string, string, error) {
	t.rec.add("tmux.SpawnWindow")
	return t.fakeTmuxBackend.SpawnWindow(name, command, startDir, env)
}

// launchHarness wires a Runtime through the real launcher stack for one env.
type launchHarness struct {
	r        *Runtime
	tmux     *fakeTmuxBackend
	mgr      *fakeSandboxManager
	rec      *orderRecorder
	kinds    *kindCounter
	dataDir  string
	sockPath string
}

func newLaunchHarness(t *testing.T, env envKind) *launchHarness {
	return buildLaunchHarness(t, env, false)
}

// buildLaunchHarness wires a Runtime through the real launcher stack. When
// persistWarm is true the runtime's warm-frame store is enabled (Config.DataDir
// set) — only the warm-start test needs it. Cold/new-session tests leave it
// off so registerContainerFrame's fire-and-forget warm Save goroutine cannot
// race t.TempDir cleanup (it would otherwise write into a dir being removed).
func buildLaunchHarness(t *testing.T, env envKind, persistWarm bool) *launchHarness {
	t.Helper()
	rec := &orderRecorder{}
	kinds := &kindCounter{m: map[state.LaunchSubsystem]int{}}
	mgr := &fakeSandboxManager{rec: rec}
	dataDir := t.TempDir()
	sockPath := filepath.Join(dataDir, "roost.sock")

	var user platformconfig.SandboxConfig
	switch env {
	case envHost:
		user = platformconfig.SandboxConfig{Mode: "direct"}
	case envProject:
		user = platformconfig.SandboxConfig{Mode: "devcontainer", Isolation: "project"}
	case envShared:
		user = platformconfig.SandboxConfig{Mode: "devcontainer", Isolation: "shared"}
	}
	resolver := platformconfig.NewSandboxResolver(user)

	disp := &agentlaunch.SandboxDispatcher{
		Resolver: resolver,
		Direct:   agentlaunch.DirectDispatcher{SockPath: sockPath},
	}
	if env != envHost {
		disp.Devcontainer = agentlaunch.NewDevcontainerLauncher(
			mgr, resolver.Resolve, resolver.ResolveProjectScope, nil, dataDir, true)
	}

	base := newFakeTmux()
	cfg := Config{
		SessionName: "roost-test",
		Tmux:        &recordingTmux{fakeTmuxBackend: base, rec: rec},
		Launcher:    NewDispatcherAdapter(disp),
		Persist:     &recordingPersist{},
	}
	if persistWarm {
		cfg.DataDir = dataDir
	}
	r := New(cfg)
	t.Cleanup(r.shutdownContainerEndpoints)
	r.SetSandboxedProjectResolver(func(string) bool { return env != envHost })
	r.subsystemFactories = map[state.LaunchSubsystem]rsubsystem.Factory{
		state.LaunchSubsystemCLI:    &recSubsysFactory{kind: state.LaunchSubsystemCLI, rec: rec, kinds: kinds},
		state.LaunchSubsystemStream: &recSubsysFactory{kind: state.LaunchSubsystemStream, rec: rec, kinds: kinds},
	}
	return &launchHarness{r: r, tmux: base, mgr: mgr, rec: rec, kinds: kinds, dataDir: dataDir, sockPath: sockPath}
}

func (h *launchHarness) spawnEnv(t *testing.T) map[string]string {
	t.Helper()
	h.tmux.mu.Lock()
	defer h.tmux.mu.Unlock()
	if len(h.tmux.spawnEnvs) != 1 {
		t.Fatalf("SpawnWindow env captures = %d, want 1", len(h.tmux.spawnEnvs))
	}
	return h.tmux.spawnEnvs[0]
}

func (h *launchHarness) runDir(project string) string {
	return ProjectRunDir(filepath.Join(h.dataDir, "run"), project)
}

// matrixFrame builds a generic (cli-subsystem) frame for the given project.
// The command is always the zero-behaviour minimal-test driver — codex frames
// are constructed inline where the stream subsystem matters.
func matrixFrame(project string) state.SessionFrame {
	return state.SessionFrame{ID: "f1", Project: project, Command: "minimal-test", Driver: state.DriverStateBase{}}
}

const matrixSizeW, matrixSizeH = 120, 40

func matrixPaneSize() paneSize { return paneSize{width: matrixSizeW, height: matrixSizeH} }

// === cold start (spawnFrameWindow) ===

func TestFrameLaunch_ColdStart_Host(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envHost)

	frame := matrixFrame("/proj/host")
	if err := h.r.spawnFrameWindow("s1", state.SandboxOverrideAuto, frame, matrixPaneSize()); err != nil {
		t.Fatalf("spawnFrameWindow: %v", err)
	}

	env := h.spawnEnv(t)
	if env["ROOST_SOCKET"] != h.sockPath {
		t.Errorf("ROOST_SOCKET = %q, want %q", env["ROOST_SOCKET"], h.sockPath)
	}
	if env["ROOST_SESSION_ID"] != "s1" || env["ROOST_FRAME_ID"] != "f1" {
		t.Errorf("identity env not baked into host spawn: %v", env)
	}
	if _, ok := env["ROOST_SOCKET_TOKEN"]; ok {
		t.Error("host launch must not carry a container bearer token")
	}
	if _, ok := h.r.frameReg.GetMounts("f1"); ok {
		t.Error("host launch must not register mounts")
	}
	if len(h.r.containerEndpoints) != 0 {
		t.Errorf("host launch must not start a container endpoint, got %d", len(h.r.containerEndpoints))
	}
	if h.mgr.ensureN != 0 {
		t.Errorf("host launch must not touch the sandbox Manager, ensureN=%d", h.mgr.ensureN)
	}
}

func TestFrameLaunch_ColdStart_PerProject(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envProject)

	const project = "/proj/box"
	frame := matrixFrame(project)
	if err := h.r.spawnFrameWindow("s1", state.SandboxOverrideAuto, frame, matrixPaneSize()); err != nil {
		t.Fatalf("spawnFrameWindow: %v", err)
	}

	// The bearer token rides the docker-exec command (Manager env), not the
	// tmux spawn env. It must reach BuildLaunchCommand and be registered.
	buildEnv := h.mgr.buildEnv()
	tok := buildEnv["ROOST_SOCKET_TOKEN"]
	if tok == "" {
		t.Fatal("container launch must hand a bearer token to BuildLaunchCommand")
	}
	if buildEnv["ROOST_SESSION_ID"] != "s1" || buildEnv["ROOST_FRAME_ID"] != "f1" {
		t.Errorf("identity env not handed to the container: %v", buildEnv)
	}
	if id, ok := h.r.frameReg.Lookup(tok); !ok || id != "f1" {
		t.Errorf("frameReg.Lookup(token) = (%q, %v), want (f1, true)", id, ok)
	}
	if ms, ok := h.r.frameReg.GetMounts("f1"); !ok || len(ms) == 0 {
		t.Errorf("container run-dir mount not registered: GetMounts = (%v, %v)", ms, ok)
	}
	// per-project run dir is keyed by the project path.
	if _, err := os.Stat(h.runDir(project)); err != nil {
		t.Errorf("per-project run dir not created at %s: %v", h.runDir(project), err)
	}
	if _, ok := h.r.containerEndpoints[project]; !ok {
		t.Error("container launch must start the project endpoint")
	}
}

func TestFrameLaunch_ColdStart_Shared(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envShared)

	const project = "/proj/shared-a"
	frame := matrixFrame(project)
	if err := h.r.spawnFrameWindow("s1", state.SandboxOverrideAuto, frame, matrixPaneSize()); err != nil {
		t.Fatalf("spawnFrameWindow: %v", err)
	}

	// Shared mode keys the run dir by SharedContainerKey, not the project path.
	sharedDir := h.runDir(sandboxdc.SharedContainerKey)
	if _, err := os.Stat(sharedDir); err != nil {
		t.Errorf("shared run dir not created at %s: %v", sharedDir, err)
	}
	if _, err := os.Stat(h.runDir(project)); err == nil {
		t.Errorf("shared mode must not create a per-project run dir at %s", h.runDir(project))
	}
	if h.mgr.buildEnv()["ROOST_SOCKET_TOKEN"] == "" {
		t.Error("shared container launch must still inject a bearer token")
	}
}

// TestFrameLaunch_ColdStart_RecoverableCodexSpawnsResume guards the headline
// regression: a stopped codex frame with a resumable thread is relaunched via
// the stream subsystem; a stopped generic frame (no durable state) is skipped.
func TestFrameLaunch_ColdStart_RecoverableCodexSpawnsResume(t *testing.T) {
	h := newLaunchHarness(t, envHost)
	now := time.Now()
	codex := state.GetDriver("codex")
	generic := state.GetDriver("generic")
	sess := state.Session{
		ID: "s1", Project: "/proj",
		Frames: []state.SessionFrame{
			{ID: "f-codex", Project: "/proj", Command: "codex",
				Driver: codex.Restore(map[string]string{"status": "stopped", "thread_id": codexThreadID}, now)},
			{ID: "f-gen", Project: "/proj", Command: "generic",
				Driver: generic.Restore(map[string]string{"status": "stopped"}, now)},
		},
	}

	if err := h.r.recreateSessionFrames("s1", sess, matrixPaneSize()); err != nil {
		t.Fatalf("recreateSessionFrames: %v", err)
	}

	h.tmux.mu.Lock()
	calls := h.tmux.spawnCalls
	h.tmux.mu.Unlock()
	if calls != 1 {
		t.Fatalf("SpawnWindow calls = %d, want 1 (codex resumed, stopped generic skipped)", calls)
	}
	if _, ok := h.r.sessionPanes["f-codex"]; !ok {
		t.Error("recoverable stopped codex frame must be relaunched on cold start")
	}
	if _, ok := h.r.sessionPanes["f-gen"]; ok {
		t.Error("stopped generic frame must be skipped on cold start")
	}
	if h.kinds.count(state.LaunchSubsystemStream) != 1 {
		t.Errorf("codex must select the stream subsystem, stream ensures = %d", h.kinds.count(state.LaunchSubsystemStream))
	}
}

// TestFrameLaunch_ColdStart_CommandOrder pins the cross-layer launch order.
func TestFrameLaunch_ColdStart_CommandOrder(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envProject)

	frame := matrixFrame("/proj/box")
	if err := h.r.spawnFrameWindow("s1", state.SandboxOverrideAuto, frame, matrixPaneSize()); err != nil {
		t.Fatalf("spawnFrameWindow: %v", err)
	}

	want := []string{
		"subsystem.Ensure",
		"subsystem.BindFrame",
		"mgr.EnsureInstance",
		"mgr.AcquireFrame",
		"mgr.BuildLaunchCommand",
		"tmux.SpawnWindow",
	}
	got := h.rec.snapshot()
	if len(got) != len(want) {
		t.Fatalf("launch order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("launch order = %v, want %v (diverge at %d)", got, want, i)
		}
	}
}

func TestFrameLaunch_ColdStart_SubsystemKindSelection(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envHost)

	if err := h.r.spawnFrameWindow("s1", state.SandboxOverrideAuto, matrixFrame("/proj"), matrixPaneSize()); err != nil {
		t.Fatalf("spawnFrameWindow(minimal): %v", err)
	}
	if h.kinds.count(state.LaunchSubsystemCLI) != 1 || h.kinds.count(state.LaunchSubsystemStream) != 0 {
		t.Errorf("generic/minimal must select cli: cli=%d stream=%d",
			h.kinds.count(state.LaunchSubsystemCLI), h.kinds.count(state.LaunchSubsystemStream))
	}
}

// === new session (spawnTmuxWindow goroutine + handleSpawnComplete loop) ===

func (h *launchHarness) newSessionSpawn(t *testing.T, e state.EffSpawnTmuxWindow) internalSpawnComplete {
	t.Helper()
	internalCh := make(chan internalEvent, 1)
	eventCh := make(chan state.Event, 1)
	deps := spawnDeps{
		tmux:         h.r.cfg.Tmux,
		launcher:     launcher(h.r.cfg),
		factories:    h.r.subsystemFactories,
		sessionName:  h.r.cfg.SessionName,
		mainPaneSize: matrixPaneSize,
		sendInternal: func(ev internalEvent) { internalCh <- ev },
		sendEvent:    func(ev state.Event) { eventCh <- ev },
	}
	spawnTmuxWindow(deps, e)
	select {
	case ev := <-internalCh:
		sc, ok := ev.(internalSpawnComplete)
		if !ok {
			t.Fatalf("expected internalSpawnComplete, got %T", ev)
		}
		h.r.handleSpawnComplete(sc)
		return sc
	case ev := <-eventCh:
		t.Fatalf("spawn reported failure: %T %+v", ev, ev)
		return internalSpawnComplete{}
	}
}

func TestFrameLaunch_NewSession_Host(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envHost)

	sc := h.newSessionSpawn(t, state.EffSpawnTmuxWindow{
		SessionID: "s1", FrameID: "f1", Project: "/proj/host", Command: "minimal-test",
		Env: map[string]string{"ROOST_SESSION_ID": "s1", "ROOST_FRAME_ID": "f1"},
	})

	if sc.token != "" {
		t.Errorf("host spawn must produce an empty token, got %q", sc.token)
	}
	env := h.spawnEnv(t)
	if env["ROOST_SOCKET"] != h.sockPath {
		t.Errorf("ROOST_SOCKET = %q, want %q", env["ROOST_SOCKET"], h.sockPath)
	}
	if _, ok := env["ROOST_SOCKET_TOKEN"]; ok {
		t.Error("host spawn env must not carry a container bearer token")
	}
	if len(h.r.containerEndpoints) != 0 {
		t.Error("host new-session must not start a container endpoint")
	}
}

func TestFrameLaunch_NewSession_PerProject(t *testing.T) {
	registerMinimalDriver(t)
	h := newLaunchHarness(t, envProject)

	const project = "/proj/box"
	sc := h.newSessionSpawn(t, state.EffSpawnTmuxWindow{
		SessionID: "s1", FrameID: "f1", Project: project, Command: "minimal-test",
		Env: map[string]string{"ROOST_SESSION_ID": "s1", "ROOST_FRAME_ID": "f1"},
	})

	if sc.token == "" {
		t.Fatal("container spawn must produce a bearer token")
	}
	if h.mgr.buildEnv()["ROOST_SOCKET_TOKEN"] != sc.token {
		t.Errorf("token handed to BuildLaunchCommand = %q, want %q", h.mgr.buildEnv()["ROOST_SOCKET_TOKEN"], sc.token)
	}
	if id, ok := h.r.frameReg.Lookup(sc.token); !ok || id != "f1" {
		t.Errorf("frameReg.Lookup(token) = (%q, %v), want (f1, true)", id, ok)
	}
	if _, ok := h.r.frameReg.GetMounts("f1"); !ok {
		t.Error("container new-session must register run-dir mounts")
	}
	if _, ok := h.r.containerEndpoints[project]; !ok {
		t.Error("container new-session must start the project endpoint")
	}
}

// === warm start (RecoverSandboxFrames) ===

func TestFrameLaunch_WarmStart_Host(t *testing.T) {
	h := newLaunchHarness(t, envHost)
	h.r.state.Sessions["s1"] = state.Session{
		ID: "s1", Project: "/proj/host",
		Frames: []state.SessionFrame{matrixFrame("/proj/host")},
	}
	h.r.sessionPanes["f1"] = "%1"

	h.r.RecoverSandboxFrames()

	// DirectDispatcher.AdoptFrame returns nil cleanup / nil mounts for host.
	if _, ok := h.r.sandboxCleanups["f1"]; ok {
		t.Error("host adopt returns nil cleanup; nothing should be stored")
	}
	if _, ok := h.r.frameReg.GetMounts("f1"); ok {
		t.Error("host adopt has no mounts to register")
	}
	if len(h.r.containerEndpoints) != 0 {
		t.Error("host warm start must not start a container endpoint")
	}
}

func TestFrameLaunch_WarmStart_PerProject(t *testing.T) {
	h := buildLaunchHarness(t, envProject, true) // warm-frame store needed for token recovery
	const project = "/proj/box"

	// Warm start adopts a pre-running frame; unlike cold start's Wrap it does not
	// create the run dir, so the endpoint socket's parent must already exist.
	if _, err := EnsureProjectRunDir(filepath.Join(h.dataDir, "run"), project); err != nil {
		t.Fatalf("EnsureProjectRunDir: %v", err)
	}
	// A bearer token persisted by the prior daemon run must be re-registered.
	if err := h.r.warmFrames.Save(WarmFrameState{FrameID: "f1", ContainerToken: "warm-tok"}); err != nil {
		t.Fatalf("warm save: %v", err)
	}
	h.r.state.Sessions["s1"] = state.Session{
		ID: "s1", Project: project,
		Frames: []state.SessionFrame{matrixFrame(project)},
	}
	h.r.sessionPanes["f1"] = "%1"

	h.r.RecoverSandboxFrames()

	if id, ok := h.r.frameReg.Lookup("warm-tok"); !ok || id != "f1" {
		t.Errorf("warm token not recovered: Lookup = (%q, %v), want (f1, true)", id, ok)
	}
	if h.mgr.acquireN != 1 {
		t.Errorf("AdoptFrame must AcquireFrame once, acquireN = %d", h.mgr.acquireN)
	}
	if _, ok := h.r.sandboxCleanups["f1"]; !ok {
		t.Error("container adopt cleanup must be stored")
	}
	if ms, ok := h.r.frameReg.GetMounts("f1"); !ok || len(ms) == 0 {
		t.Errorf("adopt mounts not stored: GetMounts = (%v, %v)", ms, ok)
	}
	if _, ok := h.r.containerEndpoints[project]; !ok {
		t.Error("container warm start must restart the project endpoint")
	}
}
