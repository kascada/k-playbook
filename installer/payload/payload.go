// Package payload carries the files that get installed into a project's
// k-playbook/_dist directory: commands, skills, rules, review recipes, checks,
// scripts and the check runner.
//
// Embedding them into the binary is what makes a target project self-contained.
// There is no source repo to clone and no fixed host path to honour; a single
// binary can install and update any number of projects.
package payload

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The all: prefix is required: without it go:embed silently skips entries
// starting with _ or ., which would drop commands/_shared and commands/_details.
//
//go:embed all:commands all:skills all:rules all:reviews all:checks all:scripts all:bin security-tools.tsv VERSION
var files embed.FS

// Version is the payload version written to k_playbook.version. A project stores
// it so `restore` after a git clone knows which payload it expects.
func Version() string {
	data, err := files.ReadFile("VERSION")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}

// FS exposes the embedded tree for callers that only want to read.
func FS() fs.FS { return files }

// executable reports whether an extracted file needs the executable bit. embed.FS
// does not preserve permissions, so this has to be derived from the path.
func executable(name string) bool {
	return strings.HasPrefix(name, "bin/") ||
		strings.HasPrefix(name, "scripts/") ||
		strings.HasSuffix(name, ".sh")
}

// Extract writes the payload to dest, replacing it wholesale.
//
// Replacing rather than merging is the guarantee behind the _dist contract: a
// file removed from the payload must disappear from the project, and a file
// edited in the project must not survive an update. Everything outside dest is
// left untouched.
func Extract(dest string) error {
	if err := validateDest(dest); err != nil {
		return err
	}

	staging := dest + ".tmp"
	if err := os.RemoveAll(staging); err != nil {
		return fmt.Errorf("staging aufraeumen: %w", err)
	}

	if err := writeTree(staging); err != nil {
		os.RemoveAll(staging)
		return err
	}

	previous := dest + ".old"
	os.RemoveAll(previous)
	if _, err := os.Stat(dest); err == nil {
		// Swap through a temporary name so a crash mid-update cannot leave the
		// project without a usable _dist.
		if err := os.Rename(dest, previous); err != nil {
			os.RemoveAll(staging)
			return fmt.Errorf("bisherige Installation beiseite legen: %w", err)
		}
	}

	if err := os.Rename(staging, dest); err != nil {
		os.Rename(previous, dest)
		os.RemoveAll(staging)
		return fmt.Errorf("Installation aktivieren: %w", err)
	}

	return os.RemoveAll(previous)
}

func writeTree(staging string) error {
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return fmt.Errorf("Installationsverzeichnis anlegen: %w", err)
	}

	return fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}

		target := filepath.Join(staging, filepath.FromSlash(name))
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := files.ReadFile(name)
		if err != nil {
			return fmt.Errorf("%s lesen: %w", name, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		mode := os.FileMode(0o644)
		if executable(name) {
			mode = 0o755
		}
		if err := os.WriteFile(target, data, mode); err != nil {
			return fmt.Errorf("%s schreiben: %w", name, err)
		}
		return nil
	})
}

// validateDest refuses destinations that would make Extract destructive beyond
// the installation directory. Extract removes dest entirely, so a caller passing
// a project root by mistake must not silently wipe it.
func validateDest(dest string) error {
	if strings.TrimSpace(dest) == "" {
		return fmt.Errorf("Zielverzeichnis fehlt")
	}
	abs, err := filepath.Abs(dest)
	if err != nil {
		return fmt.Errorf("Zielverzeichnis aufloesen: %w", err)
	}
	if abs == string(filepath.Separator) || filepath.Dir(abs) == abs {
		return fmt.Errorf("unzulaessiges Zielverzeichnis: %s", abs)
	}
	if home, err := os.UserHomeDir(); err == nil && abs == home {
		return fmt.Errorf("unzulaessiges Zielverzeichnis: %s", abs)
	}
	return nil
}
