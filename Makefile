.PHONY: deploy update backup verify list help publish-bundle publish-status test-publish-bundle test-cover-fingerprint build-bb-vpn-host build-bb-vpn-pkg test-bb-vpn build-pkg

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
# dev/test use. build-bb-vpn-pkg produces a Darwin universal binary
# (arm64 + amd64 via lipo) for the .pkg payload (Phase 4).
#
# Requires `go` >= 1.22 (matches go.mod). Install via 'brew install go'.

GO ?= go

# Version baked into bb-vpn via -ldflags. Default reads bb_vpn from the
# committed package-manifest.json so the host-build matches what the
# next .pkg would ship. Override via `make BB_VPN_VERSION=1.0.1 ...`.
BB_VPN_VERSION ?= $(shell jq -r '.bb_vpn' config/control-plane/package-manifest.json)
LDFLAGS_VERSION := -X main.Version=$(BB_VPN_VERSION)

build-bb-vpn-host:
	@$(GO) version >/dev/null 2>&1 || { echo "go is required (1.22+); install via 'brew install go'. go.mod pins the minimum version"; exit 1; }
	mkdir -p build
	cd client/bb-vpn && $(GO) build -ldflags "$(LDFLAGS_VERSION)" -o ../../build/bb-vpn ./cmd/bb-vpn

build-bb-vpn-pkg:
	@$(GO) version >/dev/null 2>&1 || { echo "go is required (1.22+); install via 'brew install go'. go.mod pins the minimum version"; exit 1; }
	@lipo -info /usr/bin/true >/dev/null 2>&1 || { echo "lipo is required for universal builds (ships with Xcode CLT)"; exit 1; }
	mkdir -p build/pkg
	cd client/bb-vpn && GOOS=darwin GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS_VERSION)" -o ../../build/pkg/bb-vpn.arm64 ./cmd/bb-vpn
	cd client/bb-vpn && GOOS=darwin GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS_VERSION)" -o ../../build/pkg/bb-vpn.amd64 ./cmd/bb-vpn
	lipo -create -output build/pkg/bb-vpn build/pkg/bb-vpn.arm64 build/pkg/bb-vpn.amd64
	rm build/pkg/bb-vpn.arm64 build/pkg/bb-vpn.amd64
	@file build/pkg/bb-vpn

test-bb-vpn:
	@$(GO) version >/dev/null 2>&1 || { echo "go is required (1.22+); install via 'brew install go'. go.mod pins the minimum version"; exit 1; }
	cd client/bb-vpn && $(GO) test ./...

# Phase 4 of pkg-and-pull-control-plane: assemble the BB-VPN macOS
# installer .pkg. Calls build-bb-vpn-pkg first to refresh the
# universal binary, then hands off to client/pkg-build/build.sh.
# Operator must drop sing-box + xray binaries into
# client/pkg-build/payload-binaries/ first; see client/pkg-build/README.md.
build-pkg: build-bb-vpn-pkg
	./client/pkg-build/build.sh

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
	@echo "Client bb-vpn (Phase 2):"
	@echo "  build-bb-vpn-host         - Build bb-vpn for host arch -> build/bb-vpn"
	@echo "  build-bb-vpn-pkg          - Build Darwin universal binary -> build/pkg/bb-vpn"
	@echo "  test-bb-vpn               - Run client/bb-vpn Go tests"
	@echo ""
	@echo ".pkg installer (Phase 4):"
	@echo "  build-pkg                 - Assemble BB-VPN-<ver>.pkg in client/pkg-build/dist/"
	@echo ""
	@echo "  help                      - Show this help"
	@echo ""
	@echo "User management:"
	@echo "  ./scripts/xray-users add \"Name\""
	@echo "  ./scripts/xray-users url \"Name\""
	@echo "  ./scripts/xray-users remove \"Name\""
