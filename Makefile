.DEFAULT_GOAL := help

INSTALL_BIN ?= $(HOME)/.local/bin
OS_NAME ?= $(shell uname -s 2>/dev/null)
USER_SHELL ?= $(notdir $(shell printf '%s' "$${SHELL:-}"))
ifeq ($(USER_SHELL),zsh)
PATH_PROFILE ?= $(HOME)/.zprofile
else ifeq ($(USER_SHELL),bash)
ifeq ($(OS_NAME),Darwin)
PATH_PROFILE ?= $(HOME)/.bash_profile
else
PATH_PROFILE ?= $(HOME)/.profile
endif
else
PATH_PROFILE ?= $(HOME)/.profile
endif
PATH_EXPORT := export PATH="$(INSTALL_BIN):$$PATH"
INSTALLER_BINARY := k-playbook-installer
INSTALLER_SOURCE := ./installer/cmd/k-playbook-installer
INSTALLER_BUILD_DIR := bin
INSTALLER_WRAPPER := $(INSTALLER_BUILD_DIR)/$(INSTALLER_BINARY)
INSTALLER_WRAPPER_TEMPLATE := ./scripts/templates/k-playbook-installer-wrapper.sh
INSTALLER_DIST_DIR := dist
INSTALLER_RELEASE_TARGETS := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64

.PHONY: help build dist install install-from-source uninstall gui test clean installer-build installer-install installer-install-from-source installer-uninstall installer-run installer-test installer-clean path-hint path-setup

help: ## Zeigt diese Hilfe an
	@echo "Verfuegbare Targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-30s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Typischer Start:"
	@echo "  make install"
	@echo "  k-playbook-installer"
	@echo "  # alternativ ohne make: ./scripts/install-installer.sh"
	@echo ""
	@echo "Aus dem Source mit lokal installiertem Go:"
	@echo "  make install-from-source"
	@echo "  make gui"
	@echo ""
	@echo "Nach neu geladenem PATH auch direkt:"
	@echo "  k-playbook-installer"
	@echo ""

build: ## Baut alle Installer-Binaries nach ./bin/
	@mkdir -p "$(INSTALLER_BUILD_DIR)"
	@set -eu; \
	for target in $(INSTALLER_RELEASE_TARGETS); do \
		os="$${target%-*}"; \
		arch="$${target#*-}"; \
		output="../$(INSTALLER_BUILD_DIR)/$(INSTALLER_BINARY)-$${os}-$${arch}"; \
		echo "Baue $$output"; \
		(cd installer && CGO_ENABLED=0 GOOS="$$os" GOARCH="$$arch" go build -o "$$output" ./cmd/k-playbook-installer); \
	done
	install -m 0755 "$(INSTALLER_WRAPPER_TEMPLATE)" "$(INSTALLER_WRAPPER)"

dist: ## Baut Installer-Artefakte nach ./dist/
	@mkdir -p "$(INSTALLER_DIST_DIR)"
	@set -eu; \
	for target in $(INSTALLER_RELEASE_TARGETS); do \
		os="$${target%-*}"; \
		arch="$${target#*-}"; \
		output="../$(INSTALLER_DIST_DIR)/$(INSTALLER_BINARY)-$${os}-$${arch}"; \
		echo "Baue $$output"; \
		(cd installer && CGO_ENABLED=0 GOOS="$$os" GOARCH="$$arch" go build -trimpath -ldflags="-s -w" -o "$$output" ./cmd/k-playbook-installer); \
	done

install: ## Installiert den Installer ohne Go aus vorhandenen Binaries oder GitHub Releases
	./scripts/install-installer.sh --bin-dir "$(INSTALL_BIN)"

install-from-source: build ## Baut das Binary und verlinkt es nach ~/.local/bin
	@mkdir -p "$(INSTALL_BIN)"
	ln -sfn "$(CURDIR)/$(INSTALLER_WRAPPER)" "$(INSTALL_BIN)/$(INSTALLER_BINARY)"
	@echo "Verlinkt: $(INSTALL_BIN)/$(INSTALLER_BINARY) -> $(CURDIR)/$(INSTALLER_WRAPPER)"
	@$(MAKE) --no-print-directory path-setup
	@echo ""
	@echo "Starten ohne PATH-Abhaengigkeit:"
	@echo "  make gui"
	@echo ""

uninstall: ## Entfernt den Installer-Symlink aus ~/.local/bin
	rm -f "$(INSTALL_BIN)/$(INSTALLER_BINARY)"
	@echo "Entfernt: $(INSTALL_BIN)/$(INSTALLER_BINARY)"

gui: build ## Startet die Installer-GUI aus ./bin/
	"$(INSTALLER_WRAPPER)"

test: ## Fuehrt Installer-Tests aus
	cd installer && go test ./...

clean: ## Entfernt lokale Installer-Build-Artefakte
	rm -rf "$(INSTALLER_BUILD_DIR)" "$(INSTALLER_DIST_DIR)"

installer-build: build ## Alias fuer build

installer-install: install ## Alias fuer install

installer-install-from-source: install-from-source ## Alias fuer install-from-source

installer-uninstall: uninstall ## Alias fuer uninstall

installer-run: gui ## Alias fuer gui

installer-test: test ## Alias fuer test

installer-clean: clean ## Alias fuer clean

path-hint: ## Prueft, ob ~/.local/bin im PATH liegt
	@if printf '%s' ":$$PATH:" | grep -q ":$(INSTALL_BIN):"; then \
		echo "PATH OK: $(INSTALL_BIN) ist im PATH."; \
	else \
		echo "Hinweis: $(INSTALL_BIN) ist nicht im PATH."; \
		echo "Fuege z. B. diese Zeile zu $(PATH_PROFILE) hinzu:"; \
		echo '  $(PATH_EXPORT)'; \
	fi

path-setup: ## Fragt interaktiv, ob ~/.local/bin ins Shell-Profil eingetragen werden soll
	@if printf '%s' ":$$PATH:" | grep -q ":$(INSTALL_BIN):"; then \
		echo "PATH OK: $(INSTALL_BIN) ist im PATH."; \
	elif ! [ -t 0 ]; then \
		$(MAKE) --no-print-directory path-hint; \
	else \
		echo "Hinweis: $(INSTALL_BIN) ist nicht im PATH."; \
		printf 'Soll $(PATH_PROFILE) automatisch ergaenzt werden? [y/N] '; \
		read answer; \
		case "$$answer" in \
			y|Y|yes|YES|j|J|ja|JA) \
				touch "$(PATH_PROFILE)"; \
				if grep -Fq '$(PATH_EXPORT)' "$(PATH_PROFILE)"; then \
					echo "PATH-Eintrag existiert bereits in $(PATH_PROFILE)."; \
				else \
					printf '\n# k-playbook installer\n$(PATH_EXPORT)\n' >> "$(PATH_PROFILE)"; \
					echo "PATH-Eintrag zu $(PATH_PROFILE) hinzugefuegt."; \
				fi; \
				echo "Aktiviere ihn mit:"; \
				echo "  . $(PATH_PROFILE)"; \
				;; \
			*) \
				echo "Nicht geaendert. Fuege bei Bedarf manuell hinzu:"; \
				echo '  $(PATH_EXPORT)'; \
				;; \
		esac; \
	fi
