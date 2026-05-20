BINARY           := roost
BRIDGE           := roost-bridge
SOCKBRIDGE       := sockbridge
ORCHESTRATOR     := orchestrator
CLAUDE_APP_SERVER := claude-app-server
NOTIFY_PS1  := notify.ps1
SRC_DIR     := src
INSTALL_DIR    := $(HOME)/.local/bin
LIBEXEC_DIR    := $(HOME)/.local/lib/roost

CODEX_SCHEMA_DIR := $(SRC_DIR)/platform/agent/codexschema
CODEX_SCHEMA_TMP := /tmp/codex-schema-gen

.PHONY: build build-orchestrator build-claude-app-server build-all \
        build-experimental install clean test vet lint verify-bridge-deps \
        codex-schema-update codex-schema-check

build:
	cd $(SRC_DIR) && go build -o ../$(BINARY) ./cmd/roost
	cd $(SRC_DIR) && go build -o ../$(BRIDGE) ./cmd/roost-bridge
	cd $(SRC_DIR) && go build -o ../$(SOCKBRIDGE) github.com/takezoh/credproxy/cmd/sockbridge
	cp $(SRC_DIR)/platform/lib/notify/notify.ps1 ./$(NOTIFY_PS1)

build-orchestrator:
	cd $(SRC_DIR) && go build -o ../$(ORCHESTRATOR) ./cmd/orchestrator

build-claude-app-server:
	cd $(SRC_DIR) && go build -o ../$(CLAUDE_APP_SERVER) ./cmd/claude-app-server

build-all: build build-orchestrator build-claude-app-server

build-experimental:
	cd $(SRC_DIR) && go build -tags experimental -o ../$(BINARY) ./cmd/roost

install: build
	install -d $(INSTALL_DIR) $(LIBEXEC_DIR)
	install -m 755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	install -m 755 $(BRIDGE) $(LIBEXEC_DIR)/$(BRIDGE)
	install -m 755 $(SOCKBRIDGE) $(LIBEXEC_DIR)/$(SOCKBRIDGE)
	install -m 644 $(NOTIFY_PS1) $(LIBEXEC_DIR)/$(NOTIFY_PS1)

test:
	cd $(SRC_DIR) && go test ./...

vet:
	cd $(SRC_DIR) && go vet ./...

lint:
	cd $(SRC_DIR) && go tool golangci-lint run ./...

verify-bridge-deps:
	@echo "Checking that roost-bridge does not import client/state, client/uiproc, or platform/features..."
	@cd $(SRC_DIR) && go list -deps ./cmd/roost-bridge | grep -E 'takezoh/agent-roost/(client/(state|uiproc)|platform/features)$$' && echo "FAIL: bridge imports forbidden packages" && exit 1 || echo "OK: bridge deps are clean"

clean:
	rm -f $(BINARY) $(BRIDGE) $(SOCKBRIDGE) $(ORCHESTRATOR) $(CLAUDE_APP_SERVER) $(NOTIFY_PS1)

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
