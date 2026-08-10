.DEFAULT_GOAL := help

# Ziel fuer den optionalen host-weiten Symlink. Der Regelfall ist der Aufruf
# ueber bin/k-playbook im jeweiligen Clone; ~/.local/bin ist nur Komfort.
INSTALL_BIN ?= $(HOME)/.local/bin
PATH_BIN ?= $(INSTALL_BIN)
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
ifeq ($(PATH_BIN),$(HOME)/.local/bin)
PATH_EXPORT := export PATH="$$HOME/.local/bin:$$PATH"
else
PATH_EXPORT := export PATH="$(PATH_BIN):$$PATH"
endif
INSTALLER_BINARY := k-playbook
# Paketpfad relativ zum installer/-Verzeichnis, in dem go build laeuft.
INSTALLER_PKG := ./cmd/k-playbook
# Der Wrapper liegt versioniert im Repo, damit direkt nach dem Clone ein
# Einstiegspunkt vorhanden ist. Er wird nicht gebaut.
INSTALLER_WRAPPER := bin/$(INSTALLER_BINARY)
INSTALLER_DIST_DIR := dist
INSTALLER_RELEASE_TARGETS := linux-amd64 linux-arm64 darwin-amd64 darwin-arm64

.PHONY: help build dist install install-from-source uninstall gui test installer-build installer-install installer-install-from-source installer-uninstall installer-run installer-test path-hint path-setup

help: ## Zeigt diese Hilfe an
	@echo "Verfuegbare Targets:"
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
	@echo "Host-weit verfuegbar machen:"
	@echo "  make install-from-source"
	@echo "  k-playbook"
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

build: dist ## Alias fuer dist

install: ## Installiert den Installer ohne Go aus vorhandenen Binaries oder GitHub Releases
	PATH_BIN="$(PATH_BIN)" PATH_PROFILE="$(PATH_PROFILE)" ./scripts/install-installer.sh --bin-dir "$(INSTALL_BIN)"

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

gui: dist ## Baut und startet die GUI ueber den Wrapper
	"$(INSTALLER_WRAPPER)"

test: ## Fuehrt die Tests aus
	cd installer && go test ./...

installer-build: build ## Alias fuer build

installer-install: install ## Alias fuer install

installer-install-from-source: install-from-source ## Alias fuer install-from-source

installer-uninstall: uninstall ## Alias fuer uninstall

installer-run: gui ## Alias fuer gui

installer-test: test ## Alias fuer test

path-hint: ## Prueft, ob der Installationsort im PATH liegt
	@if printf '%s' ":$$PATH:" | grep -q ":$(PATH_BIN):"; then \
		echo "PATH OK: $(PATH_BIN) ist im PATH."; \
	else \
		echo "Hinweis: $(PATH_BIN) ist nicht im PATH."; \
		echo "Fuege z. B. diese Zeile zu $(PATH_PROFILE) hinzu:"; \
		echo '  $(PATH_EXPORT)'; \
	fi

path-setup: ## Stellt sicher, dass der Installationsort im Shell-Profil steht
	@if printf '%s' ":$$PATH:" | grep -q ":$(PATH_BIN):"; then \
		echo "PATH OK: $(PATH_BIN) ist im PATH."; \
	else \
		touch "$(PATH_PROFILE)"; \
		if grep -Fq '$(PATH_BIN)' "$(PATH_PROFILE)" || grep -Fq '$$HOME/.local/bin' "$(PATH_PROFILE)" || grep -Fq '$${HOME}/.local/bin' "$(PATH_PROFILE)"; then \
			echo "PATH-Eintrag existiert bereits in $(PATH_PROFILE), ist aber in dieser Shell noch nicht aktiv."; \
		else \
			printf '\n# k-playbook installer PATH\n$(PATH_EXPORT)\n' >> "$(PATH_PROFILE)"; \
			echo "PATH-Eintrag zu $(PATH_PROFILE) hinzugefuegt."; \
		fi; \
		echo "Aktiviere ihn mit:"; \
		echo "  . $(PATH_PROFILE)"; \
	fi
