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

# Dieses Repo ist zugleich sein eigenes Zielprojekt: die Installation liegt
# darunter und ist ein eigener Clone. Sie traegt deshalb den zuletzt gepushten
# Stand, nicht den, an dem gerade gearbeitet wird — und die Oberflaeche liest
# Skripte, Regeln und Reviews immer von dort.
PLAYBOOK_DIR := k-playbook
INSTALLATION_DIR := $(if $(wildcard $(PLAYBOOK_DIR)/.git),$(PLAYBOOK_DIR),.)
DEV_MARKER := .k-playbook-devsync

.PHONY: help build dist gui test installer-build installer-run installer-test installer-sync installer-readonly installer-writable installer-update

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

# -buildvcs=false ist kein Detail: ohne das stempelt Go den Commit-Hash ins
# Binary. Jeder Commit ergäbe dann vier neue Binaries à 12 MB, die der nächste
# `git add -A` mitnimmt — auch wenn sich am Code nichts geändert hat. Gelesen
# wird die Revision von niemandem.
dist: ## Baut die Binaries aller Plattformen nach ./dist/
	@mkdir -p "$(INSTALLER_DIST_DIR)"
	@set -eu; \
	for target in $(INSTALLER_RELEASE_TARGETS); do \
		os="$${target%-*}"; \
		arch="$${target#*-}"; \
		output="../$(INSTALLER_DIST_DIR)/$(INSTALLER_BINARY)-$${os}-$${arch}"; \
		echo "Baue $$output"; \
		(cd installer && CGO_ENABLED=0 GOOS="$$os" GOARCH="$$arch" go build -trimpath -buildvcs=false -ldflags="-s -w" -o "$$output" "$(INSTALLER_PKG)"); \
	done

build: dist ## Alias für dist

# Nur im Entwicklungsrepo sinnvoll: in einem Zielprojekt gibt es keinen
# Arbeitsstand, aus dem gesynct werden könnte, und ein rsync über die
# Installation wäre dort schlicht Datenverlust.
define require_dev_repo
	@test -d "$(PLAYBOOK_DIR)/.git" -a -d installer || { \
	  echo "installer-sync gilt nur im Entwicklungsrepo: $(PLAYBOOK_DIR)/ muss ein Clone sein und installer/ vorhanden." >&2; \
	  exit 1; \
	}
endef

# Uebertragen wird genau der verfolgte Dateisatz — das ist per Definition, was
# ein Clone enthaelt. Ein Filter auf .gitignore waere nur eine Naeherung: er
# schleppt unverfolgte, aber nicht ignorierte Dateien mit, etwa
# .claude/settings.local.json.
installer-sync: ## Spielt den Arbeitsstand in die Installation ein (nur Entwicklungsrepo)
	$(require_dev_repo)
	@set -eu; \
	  trap 'chmod -R a-w "$(PLAYBOOK_DIR)"' EXIT; \
	  chmod -R u+w "$(PLAYBOOK_DIR)"; \
	  git -C "$(PLAYBOOK_DIR)" checkout -- .; \
	  git -C "$(PLAYBOOK_DIR)" clean -qfd; \
	  git ls-files -z | rsync -a --from0 --files-from=- --delete-missing-args ./ "$(PLAYBOOK_DIR)/"; \
	  soll="$$(mktemp)"; ist="$$(mktemp)"; \
	  git ls-files | sort > "$$soll"; \
	  (cd "$(PLAYBOOK_DIR)" && find . -path ./.git -prune -o \( -type f -o -type l \) -print) \
	    | sed 's|^\./||' | grep -vx '$(DEV_MARKER)' | sort > "$$ist"; \
	  comm -13 "$$soll" "$$ist" | while IFS= read -r path; do rm -f "$(PLAYBOOK_DIR)/$$path"; done; \
	  rm -f "$$soll" "$$ist"; \
	  find "$(PLAYBOOK_DIR)" -depth -type d -empty \
	    ! -path "$(PLAYBOOK_DIR)/.git" ! -path "$(PLAYBOOK_DIR)/.git/*" -delete; \
	  printf 'Eingespielter Arbeitsstand, kein Clone.\nEntstanden durch "make installer-sync".\nZurück: in der Oberfläche "Arbeitsstand verwerfen".\n' \
	    > "$(PLAYBOOK_DIR)/$(DEV_MARKER)"
	@echo "Arbeitsstand eingespielt nach $(PLAYBOOK_DIR)/"

installer-writable: ## Macht die lokale Installation temporär beschreibbar
	@chmod -R u+w "$(INSTALLATION_DIR)"

installer-readonly: ## Sperrt Schreibzugriffe auf die lokale Installation
	@chmod -R a-w "$(INSTALLATION_DIR)"

# Kein pull --ff-only: dieses Ziel laeuft auch, wenn zwischenzeitlich ein
# installer-sync in die Installation geschrieben hat. Der Vertrag von
# INSTALLATION_DIR (nie schreiben) macht reset --hard sicher; Sync-Reste und
# der Marker werden dabei entfernt und die Installation ist wieder ein
# sauberer Clone.
installer-update: ## Aktualisiert die lokale Installation und sperrt sie danach
	@set -eu; \
	  trap 'chmod -R a-w "$(INSTALLATION_DIR)"' EXIT; \
	  chmod -R u+w "$(INSTALLATION_DIR)"; \
	  before="$$(git -C "$(INSTALLATION_DIR)" rev-parse HEAD)"; \
	  git -C "$(INSTALLATION_DIR)" fetch --quiet origin; \
	  git -C "$(INSTALLATION_DIR)" reset --hard --quiet origin/main; \
	  git -C "$(INSTALLATION_DIR)" clean -qfd; \
	  after="$$(git -C "$(INSTALLATION_DIR)" rev-parse HEAD)"; \
	  if [ "$$before" = "$$after" ]; then \
	    echo "Installation bereits aktuell ($$after)"; \
	  else \
	    echo "Installation aktualisiert: $$before -> $$after"; \
	  fi

gui: dist installer-sync ## Baut, spielt den Arbeitsstand ein und startet die GUI
	"$(INSTALLER_WRAPPER)"

test: ## Führt die Tests aus
	cd installer && go test ./...

installer-build: build ## Alias für build

installer-run: gui ## Alias für gui

installer-test: test ## Alias für test
