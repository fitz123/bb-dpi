.PHONY: deploy update backup verify list help publish-bundle publish-status test-publish-bundle test-cover-fingerprint

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
	@echo "  help                      - Show this help"
	@echo ""
	@echo "User management:"
	@echo "  ./scripts/xray-users add \"Name\""
	@echo "  ./scripts/xray-users url \"Name\""
	@echo "  ./scripts/xray-users remove \"Name\""
