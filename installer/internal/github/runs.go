package github

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

// RunLimit ist die Zahl der Läufe, die die Karte zeigt. Die Ansicht beantwortet
// „ist es grün, und warum nicht" — sie baut GitHubs Verlauf nicht nach.
const RunLimit = 20

// Run ist ein Lauf, so wie ihn die Karte zeigt.
type Run struct {
	ID         int64  `json:"id"`
	Workflow   string `json:"workflow"`
	Title      string `json:"title"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	// Ref ist Branch oder Tag, IsTag sagt welches von beidem. `gh run list`
	// nennt beides `headBranch`; ohne die Unterscheidung sähe der Lauf zu
	// einem Tag aus wie ein Branch.
	Ref   string `json:"ref"`
	IsTag bool   `json:"isTag"`
	Event string `json:"event"`
	// Seconds ist die Dauer zwischen Start und letzter Änderung. 0 bei einem
	// Lauf, der noch läuft oder dessen Zeiten fehlen.
	Seconds   int    `json:"seconds"`
	CreatedAt string `json:"createdAt"`
	URL       string `json:"url"`
	// Failed sagt, ob die Karte für diesen Lauf die Ursache anbieten soll. Das
	// Log wird erst beim Aufklappen geholt, nie beim Laden der Seite.
	Failed bool `json:"failed"`
}

// Runs ist die Antwort der Lauf-Karte.
type Runs struct {
	Result
	Runs  []Run `json:"runs"`
	Limit int   `json:"limit"`
}

// FetchRuns liest die letzten Läufe. Ohne Log: das ist die teure Abfrage und
// läuft erst auf Anforderung.
func (c *Client) FetchRuns(ctx context.Context) Runs {
	raw, err := c.gh(ctx, "run", "list", "--limit", strconv.Itoa(RunLimit), "--json", runListFields)
	if err != nil {
		return Runs{Result: Classify(err), Runs: []Run{}, Limit: RunLimit}
	}
	return ParseRuns(raw)
}

// ParseRuns wertet `gh run list --json` aus. Eigene Funktion für die Tests.
func ParseRuns(raw []byte) Runs {
	runs := Runs{Result: Result{State: StateOK}, Runs: []Run{}, Limit: RunLimit}

	var entries []runListEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return Runs{
			Result: Result{State: StateError, Message: Explain(StateError) + " Die Antwort von gh war kein lesbares JSON."},
			Runs:   []Run{},
			Limit:  RunLimit,
		}
	}

	for _, entry := range entries {
		runs.Runs = append(runs.Runs, Run{
			ID:         entry.DatabaseID,
			Workflow:   entry.WorkflowName,
			Title:      entry.DisplayTitle,
			Status:     entry.Status,
			Conclusion: entry.Conclusion,
			Ref:        entry.HeadBranch,
			IsTag:      isTagEvent(entry),
			Event:      entry.Event,
			Seconds:    runSeconds(entry),
			CreatedAt:  entry.CreatedAt,
			URL:        entry.URL,
			Failed:     failedConclusion(entry.Conclusion),
		})
	}
	return runs
}

// isTagEvent erkennt den Lauf zu einem Tag. GitHub schreibt den Tagnamen in
// dasselbe Feld wie den Branch; erkennbar ist er am Ereignis `release` oder
// daran, dass der Name wie eine Version aussieht.
func isTagEvent(entry runListEntry) bool {
	if entry.Event == "release" {
		return true
	}
	ref := entry.HeadBranch
	if len(ref) < 2 || ref[0] != 'v' {
		return false
	}
	return ref[1] >= '0' && ref[1] <= '9'
}

func failedConclusion(conclusion string) bool {
	switch conclusion {
	case "failure", "timed_out", "startup_failure":
		return true
	}
	return false
}

func runSeconds(entry runListEntry) int {
	// Nur ein fertiger Lauf hat eine Dauer. Bei einem laufenden ist updatedAt
	// der Stand von eben, nicht das Ende — die Differenz wäre keine Laufzeit,
	// sondern das Alter der letzten Meldung.
	if entry.Status != "completed" {
		return 0
	}
	started, err := time.Parse(time.RFC3339, entry.StartedAt)
	// Ein Lauf, der noch nicht gestartet ist, trägt die Nullzeit. Die parst
	// fehlerfrei, ergäbe aber eine Dauer von rund zweitausend Jahren.
	if err != nil || started.IsZero() {
		return 0
	}
	finished, err := time.Parse(time.RFC3339, entry.UpdatedAt)
	if err != nil || finished.IsZero() {
		return 0
	}
	seconds := int(finished.Sub(started).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}
