BINARY           := arc
BRIDGE           := reactor-bridge
ORCHESTRATOR     := orchestrator
CLAUDE_APP_SERVER := claude-app-server
SERVER           := server
WEB              := web
NOTIFY_PS1  := notify.ps1
SRC_DIR     := src
INSTALL_DIR    := $(HOME)/.local/bin
LIBEXEC_DIR    := $(HOME)/.local/lib/agent-reactor
SYSTEMD_USER_DIR := $(HOME)/.config/systemd/user

# install-systemd renames the three production binaries to their runtime/server/
# web roles so the systemd unit file ExecStart= lines and `journalctl --user -u
# agent-reactor-…` namespacing read in service vocabulary rather than the
# TUI-era binary name. The binaries themselves are unchanged.
RUNTIME_BIN := agent-reactor-runtime
SERVER_BIN  := agent-reactor-server
WEB_BIN     := agent-reactor-web

CODEX_SCHEMA_DIR := $(SRC_DIR)/platform/agent/codexschema
CODEX_SCHEMA_TMP := /tmp/codex-schema-gen

.PHONY: build build-orchestrator build-claude-app-server build-server build-web build-all \
        build-web-frontend \
        run-dev build-experimental install install-systemd clean test test-race vet lint \
        verify-bridge-deps \
        codex-schema-update codex-schema-check

build:
	cd $(SRC_DIR) && go build -o ../$(BINARY) ./cmd/arc
	cd $(SRC_DIR) && go build -o ../$(BRIDGE) ./cmd/reactor-bridge
	cp $(SRC_DIR)/platform/lib/notify/notify.ps1 ./$(NOTIFY_PS1)

build-orchestrator:
	cd $(SRC_DIR) && go build -o ../$(ORCHESTRATOR) ./cmd/orchestrator

build-claude-app-server:
	cd $(SRC_DIR) && go build -o ../$(CLAUDE_APP_SERVER) ./cmd/claude-app-server

# build-server builds the tmux-free backend (cmd/server): pty session host with a
# REST + WebSocket API that clients connect to. It serves no UI.
build-server:
	cd $(SRC_DIR) && go build -o ../$(SERVER) ./cmd/server

WEB_DIR             := src/client/web
WEB_DIST            := $(WEB_DIR)/dist/index.html
WEB_NPM_STAMP       := $(WEB_DIR)/node_modules/.install-stamp
# -prune skips the descent into node_modules / dist entirely (a post-filter
# would still walk those subtrees — ~50k files for a React app — on every
# make invocation, since $(shell …) evaluates at parse time).
WEB_SRC_FILES       := $(shell find $(WEB_DIR) \
                         \( -path '$(WEB_DIR)/node_modules' -o -path '$(WEB_DIR)/dist' \) -prune \
                         -o -type f -print 2>/dev/null)

# Stamp file owned by us — directory mtime is touched by IDE / npm internals
# and can't be trusted as a "last install" signal.
$(WEB_NPM_STAMP): $(WEB_DIR)/package-lock.json
	cd $(WEB_DIR) && npm ci
	@touch $@

# dist/index.html is the artifact embedded by cmd/web (//go:embed dist).
$(WEB_DIST): $(WEB_NPM_STAMP) $(WEB_SRC_FILES)
	cd $(WEB_DIR) && npm run build

# build-web-frontend: alias for $(WEB_DIST). Stays in the .PHONY list as a
# discoverable target name; the real work lives in the file-target rule above.
build-web-frontend: $(WEB_DIST)

# build-web builds the web-client host (cmd/web): serves the browser UI and
# reverse-proxies /api and /ws to the backend. Depends on $(WEB_DIST) so the
# //go:embed dist directive always picks up fresh CSS / JS.
build-web: $(WEB_DIST)
	cd $(SRC_DIR) && go build -o ../$(WEB) ./cmd/web

build-all: build build-orchestrator build-claude-app-server build-server build-web

# run-dev builds and launches the backend + web-client together for local dev.
run-dev: build-server build-web
	./scripts/run-dev.sh

build-experimental:
	cd $(SRC_DIR) && go build -tags experimental -o ../$(BINARY) ./cmd/arc

install: build
	install -d $(INSTALL_DIR) $(LIBEXEC_DIR)
	install -m 755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	install -m 755 $(BRIDGE) $(LIBEXEC_DIR)/$(BRIDGE)
	install -m 644 $(NOTIFY_PS1) $(LIBEXEC_DIR)/$(NOTIFY_PS1)

# install-systemd installs the three production binaries (runtime, server, web)
# plus the runtime's helper libexec (reactor-bridge + notify.ps1) into the
# user-scope locations consumed by deploy/systemd/agent-reactor-{runtime,server,
# web}.service, and copies those unit files into ~/.config/systemd/user/.
# Binaries are renamed to their service role on disk so unit-file ExecStart=
# lines and journald output stay in runtime/server/web vocabulary.
#
# After `make install-systemd`, run `systemctl --user daemon-reload && \
# systemctl --user enable --now agent-reactor-web.service` to start the stack;
# see docs/user/systemd.md for the full procedure.
#
# The existing `install` target (TUI-only workflow) is intentionally left
# untouched — operators running both can run both targets independently.
install-systemd: build build-server build-web
	install -d $(INSTALL_DIR) $(LIBEXEC_DIR) $(SYSTEMD_USER_DIR)
	install -m 755 $(BINARY) $(INSTALL_DIR)/$(RUNTIME_BIN)
	install -m 755 $(SERVER) $(INSTALL_DIR)/$(SERVER_BIN)
	install -m 755 $(WEB)    $(INSTALL_DIR)/$(WEB_BIN)
	install -m 755 $(BRIDGE) $(LIBEXEC_DIR)/$(BRIDGE)
	install -m 644 $(NOTIFY_PS1) $(LIBEXEC_DIR)/$(NOTIFY_PS1)
	install -m 644 deploy/systemd/agent-reactor-runtime.service $(SYSTEMD_USER_DIR)/
	install -m 644 deploy/systemd/agent-reactor-server.service  $(SYSTEMD_USER_DIR)/
	install -m 644 deploy/systemd/agent-reactor-web.service     $(SYSTEMD_USER_DIR)/
	@echo
	@echo "Installed. Next:"
	@echo "  systemctl --user daemon-reload"
	@echo "  systemctl --user enable --now agent-reactor-web.service"
	@echo "  loginctl enable-linger $$USER   # boot-time autostart"
	@echo "See docs/user/systemd.md for the full guide."

test:
	cd $(SRC_DIR) && go test ./...

# test-race runs the concurrency-sensitive subtrees under the race detector.
# Scoped to platform/termvt (Session actor model + emulator drain) and
# client/runtime (single-dispatcher event loop + ipc fan-out) because the
# rest of the tree adds noise (third-party stubs, large startup paths) we
# haven't validated under -race yet. Add subtrees here as they're audited.
# See docs/agent/testing.md for the rollout plan.
test-race:
	cd $(SRC_DIR) && go test -race -count=1 ./platform/termvt/... ./client/runtime/...

vet:
	cd $(SRC_DIR) && go vet ./...

lint:
	cd $(SRC_DIR) && go tool golangci-lint run ./...

# Opt-in fidelity backstop: routing-isolation invariant against a REAL app-server
# (not codex-only). Configure via REACTOR_E2E_CODEX_BIN and/or
# REACTOR_E2E_APPSERVER_BIN; skips if none set. Validates the in-process fake —
# see docs/technical/client/stream-backend-e2e.md and docs/adr/0002.
test-e2e:
	cd $(SRC_DIR) && go test -tags e2e -run TestStreamRoutingE2E ./client/runtime/subsystem/stream/ -v

verify-bridge-deps:
	@echo "Checking that reactor-bridge does not import client/state, client/uiproc, or platform/features..."
	@cd $(SRC_DIR) && go list -deps ./cmd/reactor-bridge | grep -E 'takezoh/agent-reactor/(client/(state|uiproc)|platform/features)$$' && echo "FAIL: bridge imports forbidden packages" && exit 1 || echo "OK: bridge deps are clean"

clean:
	rm -f $(BINARY) $(BRIDGE) $(ORCHESTRATOR) $(CLAUDE_APP_SERVER) $(NOTIFY_PS1)

# codex-schema-check — verify committed bundle files match current codex output.
# Comparison is done with sorted keys so JSON object ordering doesn't matter.
# Requires codex and jq in PATH (use mise: `mise use codex@0.128.0`).
codex-schema-check:
	@echo "Generating codex JSON Schema into $(CODEX_SCHEMA_TMP)..."
	@rm -rf $(CODEX_SCHEMA_TMP)
	codex app-server generate-json-schema --out $(CODEX_SCHEMA_TMP)
	@echo "Diffing committed bundles against generated output (sorted keys)..."
	jq --sort-keys . $(CODEX_SCHEMA_DIR)/schema/codex_app_server_protocol.schemas.json > /tmp/_schema_committed.json
	jq --sort-keys . $(CODEX_SCHEMA_TMP)/codex_app_server_protocol.schemas.json > /tmp/_schema_generated.json
	diff /tmp/_schema_committed.json /tmp/_schema_generated.json
	jq --sort-keys . $(CODEX_SCHEMA_DIR)/schema/codex_app_server_protocol.v2.schemas.json > /tmp/_schemav2_committed.json
	jq --sort-keys . $(CODEX_SCHEMA_TMP)/codex_app_server_protocol.v2.schemas.json > /tmp/_schemav2_generated.json
	diff /tmp/_schemav2_committed.json /tmp/_schemav2_generated.json
	@echo "OK: schema bundles are in sync with codex 0.128.0"

# codex-schema-update — regenerate pinned schema bundles and Go types.
# Run this when upgrading codex. Requires codex and npx (quicktype) in PATH.
# After running: update the version line in src/platform/agent/codexschema/README.md.
codex-schema-update:
	@echo "Generating codex JSON Schema into $(CODEX_SCHEMA_TMP)..."
	@rm -rf $(CODEX_SCHEMA_TMP)
	codex app-server generate-json-schema --out $(CODEX_SCHEMA_TMP)
	@echo "Copying bundle files..."
	cp $(CODEX_SCHEMA_TMP)/codex_app_server_protocol.schemas.json \
	   $(CODEX_SCHEMA_DIR)/schema/codex_app_server_protocol.schemas.json
	cp $(CODEX_SCHEMA_TMP)/codex_app_server_protocol.v2.schemas.json \
	   $(CODEX_SCHEMA_DIR)/schema/codex_app_server_protocol.v2.schemas.json
	@echo "Regenerating v1 Go types..."
	npx quicktype --lang go --package codexschemav1 --src-lang schema \
	    -o $(CODEX_SCHEMA_DIR)/v1/types.gen.go $(CODEX_SCHEMA_TMP)/v1/*.json
	@echo "Regenerating v2 Go types..."
	npx quicktype --lang go --package codexschemav2 --src-lang schema \
	    -o $(CODEX_SCHEMA_DIR)/v2/types.gen.go $(CODEX_SCHEMA_TMP)/v2/*.json
	@echo "Done. Update the pinned version in $(CODEX_SCHEMA_DIR)/README.md."
