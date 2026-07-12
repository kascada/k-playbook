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
