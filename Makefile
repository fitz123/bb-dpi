.PHONY: deploy update backup verify list help

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

help:
	@echo "XRay REALITY Management"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  deploy   - Deploy XRay to server (first time)"
	@echo "  update   - Update XRay to latest version"
	@echo "  backup   - Backup config and secrets"
	@echo "  verify   - Check server connectivity"
	@echo "  list     - List users"
	@echo "  help     - Show this help"
	@echo ""
	@echo "User management:"
	@echo "  ./scripts/xray-users add \"Name\""
	@echo "  ./scripts/xray-users url \"Name\""
	@echo "  ./scripts/xray-users remove \"Name\""
