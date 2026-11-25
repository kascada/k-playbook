package project

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Die Vorprüfung der Befund-Queue (Task 166): Bevor /k-befunde-einordnen den
// ersten Befund anfasst, muss feststehen, dass die Replica frisch ist, die
// Rechte zum Löschen fremder Befunde da sind und jeder Eintrag unter
// queue/findings/ ein lesbarer Befund ist. Das ist mechanisch — über 200
// Namen, die Zählung je Nutzer — und gehört deshalb ins Programm, nicht in
// den Command-Text. Hüllen: `k-playbook findings check` und das MCP-Werkzeug
// k_playbook_findings_check.
//
// Geprüft wird in dieser Reihenfolge, und jede Stufe sammelt ihre Probleme,
// statt beim ersten zu enden — der Command meldet alles auf einmal, samt Zahl
// je Nutzer:
//
//  1. Replica abgleichen: das MCP-Werkzeug sync des Nodes über den Zugang aus
//     der Auskunft von Kephalaion (kephalaion.Client.SyncHub). Ohne frischen
//     Abgleich fehlt, was ein anderer Rechner gerade angelegt hat. Kein
//     Prozess und keine Datenbank des Nodes auf diesem Rechner: Die Stufe
//     geht auch in einem reinen Client, der nur die config des Hosts und
//     einen Weg zu dessen Node hat.
//  2. Anbindung und Rechte: read, write, supersede. Ohne supersede scheitert
//     das Löschen der Befunde anderer Nutzer.
//  3. Die Liste unter queue/findings/: Jeder Eintrag folgt dem Namensmuster
//     der Sorte findings und liegt direkt in der Ablage.
//  4. Jeden Befund lesen. Ein leerer oder in den letzten sechs Stunden
//     geänderter Befund wird übersprungen (skipped), nicht als Problem gezählt:
//     k_playbook_doc_create legt ihn zuerst leer an, der Inhalt folgt im zweiten
//     Aufruf, und eine Sitzung kann ihn noch nachtragen.

// FindingsKind ist der Name der Sorte, deren Ablage die Vorprüfung liest.
const FindingsKind = "findings"

// FindingsRequiredRights sind die Rechte auf die Collection, ohne die die
// Einordnung nicht anfängt.
var FindingsRequiredRights = []string{"read", "write", "supersede"}

// FindingsRecentWindow ist das Zeitfenster für „jünger als 6 h“: Ein Befund,
// dessen letzte Änderung (updated) jünger ist, kann noch geschrieben werden und
// kommt in einem späteren Lauf dran. Maßstab ist die letzte Änderung, nicht das
// Anlegen: Sie liegt nie davor, und ein Nachtrag setzt die Wartezeit neu, damit
// nie gelesen wird, woran noch jemand schreibt.
const FindingsRecentWindow = 6 * time.Hour

// Die Arten eines Problems der Vorprüfung (FindingsProblem.Kind).
const (
	// FindingsProblemKephalaion: Die Anbindung ist nicht bereit, oder die
	// Collection lässt sich nicht öffnen oder listen.
	FindingsProblemKephalaion = "kephalaion"
	// FindingsProblemSync: Der Abgleich der Replica ist gescheitert, nicht
	// bestätigt oder nicht rechtzeitig beendet, der Node ist zu alt für das
	// Werkzeug sync, oder der Zugang zum Node ließ sich dafür nicht öffnen.
	FindingsProblemSync = "sync"
	// FindingsProblemRight: Ein Recht aus FindingsRequiredRights fehlt; Name
	// ist das Recht.
	FindingsProblemRight = "right"
	// FindingsProblemName: Ein Eintrag direkt unter queue/findings/ folgt
	// nicht dem Namensmuster der Sorte.
	FindingsProblemName = "name"
	// FindingsProblemSubdir: Ein Eintrag liegt in einem Unterverzeichnis von
	// queue/findings/.
	FindingsProblemSubdir = "subdir"
	// FindingsProblemUnreadable: Der Befund ließ sich nicht lesen.
	FindingsProblemUnreadable = "unreadable"
	// FindingsProblemFrontmatter: Kephalaion konnte das Frontmatter nicht
	// lesen.
	FindingsProblemFrontmatter = "frontmatter"
)

// Die Gründe, aus denen ein Befund übersprungen wird (FindingsSkipped.Reason).
// Sie sind zugleich der Text der Protokollzeile „übersprungen (<grund>)“.
const (
	FindingsSkipEmpty  = "leer"
	FindingsSkipRecent = "jünger als 6 h"
)

// findingNamePattern ist das Namensmuster der Sorte findings: dreistellige
// (oder längere) Nummer, Bindestrich, Kurzname nach NormalizeDocName, .md —
// wie docFileName es bildet.
var findingNamePattern = regexp.MustCompile(`^[0-9]{3,}-[a-z0-9]+(-[a-z0-9]+)*\.md$`)

// FindingsProblem ist ein Grund, warum die Einordnung nicht anfängt.
type FindingsProblem struct {
	Kind string `json:"kind"`
	// Name ist der volle Name in der Collection, bei right das Recht; leer,
	// wo es um keinen einzelnen Eintrag geht.
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

// FindingsSync ist der Stand des Abgleichs nach der Vorprüfung, wie der Node
// ihn meldet.
type FindingsSync struct {
	At       string `json:"at,omitempty"`
	Revision *int64 `json:"revision,omitempty"`
}

// FindingItem ist ein Befund, wie die Vorprüfung ihn gelesen hat.
type FindingItem struct {
	// Name ist der volle Name in der Collection, etwa
	// queue/findings/012-x.md — so, wie Kephalaion delete und rename ihn
	// brauchen.
	Name string `json:"name"`
	Ref  string `json:"ref"`
	// Revision ist die Revision beim Lesen, für base_revision.
	Revision int64 `json:"revision"`
	// User ist, wer den Befund angelegt hat (Autor-Metadaten von Kephalaion).
	User    string `json:"user"`
	Created string `json:"created"`
	Updated string `json:"updated"`
	// Ziel ist das Feld ziel des Frontmatters; leer, wenn es fehlt.
	Ziel string `json:"ziel"`
	// Thema ist das Feld thema des Frontmatters; leer, wenn es fehlt. Die
	// Einordnung nimmt Befunde mit demselben Thema direkt nacheinander.
	Thema string `json:"thema"`
}

// FindingsSkipped ist ein Befund, den die Einordnung diesmal überspringt.
type FindingsSkipped struct {
	FindingItem
	Reason string `json:"reason"`
}

// FindingsCheck ist das Ergebnis der Vorprüfung. Findings und Skipped sind
// getrennte Mengen; Total und ByUser zählen beide.
type FindingsCheck struct {
	Ready    bool              `json:"ready"`
	Problems []FindingsProblem `json:"problems"`
	Sync     *FindingsSync     `json:"sync,omitempty"`
	Rights   []string          `json:"rights"`
	Total    int               `json:"total"`
	ByUser   map[string]int    `json:"byUser"`
	Skipped  []FindingsSkipped `json:"skipped"`
	Findings []FindingItem     `json:"findings"`
}

// FindingsStore sind die Aufrufe an Kephalaion, die die Vorprüfung braucht.
// *kephalaion.Client erfüllt sie; Tests setzen eine Fälschung ein.
type FindingsStore interface {
	List(ctx context.Context, req kephalaion.ListRequest) ([]kephalaion.Entry, error)
	Read(ctx context.Context, name string) (kephalaion.ReadResult, error)
}

// findingsEnv sind die Zugänge der Vorprüfung, austauschbar für Tests.
type findingsEnv struct {
	binding func() (kephalaion.Binding, error)
	sync    func(ctx context.Context, binding kephalaion.Binding) (kephalaion.SyncState, error)
	status  func() kephalaion.Status
	open    func(status kephalaion.Status) (FindingsStore, error)
	now     func() time.Time
}

// CheckFindings führt die Vorprüfung für das Projekt aus. Sie scheitert nie:
// Jeder Fehler wird ein Problem, und Ready ist genau dann wahr, wenn es
// keines gibt. Geschrieben wird nichts — außer der Replica des Nodes, die
// der Abgleich nachzieht.
func CheckFindings(ctx context.Context, projectDir string) FindingsCheck {
	return findingsEnv{
		binding: func() (kephalaion.Binding, error) { return ProjectBinding(projectDir) },
		sync:    syncBinding,
		status:  func() kephalaion.Status { return KephalaionStatus(projectDir) },
		open: func(status kephalaion.Status) (FindingsStore, error) {
			client, _, err := openKephClientStatus(projectDir, "", status)
			if err != nil {
				return nil, err
			}
			return client, nil
		},
		now: time.Now,
	}.check(ctx)
}

func (env findingsEnv) check(ctx context.Context) (result FindingsCheck) {
	result = FindingsCheck{Problems: []FindingsProblem{}, Rights: []string{}, ByUser: map[string]int{},
		Skipped: []FindingsSkipped{}, Findings: []FindingItem{}}
	problem := func(kind, name, message string) {
		result.Problems = append(result.Problems, FindingsProblem{Kind: kind, Name: name, Message: message})
	}
	// Ready folgt allein aus den Problemen, auch bei einem frühen Ende.
	defer func() { result.Ready = len(result.Problems) == 0 }()

	// 1. Abgleich. Ohne Hub gibt es nichts abzugleichen; das meldet die
	// Anbindung in Stufe 2.
	binding, err := env.binding()
	switch {
	case err != nil:
		problem(FindingsProblemKephalaion, "", err.Error())
		return result
	case binding.Hub != "":
		if state, err := env.sync(ctx, binding); err != nil {
			problem(FindingsProblemSync, "", err.Error())
		} else {
			revision := state.Revision
			result.Sync = &FindingsSync{At: state.LastSuccess, Revision: &revision}
		}
	}

	// 2. Anbindung und Rechte. Den Stand des Abgleichs nennt sonst der Node.
	status := env.status()
	if result.Sync == nil && status.Sync != nil {
		result.Sync = &FindingsSync{At: status.Sync.LastSuccess, Revision: status.Sync.Revision}
	}
	if !status.OK {
		problem(FindingsProblemKephalaion, "", "Kephalaion ist nicht bereit: "+status.Message)
		return result
	}
	result.Rights = append(result.Rights, status.Rights...)
	for _, right := range FindingsRequiredRights {
		if !slices.Contains(status.Rights, right) {
			problem(FindingsProblemRight, right, fmt.Sprintf(
				"Das Recht %s auf %s fehlt (vorhanden: %s). Abhilfe: beim Admin des Hubs das Recht %s für den User %s erbitten.",
				right, status.Address, strings.Join(status.Rights, ", "), right, status.User))
		}
	}
	store, err := env.open(status)
	if err != nil {
		problem(FindingsProblemKephalaion, "", err.Error())
		return result
	}

	// 3. Die Liste. Ein leeres oder fehlendes Verzeichnis ist kein Fehler:
	// Kephalaion speichert keine leeren Verzeichnisse.
	dir := findingsDir()
	entries, err := store.List(ctx, kephalaion.ListRequest{Path: dir, Recursive: true, Frontmatter: true})
	if err != nil {
		problem(FindingsProblemKephalaion, "", fmt.Sprintf("%s/ lässt sich nicht listen: %v", dir, kephError(err)))
		return result
	}
	now := env.now()
	for _, entry := range entries {
		rest, ok := strings.CutPrefix(entry.Name, dir+"/")
		switch {
		case !ok:
			continue
		case strings.Contains(rest, "/"):
			problem(FindingsProblemSubdir, entry.Name, fmt.Sprintf(
				"%s liegt in einem Unterverzeichnis von %s/; dort gehören nur Befunde hin.", entry.Name, dir))
			continue
		case !findingNamePattern.MatchString(rest):
			problem(FindingsProblemName, entry.Name, fmt.Sprintf(
				"%s folgt nicht dem Namensmuster eines Befunds (<nummer>-<kurzname>.md).", entry.Name))
			continue
		}

		// 4. Lesen.
		item := FindingItem{Name: entry.Name, Ref: DocSchemeKeph + ":/" + entry.Name, Revision: entry.Revision}
		if entry.Created != nil {
			item.Created, item.User = entry.Created.At, entry.Created.By
		}
		if entry.Updated != nil {
			item.Updated = entry.Updated.At
		}
		if ziel, ok := entry.Frontmatter["ziel"].(string); ok {
			item.Ziel = strings.TrimSpace(ziel)
		}
		if thema, ok := entry.Frontmatter["thema"].(string); ok {
			item.Thema = strings.TrimSpace(thema)
		}
		read, err := store.Read(ctx, entry.Name)
		switch {
		case err != nil:
			problem(FindingsProblemUnreadable, entry.Name, fmt.Sprintf("%s lässt sich nicht lesen: %v", entry.Name, kephError(err)))
			continue
		case read.Kind != kephalaion.ReadKindDocument || read.Content == nil:
			problem(FindingsProblemUnreadable, entry.Name, fmt.Sprintf(
				"%s steht in der Liste, lässt sich aber nicht lesen (Art %q) — gerade gelöscht oder verschoben?", entry.Name, read.Kind))
			continue
		}
		if read.Revision > 0 {
			item.Revision = read.Revision
		}
		if read.Updated != nil && read.Updated.At != "" {
			item.Updated = read.Updated.At
		}
		if item.User != "" {
			result.ByUser[item.User]++
		} else {
			result.ByUser["?"]++
		}
		result.Total++
		switch {
		case strings.TrimSpace(*read.Content) == "":
			result.Skipped = append(result.Skipped, FindingsSkipped{FindingItem: item, Reason: FindingsSkipEmpty})
		case findingRecent(item.Updated, now):
			result.Skipped = append(result.Skipped, FindingsSkipped{FindingItem: item, Reason: FindingsSkipRecent})
		case entry.FrontmatterError != "":
			problem(FindingsProblemFrontmatter, entry.Name, fmt.Sprintf(
				"%s: Frontmatter nicht lesbar: %s", entry.Name, entry.FrontmatterError))
		default:
			result.Findings = append(result.Findings, item)
		}
	}
	sortFindings(result.Findings, func(i int) FindingItem { return result.Findings[i] })
	sortFindings(result.Skipped, func(i int) FindingItem { return result.Skipped[i].FindingItem })
	return result
}

// syncBinding fordert den Abgleich für den Hub der Anbindung beim Node an,
// mit Endpunkt, Account und Token auf demselben Weg wie die übrigen Aufrufe
// (kephalaionAccess, kephalaion.NewClient). Lässt sich der Zugang nicht
// öffnen, ist das ein Fehler des Abgleichs; Stufe 2 meldet dieselbe Ursache
// womöglich noch einmal als Anbindung.
func syncBinding(ctx context.Context, binding kephalaion.Binding) (kephalaion.SyncState, error) {
	access, err := kephalaionAccess(binding.Hub)
	if err != nil {
		return kephalaion.SyncState{}, fmt.Errorf("Abgleich nicht möglich: kein Zugang zum Node für den Hub %s: %v", binding.Hub, err)
	}
	client, err := kephalaion.NewClient(binding, access)
	if err != nil {
		return kephalaion.SyncState{}, fmt.Errorf("Abgleich nicht möglich: %v", err)
	}
	return client.SyncHub(ctx)
}

// findingsDir ist die Ablage der Sorte findings.
func findingsDir() string {
	for _, kind := range DocKinds {
		if kind.Kind == FindingsKind {
			return kind.Dir
		}
	}
	panic("Sorte " + FindingsKind + " fehlt in DocKinds")
}

// findingRecent sagt, ob ein Zeitpunkt jünger ist als FindingsRecentWindow.
// Ein unlesbarer Zeitpunkt gilt nicht als kürzlich.
func findingRecent(at string, now time.Time) bool {
	if at == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return false
	}
	return now.Sub(t) < FindingsRecentWindow
}

// sortFindings ordnet nach Alter: angelegt aufsteigend, bei Gleichstand nach
// Name.
func sortFindings[T any](items []T, at func(int) FindingItem) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := at(i), at(j)
		if a.Created != b.Created {
			return a.Created < b.Created
		}
		return a.Name < b.Name
	})
}
