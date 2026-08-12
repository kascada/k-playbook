.DEFAULT_GOAL := help

# Die host-weite Verfügbarkeit richtet das Programm selbst ein: jeder Start der
# Oberfläche spiegelt sich nach ~/.local/share/k-playbook/installation und
# verlinkt nach ~/.local/bin. Das Makefile baut nur.
INSTALLER_BINARY := k-playbook
# Paketpfad relativ zum installer/-Verzeichnis, in dem go build läuft.
INSTALLER_PKG := ./cmd/k-playbook
# Der Wrapper liegt versioniert im Repo, damit direkt nach dem Clone ein
# Einstiegspunkt vorhanden ist. Er wird nicht gebaut.
INSTALLER_WRAPPER := bin/$(INSTALLER_BINARY)
INSTALLER_DIST_DIR := dist
INSTALLER_RELEASE_TARGETS := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64

.PHONY: help build dist gui test installer-build installer-run installer-test

help: ## Zeigt diese Hilfe an
	@echo "Verfügbare Targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-30s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Direkt starten, ohne Go und ohne Installation:"
	@echo "  bin/k-playbook"
	@echo ""
	@echo "Selbst bauen (braucht Go) und starten:"
	@echo "  make dist"
	@echo "  make gui"
	@echo ""
	@echo "Host-weit verfügbar: richtet der erste Start selbst ein."
	@echo "  danach genügt überall:  k-playbook"
	@echo ""

dist: ## Baut die Binaries aller Plattformen nach ./dist/
	@mkdir -p "$(INSTALLER_DIST_DIR)"
	@set -eu; \
	for target in $(INSTALLER_RELEASE_TARGETS); do \
		os="$${target%-*}"; \
		arch="$${target#*-}"; \
		output="../$(INSTALLER_DIST_DIR)/$(INSTALLER_BINARY)-$${os}-$${arch}"; \
		echo "Baue $$output"; \
		(cd installer && CGO_ENABLED=0 GOOS="$$os" GOARCH="$$arch" go build -trimpath -ldflags="-s -w" -o "$$output" "$(INSTALLER_PKG)"); \
	done

build: dist ## Alias für dist

gui: dist ## Baut und startet die GUI über den Wrapper
	"$(INSTALLER_WRAPPER)"

test: ## Führt die Tests aus
	cd installer && go test ./...

installer-build: build ## Alias für build

installer-run: gui ## Alias für gui

installer-test: test ## Alias für test
