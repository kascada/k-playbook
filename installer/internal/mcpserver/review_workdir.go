package mcpserver

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/kascada/k-playbook/installer/internal/project"
	"github.com/kascada/k-playbook/installer/internal/review/merge"
	"github.com/kascada/k-playbook/installer/internal/review/workdir"
)

// Die beiden Werkzeuge eines eigenständigen Reviews (/k-review, Task 197):
// das Arbeitsverzeichnis bestimmen und die Fundlisten darin zusammenfassen.
// Sie legen keinen Audit-Lauf an und lesen keinen und stehen deshalb nicht
// hinter review.Paused; die Hüllen über dieselben Funktionen sind
// `k-playbook review workdir` und `k-playbook review merge`.

const (
	reviewToolWorkdir  = "k_playbook_review_workdir"
	reviewToolMergeDir = "k_playbook_review_merge_dir"
)

type reviewWorkdirInput struct {
	reviewBaseInput
	Recipe string `json:"recipe" jsonschema:"Pflicht. Der Schlüssel des Rezepts, etwa code oder tech; er steht im Namen des Standardverzeichnisses."`
	Dir    string `json:"dir,omitempty" jsonschema:"Optional. Ein absoluter Pfad, der als Arbeitsverzeichnis gilt; fehlt er, legt das Werkzeug mit 0700 ein neues unmittelbar unter /tmp an."`
}

type reviewMergeDirInput struct {
	reviewBaseInput
	Dir   string `json:"dir" jsonschema:"Pflicht. Absoluter Pfad des Arbeitsverzeichnisses mit den Fundlisten <eintrag>.sarif."`
	Check bool   `json:"check,omitempty" jsonschema:"Nur prüfen, nichts schreiben: der Abschluss des Erhebens."`
}

func addReviewWorkdirTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name: reviewToolWorkdir,
		Description: "Bestimmt das Arbeitsverzeichnis eines eigenständigen Reviews (/k-review). Ohne dir legt es ein neues Verzeichnis " +
			"unmittelbar unter /tmp an ($TMPDIR gilt nicht), mit Projekt, Rezept, Zeitstempel und zufälligem Teil " +
			"im Namen und den Rechten 0700; mit dir nimmt es diesen absoluten Pfad, legt ihn mit 0700 an, wenn er fehlt, und warnt " +
			"(warnings), wenn er im Projekt liegt oder für andere zugänglich ist. Antwort: dir, created, default, insideProject. " +
			"Legt keinen Audit-Lauf an.",
	}, reviewWorkdirTool)
	mcp.AddTool(server, &mcp.Tool{
		Name: reviewToolMergeDir,
		Description: "Fasst die Fundlisten eines eigenständigen Reviews zusammen: liest jede Datei <eintrag>.sarif unmittelbar im " +
			"Arbeitsverzeichnis dir, prüft sie gegen den Vertrag des Rezepts mit diesem Schlüssel (tool.driver.name gleich Eintrag, " +
			"jede ruleId aus audit.ruleIds, Funde außerhalb audit.scope.paths werden verworfen und gezählt) und schreibt " +
			"review-input.json und review-input.md dorthin, mit den Rechten 0600 — Gruppen, stabile IDs und Schwere wie im Audit, " +
			"ohne known-decisions. Mit check nur die Prüfung, nichts geschrieben. Eine ungültige Fundliste: sarif_invalid mit jeder " +
			"Datei und ihrem Grund, nichts geschrieben. Legt keinen Audit-Lauf an und liest keinen.",
	}, reviewMergeDirTool)
}

func reviewWorkdirTool(ctx context.Context, req *mcp.CallToolRequest, input reviewWorkdirInput) (*mcp.CallToolResult, any, error) {
	root, envelope, toolErr := resolveReviewProject(input.ProjectDir)
	if toolErr.Code != "" {
		return reviewErrorResult(reviewToolWorkdir, envelope, toolErr), nil, nil
	}
	if strings.TrimSpace(input.Recipe) == "" {
		return reviewErrorResult(reviewToolWorkdir, envelope, reviewToolError{Code: "invalid_input", Message: "recipe fehlt."}), nil, nil
	}
	if input.Dir != "" && !filepath.IsAbs(input.Dir) {
		return reviewErrorResult(reviewToolWorkdir, envelope, reviewToolError{
			Code: "invalid_input", Message: "dir muss ein absoluter Pfad sein.", Details: map[string]any{"dir": input.Dir},
		}), nil, nil
	}
	prepared, err := workdir.Prepare(root, input.Recipe, input.Dir, time.Now())
	if err != nil {
		return reviewErrorResult(reviewToolWorkdir, envelope, wrapError(err, "workdir_failed",
			"Das Arbeitsverzeichnis ließ sich nicht bestimmen.", map[string]any{"dir": input.Dir})), nil, nil
	}
	return reviewSuccessResult(reviewToolWorkdir, envelope, prepared, prepared.Warnings), nil, nil
}

func reviewMergeDirTool(ctx context.Context, req *mcp.CallToolRequest, input reviewMergeDirInput) (*mcp.CallToolResult, any, error) {
	root, envelope, toolErr := resolveReviewProject(input.ProjectDir)
	if toolErr.Code != "" {
		return reviewErrorResult(reviewToolMergeDir, envelope, toolErr), nil, nil
	}
	if strings.TrimSpace(input.Dir) == "" || !filepath.IsAbs(input.Dir) {
		return reviewErrorResult(reviewToolMergeDir, envelope, reviewToolError{
			Code: "invalid_input", Message: "dir muss ein absoluter Pfad sein.", Details: map[string]any{"dir": input.Dir},
		}), nil, nil
	}
	output, err := workdir.Merge(root, input.Dir, input.Check, mcpKPlaybookVersion(), merge.SeverityCatalog(project.SharedDir()))
	if err != nil {
		var checkErr *merge.DirCheckError
		switch {
		case errors.As(err, &checkErr):
			return reviewErrorResult(reviewToolMergeDir, envelope, reviewToolError{
				Code: "sarif_invalid", Message: "Mindestens eine Fundliste erfüllt den Vertrag ihres Rezepts nicht; es wurde nichts geschrieben.",
				Details: map[string]any{"dir": input.Dir, "problems": checkErr.Problems},
			}), nil, nil
		case errors.Is(err, merge.ErrNoDirSARIF):
			return reviewErrorResult(reviewToolMergeDir, envelope, wrapError(err, "sarif_missing",
				"Im Arbeitsverzeichnis liegt keine Fundliste.", map[string]any{"dir": input.Dir})), nil, nil
		case errors.Is(err, os.ErrNotExist):
			return reviewErrorResult(reviewToolMergeDir, envelope, wrapError(err, "read_failed",
				"Das Arbeitsverzeichnis ist nicht lesbar.", map[string]any{"dir": input.Dir})), nil, nil
		default:
			return reviewErrorResult(reviewToolMergeDir, envelope, wrapError(err, "merge_failed",
				"Die Fundlisten ließen sich nicht zusammenfassen.", map[string]any{"dir": input.Dir})), nil, nil
		}
	}
	return reviewSuccessResult(reviewToolMergeDir, envelope, output, output.Warnings), nil, nil
}

// resolveReviewProject löst nur das Projekt auf — anders als
// resolveReviewEnvironment ohne Ergebnisverzeichnis, das dieser Weg nicht
// braucht.
func resolveReviewProject(inputDir string) (string, reviewProjectEnvelope, reviewToolError) {
	envelope := reviewProjectEnvelope{InputDir: inputDir}
	if err := checkProjectDir(inputDir); err != nil {
		return "", envelope, reviewToolError{Code: "invalid_input", Message: err.Error(), Details: map[string]any{}}
	}
	dir := filepath.Clean(inputDir)
	if !filepath.IsAbs(dir) {
		workdirPath, err := os.Getwd()
		if err != nil {
			return "", envelope, reviewToolError{Code: "project_not_found", Message: "Arbeitsverzeichnis nicht lesbar.", Details: map[string]any{"error": err.Error()}}
		}
		dir = filepath.Join(workdirPath, dir)
	}
	environment := project.DetectFrom(dir)
	if !environment.Installed {
		return "", envelope, reviewToolError{
			Code: "project_not_found", Message: "Kein k-playbook-Projekt gefunden.",
			Details: map[string]any{"searchedFrom": dir, "config": project.ConfigFileName},
		}
	}
	envelope.InputDir = dir
	envelope.Root = environment.ProjectDir
	return environment.ProjectDir, envelope, reviewToolError{}
}
