.PHONY: deploy update backup verify list help publish-bundle publish-status test-publish-bundle test-cover-fingerprint build-bb-vpn-host test-bb-vpn

deploy:
	./scripts/deploy.sh

update:
	./scripts/update.sh

backup:
	./scripts/backup.sh

verify:
	./scripts/verify.sh

list:
	./scripts/xray-users list

# Control-plane targets (Phase 1 of pkg-and-pull-control-plane plan).
# Precondition: dev-machine first-time setup completed
# (config/control-plane/endpoints.json + token minted). See
# docs/control-plane-bootstrap.md.

publish-bundle:
	@test -f config/control-plane/endpoints.json \
	    || { echo "missing config/control-plane/endpoints.json. See docs/control-plane-bootstrap.md."; exit 1; }
	@test -f config/control-plane/token \
	    || { echo "missing config/control-plane/token. See docs/control-plane-bootstrap.md."; exit 1; }
	@test -f config/control-plane/package-manifest.json \
	    || { echo "missing config/control-plane/package-manifest.json (should be committed). See docs/control-plane-bootstrap.md."; exit 1; }
	./scripts/publish-bundle

publish-status:
	./scripts/publish-status

test-publish-bundle:
	./scripts/test-publish-bundle

test-cover-fingerprint:
	./scripts/test-cover-fingerprint

# Phase 2 of pkg-and-pull-control-plane: client/bb-vpn Go binary.
# build-bb-vpn-host produces a host-arch binary at build/bb-vpn for
# dev/test use. Universal binary for the .pkg payload (lipo of
# darwin/amd64 + darwin/arm64) is build-bb-vpn-pkg, added in a later
# Phase 2 PR.
#
# Requires `go` >= 1.22 (matches go.mod). Install via `brew install go`.

GO ?= go

build-bb-vpn-host:
	@$(GO) version >/dev/null 2>&1 || { echo "go is required (1.22+); install via 'brew install go'. go.mod pins the minimum version"; exit 1; }
	mkdir -p build
	cd client/bb-vpn && $(GO) build -o ../../build/bb-vpn ./cmd/bb-vpn

test-bb-vpn:
	@$(GO) version >/dev/null 2>&1 || { echo "go is required (1.22+); install via 'brew install go'. go.mod pins the minimum version"; exit 1; }
	cd client/bb-vpn && $(GO) test ./...

help:
	@echo "XRay REALITY Management"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Server targets:"
	@echo "  deploy                    - Deploy XRay to server (first time)"
	@echo "  update                    - Update XRay to latest version"
	@echo "  backup                    - Backup config and secrets"
	@echo "  verify                    - Check server connectivity"
	@echo "  list                      - List users"
	@echo ""
	@echo "Control plane (Phase 1):"
	@echo "  publish-bundle            - Assemble + publish bundle.json to every endpoint"
	@echo "  publish-status            - Probe each endpoint, report issued_at + sha drift"
	@echo "  test-publish-bundle       - Semantic test: assert allowlist holds (no leaks)"
	@echo "  test-cover-fingerprint    - Probe-fingerprint check vs cover-site 404 baseline"
	@echo ""
	@echo "Client bb-vpn (Phase 2, in progress):"
	@echo "  build-bb-vpn-host         - Build bb-vpn for host arch -> build/bb-vpn"
	@echo "  test-bb-vpn               - Run client/bb-vpn Go tests"
	@echo ""
	@echo "  help                      - Show this help"
	@echo ""
	@echo "User management:"
	@echo "  ./scripts/xray-users add \"Name\""
	@echo "  ./scripts/xray-users url \"Name\""
	@echo "  ./scripts/xray-users remove \"Name\""
