package project

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Stand des Quellcodes beim docs checkout (Task 195). Die Merkdatei hält
// Branch, Commit und die Prüfsummen der geänderten Dateien des Arbeitsbaums
// fest; checkout bei existing, der Prüfmodus und publish vergleichen den
// aktuellen Stand damit. Eine Abweichung wird gemeldet, nie verhindert; ohne
// git oder bei einer Merkdatei ohne Stand gibt es keinen Vergleich und keinen
// Fehler.

// docsStandMaxDirty begrenzt die festgehaltenen geänderten Dateien; die
// übrigen zählt dirtyTruncated, ohne Prüfsumme.
const docsStandMaxDirty = 500

// Die Gründe eines unvollständigen Vergleichs (DocsStandCheck.Incomplete).
const (
	// DocsStandDirtyTruncated: beim checkout oder jetzt mehr als
	// docsStandMaxDirty geänderte Dateien; die übrigen sind nicht geprüft.
	DocsStandDirtyTruncated = "dirty_truncated"
	// DocsStandCommitUnknown: der festgehaltene Commit lässt sich nicht mehr
	// auflösen; die zwischen den Commits geänderten Dateien fehlen in Files.
	DocsStandCommitUnknown = "commit_unknown"
)

// docsStand ist der Stand in der Merkdatei. Git ist false, wenn
// project.repoRoot kein git-Arbeitsbaum war. Dirty sind die geänderten und
// unverfolgten Dateien aus git status (Pfad ab der Wurzel des Repos, SHA-256
// des Inhalts, leer bei einer gelöschten Datei), sortiert nach Pfad,
// höchstens docsStandMaxDirty; DirtyTruncated zählt die übrigen.
type docsStand struct {
	Git            bool            `json:"git"`
	Branch         string          `json:"branch,omitempty"`
	Commit         string          `json:"commit,omitempty"`
	Dirty          []docsStandFile `json:"dirty,omitempty"`
	DirtyTruncated int             `json:"dirtyTruncated,omitempty"`
}

// docsStandFile ist eine geänderte Datei mit ihrer Prüfsumme.
type docsStandFile struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

// DocsStandPoint ist Branch und Commit eines Stands.
type DocsStandPoint struct {
	Branch string `json:"branch"`
	Commit string `json:"commit"`
}

// DocsStandCheck ist der Vergleich des aktuellen Stands mit dem beim letzten
// checkout. Compared ist false, wenn es nichts zu vergleichen gibt — kein
// Stand festgehalten (Merkdatei ohne stand oder beim checkout ohne git) oder
// jetzt kein git; Message sagt dann warum. Changed ist wahr, wenn Branch
// oder Commit abweichen oder sich die Menge oder eine Prüfsumme der
// geänderten Dateien unterscheidet; Files nennt die betroffenen Pfade (ab
// der Wurzel des Repos). Incomplete nennt, warum der Vergleich nicht alles
// prüfen konnte (dirty_truncated, commit_unknown) — kein Fehler; Changed
// false mit Incomplete heißt nur, dass im Geprüften nichts abweicht.
type DocsStandCheck struct {
	Compared   bool            `json:"compared"`
	Changed    bool            `json:"changed"`
	Files      []string        `json:"files,omitempty"`
	Recorded   *DocsStandPoint `json:"recorded,omitempty"`
	Current    *DocsStandPoint `json:"current,omitempty"`
	Incomplete []string        `json:"incomplete,omitempty"`
	Message    string          `json:"message,omitempty"`
}

// docsGit führt einen git-Aufruf mit der Zeitgrenze eines Aufrufs aus.
func docsGit(ctx context.Context, dir string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, gitCallTimeout)
	defer cancel()
	out, code, _ := runGit(ctx, dir, args...)
	return out, code == 0
}

// docsRepoRoot ist project.repoRoot; ohne lesbare Konfiguration das
// Projektverzeichnis.
func docsRepoRoot(projectDir string) string {
	config, err := ReadConfig(projectDir)
	if err != nil {
		return projectDir
	}
	return RepoRootDir(projectDir, config)
}

// readDocsStand erhebt den Stand in repoRoot: Branch, vollen Commit und je
// Datei aus git status --porcelain --untracked-files=all Pfad und Prüfsumme.
// Ohne git ist Git false.
func readDocsStand(ctx context.Context, repoRoot string) docsStand {
	top, ok := docsGit(ctx, repoRoot, "rev-parse", "--show-toplevel")
	if !ok {
		return docsStand{}
	}
	top = strings.TrimSpace(top)
	stand := docsStand{Git: true}
	if branch, ok := docsGit(ctx, repoRoot, "rev-parse", "--abbrev-ref", "HEAD"); ok {
		stand.Branch = strings.TrimSpace(branch)
	}
	if commit, ok := docsGit(ctx, repoRoot, "rev-parse", "--verify", "--quiet", "HEAD"); ok {
		stand.Commit = strings.TrimSpace(commit)
	}
	status, ok := docsGit(ctx, repoRoot, "status", "--porcelain", "-z", "--untracked-files=all")
	if !ok {
		return stand
	}
	paths := docsStatusPaths(status)
	for i, rel := range paths {
		if i >= docsStandMaxDirty {
			stand.DirtyTruncated = len(paths) - docsStandMaxDirty
			break
		}
		stand.Dirty = append(stand.Dirty, docsStandFile{Path: rel, Hash: docsFileHash(filepath.Join(top, filepath.FromSlash(rel)))})
	}
	return stand
}

// docsStatusPaths liest die Pfade aus git status --porcelain -z: je Eintrag
// „XY pfad“, bei Umbenennen und Kopieren folgt der alte Pfad als eigenes
// Feld — im Index (X) wie im Arbeitsbaum (Y, etwa nach mv und git add -N).
// Beide Pfade zählen; zurück kommen sie sortiert und ohne Doppel.
func docsStatusPaths(status string) []string {
	fields := strings.Split(status, "\x00")
	var paths []string
	for i := 0; i < len(fields); i++ {
		field := fields[i]
		if len(field) < 4 {
			continue
		}
		paths = append(paths, field[3:])
		if strings.ContainsAny(field[:2], "RC") && i+1 < len(fields) {
			i++
			paths = append(paths, fields[i])
		}
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

// docsFileHash ist der SHA-256 des Inhalts einer Datei, bei einem Link der
// seines Ziels; leer, wenn die Datei fehlt oder keine gewöhnliche Datei ist.
func docsFileHash(file string) string {
	info, err := os.Lstat(file)
	if err != nil {
		return ""
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(file)
		if err != nil {
			return ""
		}
		return docsHash([]byte("link:" + target))
	}
	if !info.Mode().IsRegular() {
		return ""
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return ""
	}
	return docsHash(data)
}

// compareDocsStand vergleicht den aktuellen Stand in repoRoot mit dem
// festgehaltenen. Ein Fehler von git endet nie als Fehler, sondern als
// Message oder Incomplete.
func compareDocsStand(ctx context.Context, repoRoot string, recorded *docsStand) *DocsStandCheck {
	switch {
	case recorded == nil:
		return &DocsStandCheck{Message: "kein Stand festgehalten — die Merkdatei stammt von einem checkout ohne Stand; kein Vergleich"}
	case !recorded.Git:
		return &DocsStandCheck{Message: "kein Stand festgehalten — beim checkout war project.repoRoot kein git-Arbeitsbaum; kein Vergleich"}
	}
	current := readDocsStand(ctx, repoRoot)
	if !current.Git {
		return &DocsStandCheck{Message: "project.repoRoot ist jetzt kein git-Arbeitsbaum; kein Vergleich"}
	}
	check := &DocsStandCheck{Compared: true,
		Recorded: &DocsStandPoint{Branch: recorded.Branch, Commit: recorded.Commit},
		Current:  &DocsStandPoint{Branch: current.Branch, Commit: current.Commit}}
	files := map[string]bool{}
	if recorded.Branch != current.Branch || recorded.Commit != current.Commit {
		check.Changed = true
	}
	if recorded.Commit != current.Commit && recorded.Commit != "" && current.Commit != "" {
		if _, ok := docsGit(ctx, repoRoot, "rev-parse", "--verify", "--quiet", recorded.Commit+"^{commit}"); !ok {
			check.Incomplete = append(check.Incomplete, DocsStandCommitUnknown)
		} else if diff, ok := docsGit(ctx, repoRoot, "diff", "--name-only", "-z", recorded.Commit, current.Commit); ok {
			for _, rel := range strings.Split(diff, "\x00") {
				if rel != "" {
					files[rel] = true
				}
			}
		} else {
			check.Incomplete = append(check.Incomplete, DocsStandCommitUnknown)
		}
	}
	if recorded.DirtyTruncated > 0 || current.DirtyTruncated > 0 {
		check.Incomplete = append(check.Incomplete, DocsStandDirtyTruncated)
	}
	for rel := range dirtyDifferences(*recorded, current) {
		files[rel] = true
		check.Changed = true
	}
	for rel := range files {
		check.Files = append(check.Files, rel)
	}
	slices.Sort(check.Files)
	return check
}

// dirtyDifferences sind die Pfade, die nur auf einer Seite geändert sind oder
// deren Prüfsumme abweicht. Ist eine Seite gekappt, zählt nur, was auf ihr
// bis zum letzten festgehaltenen Pfad liegt: Dahinter fehlt die Prüfsumme,
// und ein Pfad dort wäre keine belegte Abweichung.
func dirtyDifferences(recorded, current docsStand) map[string]bool {
	index := func(stand docsStand) (map[string]string, string) {
		hashes := map[string]string{}
		for _, file := range stand.Dirty {
			hashes[file.Path] = file.Hash
		}
		limit := ""
		if stand.DirtyTruncated > 0 && len(stand.Dirty) > 0 {
			limit = stand.Dirty[len(stand.Dirty)-1].Path
		}
		return hashes, limit
	}
	before, beforeLimit := index(recorded)
	after, afterLimit := index(current)
	covered := func(rel string) bool {
		return (beforeLimit == "" || rel <= beforeLimit) && (afterLimit == "" || rel <= afterLimit)
	}
	out := map[string]bool{}
	for rel, hash := range before {
		if other, ok := after[rel]; (!ok || other != hash) && covered(rel) {
			out[rel] = true
		}
	}
	for rel := range after {
		if _, ok := before[rel]; !ok && covered(rel) {
			out[rel] = true
		}
	}
	return out
}
