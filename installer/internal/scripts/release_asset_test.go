package scripts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Die Auflösung des Release-Assets wird gegen eine vorgegebene Asset-Liste
// geprüft, nicht gegen die GitHub-API: ein Test darf nicht ins Netz, und ein
// Muster, das erst beim nächsten Upstream-Release auffällt, ist kein Test.
//
// resolve_release_asset nimmt os und arch als Parameter, damit sich jede
// Plattform prüfen lässt, ohne auf ihr zu laufen.

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type releaseFixture struct {
	Tag    string         `json:"tag_name"`
	Assets []releaseAsset `json:"assets"`
}

// writeReleases legt eine Release-Antwort als Datei ab. Die Reihenfolge der
// Namen bleibt erhalten: der Resolver nimmt den ersten Treffer, und genau das
// macht die Prüfsummen-Geschwister gefährlich.
func writeReleases(t *testing.T, tag string, names []string) string {
	t.Helper()
	fixture := releaseFixture{Tag: tag}
	for _, name := range names {
		fixture.Assets = append(fixture.Assets, releaseAsset{
			Name: name,
			URL:  "https://example.invalid/" + name,
		})
	}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatalf("Fixture bauen: %v", err)
	}
	path := filepath.Join(t.TempDir(), "releases.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("Fixture schreiben: %v", err)
	}
	return path
}

// resolveAsset ruft resolve_release_asset aus der gemeinsamen Bibliothek auf.
// Die Bibliothek verlangt die(), log() und run_or_print() vom sourcenden
// Skript; hier stehen sie als Minimalfassung.
func resolveAsset(t *testing.T, assetRef, repo, pattern, osName, arch, releases string) result {
	t.Helper()
	lib := filepath.Join(repoRoot(t), "scripts", "lib", "install-common.sh")
	program := `set -euo pipefail
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
log() { printf '%s\n' "$*" >&2; }
run_or_print() { "$@"; }
DRY_RUN=0
source "$1"
resolve_release_asset "$2" "$3" "$4" "$5" "$6" "$7"`

	return runBash(t, program, lib, assetRef, repo, pattern, osName, arch, releases)
}

// TestBestehendeMusterLoesenDasselbeAssetAuf ist der Regressionstest zur
// Erweiterung des Platzhaltersatzes.
//
// Die Erweiterung um {arch_raw} und {vendor_os} sowie der Ausschluss der
// .sha256-Assets sind rein additiv: jedes Muster der Security-Matrix muss
// weiterhin genau dasselbe Asset treffen. Gelesen werden die Muster aus der
// ausgelieferten Matrix, nicht aus einer Kopie im Test — sonst prüfte der Test
// seine eigene Abschrift.
func TestBestehendeMusterLoesenDasselbeAssetAuf(t *testing.T) {
	// Je Werkzeug: die Assets eines echten Releases in ihrer üblichen Form,
	// und was das Muster auf zwei Plattformen treffen muss.
	expectations := map[string]struct {
		tag    string
		assets []string
		linux  string
		darwin string
	}{
		"gitleaks": {
			tag: "v8.30.1",
			assets: []string{
				"gitleaks_8.30.1_checksums.txt",
				"gitleaks_8.30.1_linux_arm64.tar.gz",
				"gitleaks_8.30.1_linux_x64.tar.gz",
				"gitleaks_8.30.1_darwin_x64.tar.gz",
				"gitleaks_8.30.1_darwin_arm64.tar.gz",
			},
			linux:  "gitleaks_8.30.1_linux_x64.tar.gz",
			darwin: "gitleaks_8.30.1_darwin_arm64.tar.gz",
		},
		"trufflehog": {
			tag: "v3.90.8",
			assets: []string{
				"trufflehog_3.90.8_checksums.txt",
				"trufflehog_3.90.8_linux_arm64.tar.gz",
				"trufflehog_3.90.8_linux_amd64.tar.gz",
				"trufflehog_3.90.8_darwin_amd64.tar.gz",
				"trufflehog_3.90.8_darwin_arm64.tar.gz",
			},
			linux:  "trufflehog_3.90.8_linux_amd64.tar.gz",
			darwin: "trufflehog_3.90.8_darwin_arm64.tar.gz",
		},
		"trivy": {
			tag: "v0.66.0",
			assets: []string{
				"trivy_0.66.0_checksums.txt",
				"trivy_0.66.0_Linux-ARM64.tar.gz",
				"trivy_0.66.0_Linux-64bit.tar.gz",
				"trivy_0.66.0_macOS-64bit.tar.gz",
				"trivy_0.66.0_macOS-ARM64.tar.gz",
			},
			linux:  "trivy_0.66.0_Linux-64bit.tar.gz",
			darwin: "trivy_0.66.0_macOS-ARM64.tar.gz",
		},
		"syft": {
			tag: "v1.33.0",
			assets: []string{
				"syft_1.33.0_checksums.txt",
				"syft_1.33.0_linux_arm64.tar.gz",
				"syft_1.33.0_linux_amd64.tar.gz",
				"syft_1.33.0_darwin_amd64.tar.gz",
				"syft_1.33.0_darwin_arm64.tar.gz",
			},
			linux:  "syft_1.33.0_linux_amd64.tar.gz",
			darwin: "syft_1.33.0_darwin_arm64.tar.gz",
		},
		"grype": {
			tag: "v0.100.0",
			assets: []string{
				"grype_0.100.0_checksums.txt",
				"grype_0.100.0_linux_arm64.tar.gz",
				"grype_0.100.0_linux_amd64.tar.gz",
				"grype_0.100.0_darwin_amd64.tar.gz",
				"grype_0.100.0_darwin_arm64.tar.gz",
			},
			linux:  "grype_0.100.0_linux_amd64.tar.gz",
			darwin: "grype_0.100.0_darwin_arm64.tar.gz",
		},
		"osv-scanner": {
			tag: "v2.2.3",
			assets: []string{
				"osv-scanner_checksums.txt",
				"osv-scanner_linux_arm64",
				"osv-scanner_linux_amd64",
				"osv-scanner_darwin_amd64",
				"osv-scanner_darwin_arm64",
			},
			linux:  "osv-scanner_linux_amd64",
			darwin: "osv-scanner_darwin_arm64",
		},
		"gosec": {
			tag: "v2.22.9",
			assets: []string{
				"gosec_2.22.9_checksums.txt",
				"gosec_2.22.9_linux_arm64.tar.gz",
				"gosec_2.22.9_linux_amd64.tar.gz",
				"gosec_2.22.9_darwin_amd64.tar.gz",
				"gosec_2.22.9_darwin_arm64.tar.gz",
			},
			linux:  "gosec_2.22.9_linux_amd64.tar.gz",
			darwin: "gosec_2.22.9_darwin_arm64.tar.gz",
		},
		"golangci-lint": {
			tag: "v2.5.0",
			assets: []string{
				"golangci-lint-2.5.0-checksums.txt",
				"golangci-lint-2.5.0-linux-arm64.tar.gz",
				"golangci-lint-2.5.0-linux-amd64.tar.gz",
				"golangci-lint-2.5.0-darwin-amd64.tar.gz",
				"golangci-lint-2.5.0-darwin-arm64.tar.gz",
			},
			linux:  "golangci-lint-2.5.0-linux-amd64.tar.gz",
			darwin: "golangci-lint-2.5.0-darwin-arm64.tar.gz",
		},
	}

	entries := githubMatrixEntries(t, filepath.Join(repoRoot(t), "scripts", "security-tools.tsv"),
		matrixColumns{name: 0, method: 4, ref: 5, pattern: 6})
	if len(entries) == 0 {
		t.Fatal("security-tools.tsv führt keinen github-Eintrag mehr")
	}

	for _, entry := range entries {
		want, known := expectations[entry.name]
		if !known {
			t.Fatalf("neuer github-Eintrag %q in security-tools.tsv: die Erwartung im Regressionstest fehlt", entry.name)
		}

		releases := writeReleases(t, want.tag, want.assets)
		for _, platform := range []struct {
			os, arch, want string
		}{
			{"linux", "amd64", want.linux},
			{"darwin", "arm64", want.darwin},
		} {
			t.Run(entry.name+"/"+platform.os+"-"+platform.arch, func(t *testing.T) {
				got := resolveAsset(t, entry.name, entry.ref, entry.pattern, platform.os, platform.arch, releases)
				if got.code != 0 {
					t.Fatalf("Auflösung schlug fehl:\n%s", got.all())
				}
				lines := strings.Split(strings.TrimSpace(got.stdout), "\n")
				if len(lines) != 3 {
					t.Fatalf("erwartet Tag, URL und Name, bekommen:\n%s", got.stdout)
				}
				if lines[0] != want.tag {
					t.Errorf("Tag = %q, erwartet %q", lines[0], want.tag)
				}
				if lines[2] != platform.want {
					t.Errorf("Asset = %q, erwartet %q", lines[2], platform.want)
				}
			})
		}
	}
}

// TestRipgrepMusterTrifftDieTargetTriples ist der Grund für die Erweiterung:
// BurntSushi/ripgrep benennt seine Assets nach Rust-Target-Triples, und keine
// Kombination der bisherigen Platzhalter erzeugt x86_64-unknown-linux-musl oder
// apple-darwin. Der Libc-Teil ist unter Linux nicht einheitlich — x86_64 kommt
// als musl, aarch64 als gnu.
//
// Geprüft wird zugleich, dass {tool} an die Installationsreferenz bindet: das
// Asset heißt ripgrep-…, das Programm heißt rg.
func TestRipgrepMusterTrifftDieTargetTriples(t *testing.T) {
	entry := matrixEntry{}
	for _, candidate := range githubMatrixEntries(t, filepath.Join(repoRoot(t), "scripts", "base-tools.tsv"),
		matrixColumns{name: 0, method: 4, ref: 6, assetRef: 7, pattern: 8}) {
		if candidate.name == "rg" {
			entry = candidate
		}
	}
	if entry.name == "" {
		t.Fatal("base-tools.tsv führt rg nicht mehr über den github-Weg")
	}
	if entry.assetRef != "ripgrep" {
		t.Errorf("asset_ref = %q, erwartet ripgrep — im Asset-Muster steht die Installationsreferenz, nicht der Programmname", entry.assetRef)
	}

	assets := []string{
		"ripgrep-14.1.1-aarch64-unknown-linux-gnu.tar.gz",
		"ripgrep-14.1.1-armv7-unknown-linux-gnueabihf.tar.gz",
		"ripgrep-14.1.1-i686-unknown-linux-gnu.tar.gz",
		"ripgrep-14.1.1-powerpc64-unknown-linux-gnu.tar.gz",
		"ripgrep-14.1.1-x86_64-apple-darwin.tar.gz",
		"ripgrep-14.1.1-aarch64-apple-darwin.tar.gz",
		"ripgrep-14.1.1-x86_64-pc-windows-gnu.zip",
		"ripgrep-14.1.1-x86_64-unknown-linux-musl.tar.gz",
		"ripgrep_14.1.1-1_amd64.deb",
	}
	// Jedes Asset hat ein .sha256-Geschwister. Sie stehen hier vor den echten
	// Namen, damit ein Ausrutscher sofort auffällt.
	var withChecksums []string
	for _, name := range assets {
		withChecksums = append(withChecksums, name+".sha256")
	}
	withChecksums = append(withChecksums, assets...)
	releases := writeReleases(t, "14.1.1", withChecksums)

	cases := []struct {
		os, arch, want string
	}{
		{"linux", "amd64", "ripgrep-14.1.1-x86_64-unknown-linux-musl.tar.gz"},
		{"linux", "arm64", "ripgrep-14.1.1-aarch64-unknown-linux-gnu.tar.gz"},
		{"darwin", "amd64", "ripgrep-14.1.1-x86_64-apple-darwin.tar.gz"},
		{"darwin", "arm64", "ripgrep-14.1.1-aarch64-apple-darwin.tar.gz"},
	}

	for _, platform := range cases {
		t.Run(platform.os+"-"+platform.arch, func(t *testing.T) {
			got := resolveAsset(t, entry.assetRef, entry.ref, entry.pattern, platform.os, platform.arch, releases)
			if got.code != 0 {
				t.Fatalf("Auflösung schlug fehl:\n%s", got.all())
			}
			lines := strings.Split(strings.TrimSpace(got.stdout), "\n")
			if len(lines) != 3 || lines[2] != platform.want {
				t.Fatalf("Asset = %q, erwartet %q\n%s", lines[len(lines)-1], platform.want, got.stdout)
			}
		})
	}
}

// TestPruefsummenWerdenNieGetroffen deckt genau den Fall ab, für den der
// Ausschluss da ist: ein laxes Muster ohne Endanker träfe sonst zuerst die
// Prüfsumme und installierte eine Textdatei.
func TestPruefsummenWerdenNieGetroffen(t *testing.T) {
	releases := writeReleases(t, "v1.0.0", []string{
		"werkzeug_1.0.0_linux_amd64.tar.gz.sha256",
		"werkzeug_1.0.0_linux_amd64.tar.gz",
	})

	got := resolveAsset(t, "werkzeug", "beispiel/werkzeug", `^{tool}_{version}_{os}_{arch}\.tar\.gz`, "linux", "amd64", releases)
	if got.code != 0 {
		t.Fatalf("Auflösung schlug fehl:\n%s", got.all())
	}
	lines := strings.Split(strings.TrimSpace(got.stdout), "\n")
	if lines[2] != "werkzeug_1.0.0_linux_amd64.tar.gz" {
		t.Errorf("Asset = %q, erwartet das Archiv statt der Prüfsumme", lines[2])
	}
}
