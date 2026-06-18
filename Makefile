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

CODEX_SCHEMA_DIR := $(SRC_DIR)/platform/agent/codexschema
CODEX_SCHEMA_TMP := /tmp/codex-schema-gen

.PHONY: build build-orchestrator build-claude-app-server build-server build-web build-all \
        run-dev build-experimental install clean test vet lint verify-bridge-deps \
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

# build-web builds the web-client host (cmd/web): serves the browser UI and
# reverse-proxies /api and /ws to the backend.
build-web:
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

test:
	cd $(SRC_DIR) && go test ./...

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
