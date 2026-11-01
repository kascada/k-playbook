package project

import (
	"context"
	"path/filepath"

	"github.com/kascada/k-playbook/installer/internal/health"
)

// Die Quelle anchor der Umgebungsprüfung: Sie meldet den Stand des Ankers
// .k-playbook/K-PLAYBOOK.yaml im Repository des Projekts, mit zwei Befunden:
//
//   - Der Anker liegt in einem Git-Worktree und ist dort weder versioniert
//     noch ignoriert. Das ist ein Zustand, kein vermuteter Umzug: Er trifft
//     auch ein frisch angelegtes Projekt vor dem ersten Commit und einen
//     Anker, der absichtlich lokal bleibt. Die Abhilfe nennt beide Wege.
//   - Am früheren Ort, K-PLAYBOOK.yaml direkt im Hauptverzeichnis, steht noch
//     eine Datei im Index. Discover hat sie beim ersten Aufruf des neuen
//     Programms nach .k-playbook/ gezogen (migrateConfig); der Index weiß
//     davon nichts, bis der Umzug committet ist. Nur dieser Befund spricht
//     vom Umzug.
//
// Die Quelle spricht nur bei positivem Befund. Still bleibt sie ohne Git,
// außerhalb eines Worktrees, bei project.vcs ungleich git, bei ignoriertem
// Anker — so im Quell-Repo von k-playbook — und bei jedem Fehler von git:
// Eine unbeantwortete Frage ist kein Befund. Dafür nutzt sie die Kette von
// mcpGitTracked, die genau diese Fälle schon auseinanderhält und measure
// erst ruft, wenn das Hauptverzeichnis nachweislich in einem Repository
// liegt.
func init() {
	health.Register(healthAnchor)
}

// anchorRepoPath ist der Anker relativ zum Hauptverzeichnis, wie git ihn
// in ls-files und check-ignore sieht.
var anchorRepoPath = filepath.Join(ProjectDirName, ConfigFileName)

func healthAnchor(_ context.Context, projectDir string) []health.Problem {
	var problems []health.Problem
	// Der Rückgabewert von mcpGitTracked spielt hier keine Rolle; gebraucht
	// wird nur, dass measure ausschließlich in einem Worktree läuft.
	mcpGitTracked(projectDir, func(ctx context.Context, _ string) bool {
		problems = anchorProblems(ctx, projectDir)
		return true
	})
	return problems
}

// anchorProblems misst im Hauptverzeichnis, denn dort liegt .k-playbook/ —
// nicht in project.repo_root, das daneben liegen kann.
func anchorProblems(ctx context.Context, projectDir string) []health.Problem {
	var problems []health.Problem
	if !gitLsFilesTracked(ctx, projectDir, anchorRepoPath) && !gitIgnored(ctx, projectDir, anchorRepoPath) {
		problems = append(problems, health.Problem{
			Level:   health.LevelWarning,
			Area:    health.AreaAnchor,
			Message: anchorRepoPath + " ist weder versioniert noch ignoriert.",
			Hint: "git add " + anchorRepoPath + " und committen, damit jeder Clone des Projekts den Anker hat; " +
				"soll die Datei absichtlich lokal bleiben, " + anchorRepoPath + " in .gitignore aufnehmen.",
		})
	}
	if gitIndexHas(ctx, projectDir, ConfigFileName) {
		problems = append(problems, health.Problem{
			Level: health.LevelWarning,
			Area:  health.AreaAnchor,
			Message: ConfigFileName + " steht am früheren Ort im Hauptverzeichnis noch im Index; " +
				"der Anker ist nach " + anchorRepoPath + " umgezogen.",
			Hint: "git rm --cached " + ConfigFileName + " ausführen und den Umzug committen (git add " + anchorRepoPath + ").",
		})
	}
	return problems
}

// gitIgnored fragt check-ignore in dir nach path: Exit 0 heißt ignoriert,
// Exit 1 nicht ignoriert, alles andere unbeantwortet — und damit ignoriert,
// denn die Quelle spricht nur bei sicherem Befund.
func gitIgnored(ctx context.Context, dir string, path string) bool {
	_, code, _ := runGit(ctx, dir, "check-ignore", "-q", "--", path)
	return code != 1
}

// gitIndexHas fragt ls-files --error-unmatch in dir nach path und zählt nur
// Exit 0 als Treffer. Anders als gitLsFilesTracked fällt eine unbeantwortete
// Frage hier auf „nicht im Index": Der Befund „alter Ort noch im Index" ist
// ein positiver, und den gibt es nur mit Antwort.
func gitIndexHas(ctx context.Context, dir string, path string) bool {
	_, code, _ := runGit(ctx, dir, "ls-files", "--error-unmatch", "--", path)
	return code == 0
}
