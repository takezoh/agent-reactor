package stream

import (
	"context"
	"testing"

	"github.com/takezoh/agent-roost/state"
)

func TestFactoryMakeIDDistinguishesSandboxMode(t *testing.T) {
	// Auto (= containerized) and Host overrides must produce different IDs so
	// the runtime can keep one app-server per environment. Container-mode IDs
	// are keyed by container; host-mode IDs by project.
	f := &Factory{cfg: FactoryConfig{
		IsContainer: func(string) bool { return true },
		RunDirKey:   func(p string) string { return p },
	}}
	autoID := f.makeID("/repo", state.SandboxOverrideAuto)
	hostID := f.makeID("/repo", state.SandboxOverrideHost)
	if autoID == hostID {
		t.Fatalf("auto and host IDs collided: %q", autoID)
	}
	if want := state.SubsystemID("stream:container:/repo"); autoID != want {
		t.Errorf("autoID = %q, want %q", autoID, want)
	}
	if want := state.SubsystemID("stream:host:/repo"); hostID != want {
		t.Errorf("hostID = %q, want %q", hostID, want)
	}
}

func TestFactoryMakeIDEscapesColons(t *testing.T) {
	// ":" inside project paths would corrupt the "stream:<kind>:<key>" wire
	// format; replace with "_" so the ID stays parseable.
	f := &Factory{cfg: FactoryConfig{
		IsContainer: func(string) bool { return true },
		RunDirKey:   func(p string) string { return p },
	}}
	id := f.makeID("/repo:weird", state.SandboxOverrideAuto)
	if want := state.SubsystemID("stream:container:/repo_weird"); id != want {
		t.Errorf("id = %q, want %q", id, want)
	}
}

// Regression for "codex frame dies with 'failed to connect to remote app
// server' in shared mode" — the host-side codex app-server / sockbridge pair
// can only support one backend per container, so subsystem IDs from frames
// inside the same shared container must collapse onto a single ID. Currently
// the Factory keys IDs by project path, so two projects in one shared
// container create two backends that race for the same host socket; the loser
// dies on bind and its frame can't reach the bridge.
//
// This test pins the desired key: stream:container:<RunDirKey(project)>.
// RunDirKey returns "__shared__" for shared mode and the project path for
// per-project devcontainers, matching DevcontainerLauncher.RunDirKey.
func TestFactoryMakeID_SharedContainerCollapsesProjects(t *testing.T) {
	f := &Factory{cfg: FactoryConfig{
		IsContainer: func(string) bool { return true },
		RunDirKey:   func(string) string { return "__shared__" },
	}}
	idA := f.makeID("/workspace/agent-roost", state.SandboxOverrideAuto)
	idB := f.makeID("/workspace/fintech", state.SandboxOverrideAuto)
	if idA != idB {
		t.Fatalf("shared container: IDs must collapse to one; got %q vs %q", idA, idB)
	}
	if want := state.SubsystemID("stream:container:__shared__"); idA != want {
		t.Errorf("shared container ID = %q, want %q", idA, want)
	}
}

func TestFactoryMakeID_ProjectContainerKeyedByContainer(t *testing.T) {
	// project-isolation devcontainers: each container has its own
	// RunDirKey == projectPath, so IDs stay separate. This is the legacy
	// per-project behavior, just routed through the container key explicitly.
	f := &Factory{cfg: FactoryConfig{
		IsContainer: func(string) bool { return true },
		RunDirKey:   func(p string) string { return p },
	}}
	idA := f.makeID("/workspace/a", state.SandboxOverrideAuto)
	idB := f.makeID("/workspace/b", state.SandboxOverrideAuto)
	if idA == idB {
		t.Fatalf("project-isolation containers must stay separate; got identical %q", idA)
	}
	if want := state.SubsystemID("stream:container:/workspace/a"); idA != want {
		t.Errorf("project container ID = %q, want %q", idA, want)
	}
}

func TestFactoryMakeID_HostStaysPerProject(t *testing.T) {
	// Host-mode launches still need a per-project key — each host project
	// runs its own codex app-server in its own cwd, so collapsing them
	// would mix unrelated threads.
	f := &Factory{cfg: FactoryConfig{
		IsContainer: func(string) bool { return false },
	}}
	idA := f.makeID("/workspace/a", state.SandboxOverrideAuto)
	idB := f.makeID("/workspace/b", state.SandboxOverrideAuto)
	if idA == idB {
		t.Errorf("host mode: per-project IDs expected; got identical %q", idA)
	}
	if want := state.SubsystemID("stream:host:/workspace/a"); idA != want {
		t.Errorf("host ID = %q, want %q", idA, want)
	}
}

// Spec: 「コンテナで App Server は 1 つ」「shared モードで複数プロジェクトが
// 1 つの App Server を共有」「frame は 1 つの App Server に接続して thread を
// 取得する」。Factory.Ensure を異なるプロジェクトから呼んでも、同じ shared
// container に属する限り、同じ Backend インスタンス (= 同じ app-server
// プロセス、同じ WebSocket 接続) が返ることを保証する。
//
// Backend.Start は WebSocket dial を行うので、テストは予めキャッシュに
// sentinel Backend を入れて Ensure のキャッシュヒット経路だけを検証する。
// これにより Ensure が異なる project に対しても「同じ ID → 同じ Backend」
// を返す契約を pin できる。
func TestFactory_EnsureSharesBackendAcrossProjectsInSharedContainer(t *testing.T) {
	f := NewFactory(FactoryConfig{
		IsContainer: func(string) bool { return true },
		RunDirKey:   func(string) string { return "__shared__" },
	})
	sharedID := state.SubsystemID("stream:container:__shared__")
	sentinel := &Backend{subsystemID: sharedID}
	f.backends[sharedID] = sentinel

	plan := state.LaunchPlan{Command: "codex"}
	subA, idA, errA := f.Ensure(context.Background(), "/workspace/agent-roost", plan)
	if errA != nil {
		t.Fatalf("Ensure A: %v", errA)
	}
	subB, idB, errB := f.Ensure(context.Background(), "/workspace/fintech", plan)
	if errB != nil {
		t.Fatalf("Ensure B: %v", errB)
	}
	if idA != sharedID || idB != sharedID {
		t.Errorf("subsystem IDs not collapsed: A=%q B=%q want=%q", idA, idB, sharedID)
	}
	if subA != sentinel || subB != sentinel {
		t.Errorf("Ensure returned different Backend instances — app-server is duplicated")
	}
	if got := len(f.backends); got != 1 {
		t.Errorf("backend count = %d, want 1 (one app-server per shared container)", got)
	}
}

// Spec: project-isolation devcontainer はプロジェクトごとに別 container を
// 持つので、Backend (= app-server) もプロジェクトごとに 1 つ。Ensure は
// 同じ Factory から呼ばれても異なる Backend インスタンスを返さなければ
// ならない。
func TestFactory_EnsureKeepsSeparateBackendsForProjectMode(t *testing.T) {
	f := NewFactory(FactoryConfig{
		IsContainer: func(string) bool { return true },
		RunDirKey:   func(p string) string { return p },
	})
	idA := state.SubsystemID("stream:container:/workspace/a")
	idB := state.SubsystemID("stream:container:/workspace/b")
	backendA := &Backend{subsystemID: idA}
	backendB := &Backend{subsystemID: idB}
	f.backends[idA] = backendA
	f.backends[idB] = backendB

	plan := state.LaunchPlan{Command: "codex"}
	subA, gotIDA, errA := f.Ensure(context.Background(), "/workspace/a", plan)
	if errA != nil {
		t.Fatalf("Ensure A: %v", errA)
	}
	subB, gotIDB, errB := f.Ensure(context.Background(), "/workspace/b", plan)
	if errB != nil {
		t.Fatalf("Ensure B: %v", errB)
	}
	if gotIDA == gotIDB {
		t.Fatalf("project-mode IDs collapsed: %q", gotIDA)
	}
	if subA != backendA {
		t.Errorf("project A: got different Backend — must reuse the per-project one")
	}
	if subB != backendB {
		t.Errorf("project B: got different Backend — must reuse the per-project one")
	}
	if subA == subB {
		t.Errorf("project mode: A and B returned the same Backend; app-server isolation broken")
	}
}

// Spec: 同一 Backend に対する複数 frame は thread を別々に持つ。
// BindFrame が同じ Backend に新規 frame を登録するごとに frame binding が
// 1 つずつ追加される (thread は ResumeThreadID が空のとき空文字で記録、
// session_ready event で後から確定する)。複数 frame が同じ app-server に
// 集約される共有構造の不変条件をここで pin する。
func TestBackend_BindThreadRegistersMultipleFrameBindings(t *testing.T) {
	b := New(nil, "stream:container:__shared__", "/workspace/agent-roost",
		"codex", nil, "", false, false,
		"/tmp/codex.sock", "/opt/roost/run/codex.sock", LoopbackPort,
		func() state.FrameID { return "" },
	)

	frameA := state.FrameID("frame-a")
	frameB := state.FrameID("frame-b")

	// startTurn 呼び出しをスキップするため、ResumeThreadID 経路ではなく
	// 直接 frames map に登録するヘルパーを使う想定だが、本テストでは
	// 「frames map に 2 つの binding が共存できる」ことを直接検証する。
	b.frames[frameA] = &frameBinding{frameID: frameA, threadID: "thread-a"}
	b.threads["thread-a"] = frameA
	b.frames[frameB] = &frameBinding{frameID: frameB, threadID: "thread-b"}
	b.threads["thread-b"] = frameB

	if got := len(b.frames); got != 2 {
		t.Fatalf("frame bindings = %d, want 2 (one app-server, two frames)", got)
	}
	if b.frameForThread("thread-a") != frameA {
		t.Errorf("thread-a → frame mapping lost")
	}
	if b.frameForThread("thread-b") != frameB {
		t.Errorf("thread-b → frame mapping lost")
	}

	// ReleaseFrame は一方のみを解放し、もう一方の binding は維持されること
	// (= app-server は他の frame のために生き続ける) を保証する。
	b.ReleaseFrame(frameA)
	if _, exists := b.frames[frameA]; exists {
		t.Errorf("released frameA still in frames map")
	}
	if _, exists := b.threads["thread-a"]; exists {
		t.Errorf("released frameA's thread-a still in threads map")
	}
	if _, exists := b.frames[frameB]; !exists {
		t.Errorf("frameB was unexpectedly removed when releasing frameA")
	}
	if b.frameForThread("thread-b") != frameB {
		t.Errorf("frameB → thread-b mapping lost after releasing A")
	}
}

func TestFactoryMakeID_HostOverrideAlwaysHostKind(t *testing.T) {
	// SandboxOverrideHost is the per-frame "use host" escape hatch even when
	// the project would otherwise run in a container. It must short-circuit
	// to host kind regardless of IsContainer.
	f := &Factory{cfg: FactoryConfig{
		IsContainer: func(string) bool { return true },
		RunDirKey:   func(string) string { return "__shared__" },
	}}
	id := f.makeID("/workspace/p", state.SandboxOverrideHost)
	if want := state.SubsystemID("stream:host:/workspace/p"); id != want {
		t.Errorf("host override: id = %q, want %q", id, want)
	}
}
