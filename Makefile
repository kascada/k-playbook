.DEFAULT_GOAL := help

.PHONY: help push pdf

help: ## Zeigt diese Hilfe an
	@echo "Verfügbare Targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
	@echo ""

push: ## Committet alle Änderungen und pusht ins Remote
	git add -A
	git commit -m "update skills"
	git push

pdf: ## Baut k-playbook.pdf im DIN-A4-Querformat (via Marp CLI in Docker)
	docker run --rm --init -e MARP_USER="$$(id -u):$$(id -g)" \
		-v "$$PWD:/home/marp/app" marpteam/marp-cli \
		k-playbook.md --pdf --allow-local-files
