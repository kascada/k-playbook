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
# Die Toolchain steht in installer/go.mod und wird hier nur gelesen. Sie ist
# der Grund, warum das versionierte SHA256SUMS trägt: CI baut mit derselben
# Version und kommt deshalb auf dieselben Summen.
INSTALLER_GO_TOOLCHAIN = $(shell awk '$$1 == "toolchain" { print $$2 }' installer/go.mod)
# Die Prüfsummendatei liegt im Wurzelverzeichnis, nicht in dist/: sie kommt so
# über den Git-Remote und nicht über dieselbe HTTPS-Quelle wie das Binary.
INSTALLER_SUMS_FILE := SHA256SUMS
INSTALLER_VERSION_FILE := VERSION
# Bewusst mit = statt := : `go env` darf erst laufen, wenn ein Target es
# wirklich braucht. In einem Zielprojekt ohne Go würde die sofortige Auswertung
# sonst schon bei `make help` eine Fehlermeldung ausgeben.
INSTALLER_HOST_TARGET = $(shell go env GOOS)-$(shell go env GOARCH)

# Dieses Repo ist zugleich sein eigenes Zielprojekt: die Installation liegt
# darunter und ist ein eigener Clone. Sie trägt deshalb den zuletzt gepushten
# Stand, nicht den, an dem gerade gearbeitet wird — und die Oberfläche liest
# Skripte, Regeln und Reviews immer von dort.
PLAYBOOK_DIR := k-playbook
INSTALLATION_DIR := $(if $(wildcard $(PLAYBOOK_DIR)/.git),$(PLAYBOOK_DIR),.)
DEV_MARKER := .k-playbook-devsync
# Für Fehlermeldungen: das Ziel, das der Aufrufer genannt hat. Ohne Angabe
# greift das Standardziel, damit die Meldung nie einen leeren Namen zeigt.
GOAL = $(or $(firstword $(MAKECMDGOALS)),$(.DEFAULT_GOAL))

.PHONY: help build dist dist-host gui test release release-publish installer-build installer-run installer-test installer-sync installer-readonly installer-writable installer-update

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
	@echo "  make dist        alle Plattformen, für ein Release"
	@echo "  make gui         nur diese Plattform, dann starten"
	@echo ""
	@echo "Host-weit verfügbar: richtet der erste Start selbst ein."
	@echo "  danach genügt überall:  k-playbook"
	@echo ""

# -buildvcs=false ist kein Detail: ohne das stempelt Go den Commit-Hash ins
# Binary. Jeder Commit ergäbe dann vier neue Binaries à 12 MB, die der nächste
# `git add -A` mitnimmt — auch wenn sich am Code nichts geändert hat. Gelesen
# wird die Revision von niemandem.
define build_binaries
	$(require_writable)
	@mkdir -p "$(INSTALLER_DIST_DIR)"
	@set -eu; \
	for target in $(1); do \
		os="$${target%-*}"; \
		arch="$${target#*-}"; \
		output="../$(INSTALLER_DIST_DIR)/$(INSTALLER_BINARY)-$${os}-$${arch}"; \
		echo "Baue $$output"; \
		(cd installer && CGO_ENABLED=0 GOOS="$$os" GOARCH="$$arch" go build -trimpath -buildvcs=false -ldflags="-s -w" -o "$$output" "$(INSTALLER_PKG)"); \
	done
endef

dist: ## Baut die Binaries aller Plattformen nach ./dist/
	$(call build_binaries,$(INSTALLER_RELEASE_TARGETS))

# Für den Entwicklungs-Loop: die drei fremden Plattformen kostet jeder Durchlauf
# Zeit, gestartet wird ohnehin nur diese. Die anderen Binaries bleiben auf dem
# committeten Stand — `make dist` baut vor einem Release alle vier.
dist-host: ## Baut nur das Binary dieser Plattform nach ./dist/
	$(call build_binaries,$(INSTALLER_HOST_TARGET))

build: dist ## Alias für dist

# Beide Prüfungen unten schlagen bei derselben Verwechslung an — Aufruf in der
# Installation statt im Arbeitsstand —, scheitern aber an verschiedenen Stellen:
# der Build an der Schreibsperre, der Sync am fehlenden Arbeitsstand. Der
# Hinweis ist deshalb geteilt: gleicher Rat im Entwicklungsrepo, eigener
# Fallback für ein Zielprojekt, wo jede Prüfung etwas anderes bedeutet.
define in_dev_installation
test -d "../installer" -a -d "../$(PLAYBOOK_DIR)/.git"
endef

define hint_use_workspace
	    printf 'Hier läuft die Makefile-Kopie in der Installation. Gebaut und\n' >&2; \
	    printf 'eingespielt wird aus dem Arbeitsstand eine Ebene höher:\n\n' >&2; \
	    printf '  make -C %s %s\n\n' "$(abspath ..)" "$(GOAL)" >&2
endef

# Nur im Entwicklungsrepo sinnvoll: in einem Zielprojekt gibt es keinen
# Arbeitsstand, aus dem gesynct werden könnte, und ein rsync über die
# Installation wäre dort schlicht Datenverlust.
define require_dev_repo
	@test -d "$(PLAYBOOK_DIR)/.git" -a -d installer || { \
	  if $(in_dev_installation); then \
	    $(hint_use_workspace); \
	  else \
	    printf '%s gilt nur im Entwicklungsrepo: %s/ muss ein Clone sein und installer/ vorhanden.\n' \
	      "$(GOAL)" "$(PLAYBOOK_DIR)" >&2; \
	  fi; \
	  exit 1; \
	}
endef

# Vor dem Build, nicht mittendrin: sonst bricht `go build` erst beim Schreiben
# des Binaries ab, und zwar mit "permission denied" auf einen Pfad in /tmp.
define require_writable
	@test -w . || { \
	  if $(in_dev_installation); then \
	    $(hint_use_workspace); \
	  else \
	    printf 'Schreibgeschützt: %s\n' "$(CURDIR)" >&2; \
	    printf 'Eine Installation wird nicht überschrieben. Zum Bauen freigeben:\n' >&2; \
	    printf '  make installer-writable\n' >&2; \
	  fi; \
	  exit 1; \
	}
endef

# Übertragen wird genau der verfolgte Dateisatz — das ist per Definition, was
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

gui: dist-host installer-sync ## Baut, spielt den Arbeitsstand ein und startet die GUI
	"$(INSTALLER_WRAPPER)"

test: ## Führt die Tests aus
	cd installer && go test ./...

# Ein Release läuft in zwei Schritten, und die Reihenfolge ist nicht beliebig.
#
#   make release VERSION=v0.1.0   baut, schreibt VERSION und SHA256SUMS,
#                                 committet beides und pusht nur den Tag
#   ... CI baut, prüft gegen SHA256SUMS und lädt die Assets hoch ...
#   make release-publish VERSION=v0.1.0   bringt denselben Commit auf main
#
# Andersherum zeigte VERSION eine Zeit lang auf einen Tag ohne Downloads, und
# jede Installation, die in diesem Fenster aktualisiert, startet nicht mehr.
# Einen Versions-Fallback im Wrapper gibt es dafür nicht.
define require_toolchain
	@set -eu; \
	  want="$(INSTALLER_GO_TOOLCHAIN)"; \
	  test -n "$$want" || { \
	    printf 'In installer/go.mod fehlt die toolchain-Zeile.\n' >&2; \
	    exit 1; \
	  }; \
	  have="$$(go env GOVERSION)"; \
	  test "$$have" = "$$want" || { \
	    printf 'Go %s ist installiert, verlangt ist %s (installer/go.mod, toolchain).\n' \
	      "$$have" "$$want" >&2; \
	    printf 'Ein Release mit einer anderen Toolchain ergäbe andere Prüfsummen als CI.\n' >&2; \
	    exit 1; \
	  }
endef

define require_version_arg
	@test -n "$(VERSION)" || { \
	  printf 'Aufruf: make %s VERSION=v0.1.0\n' "$(GOAL)" >&2; \
	  exit 1; \
	}
	@case "$(VERSION)" in v*) ;; *) \
	  printf 'Die Version muss mit v beginnen: %s\n' "$(VERSION)" >&2; exit 1 ;; \
	esac
endef

release: ## Baut das Release, committet VERSION und SHA256SUMS und pusht den Tag (VERSION=v0.1.0)
	$(require_version_arg)
	$(require_toolchain)
	@set -eu; \
	  dirty="$$(git status --porcelain)"; \
	  test -z "$$dirty" || { \
	    printf 'Der Arbeitsstand ist nicht sauber. Ein Release taggt genau den Stand,\n' >&2; \
	    printf 'den CI nachbaut — offene Änderungen gehören vorher committet.\n\n' >&2; \
	    printf '%s\n\n' "$$dirty" >&2; \
	    printf 'Unverfolgtes (??) zählt mit: der Build hier nähme es auf, CI baut den\n' >&2; \
	    printf 'Tag ohne es — die Prüfsummen gingen auseinander und der Lauf würde rot.\n' >&2; \
	    exit 1; \
	  }
	@git rev-parse --verify --quiet "refs/tags/$(VERSION)" >/dev/null && { \
	  printf 'Den Tag %s gibt es bereits.\n' "$(VERSION)" >&2; exit 1; \
	} || true
	$(call build_binaries,$(INSTALLER_RELEASE_TARGETS))
	@printf '%s\n' "$(VERSION)" > "$(INSTALLER_VERSION_FILE)"
	@set -eu; \
	  if command -v sha256sum >/dev/null 2>&1; then \
	    checksum() { sha256sum "$$@"; }; \
	  else \
	    checksum() { shasum -a 256 "$$@"; }; \
	  fi; \
	  ( cd "$(INSTALLER_DIST_DIR)" && for target in $(INSTALLER_RELEASE_TARGETS); do \
	      checksum "$(INSTALLER_BINARY)-$$target"; \
	    done ) > "$(INSTALLER_SUMS_FILE)"
	@git add "$(INSTALLER_VERSION_FILE)" "$(INSTALLER_SUMS_FILE)"
	@git commit -m "Release $(VERSION)"
	@git tag -a "$(VERSION)" -m "Release $(VERSION)"
	@git push origin "refs/tags/$(VERSION)"
	@echo ""
	@echo "Tag $(VERSION) gepusht. CI baut jetzt und lädt die Assets hoch."
	@echo "Danach, und erst danach:"
	@echo "  make release-publish VERSION=$(VERSION)"

release-publish: ## Bringt den getaggten Commit auf main — erst wenn die Assets liegen
	$(require_version_arg)
	@set -eu; \
	  tagged="$$(git rev-parse "$(VERSION)^{commit}")"; \
	  head="$$(git rev-parse HEAD)"; \
	  test "$$tagged" = "$$head" || { \
	    printf 'HEAD ist nicht der getaggte Commit (%s vs. %s).\n' "$$head" "$$tagged" >&2; \
	    exit 1; \
	  }
	@set -eu; \
	  command -v gh >/dev/null 2>&1 || { \
	    printf 'Ohne gh lässt sich hier nicht prüfen, ob die Assets liegen.\n' >&2; \
	    printf 'Von Hand nachsehen und dann: git push origin main\n' >&2; \
	    exit 1; \
	  }; \
	  for target in $(INSTALLER_RELEASE_TARGETS); do \
	    name="$(INSTALLER_BINARY)-$$target"; \
	    gh release view "$(VERSION)" --json assets \
	      --jq '.assets[].name' 2>/dev/null | grep -qx "$$name" || { \
	      printf 'Im Release %s fehlt das Asset %s.\n' "$(VERSION)" "$$name" >&2; \
	      printf 'VERSION darf erst auf main, wenn alle vier Assets liegen.\n' >&2; \
	      exit 1; \
	    }; \
	  done
	@git push origin main
	@echo "$(VERSION) ist auf main."

installer-build: build ## Alias für build

installer-run: gui ## Alias für gui

installer-test: test ## Alias für test
