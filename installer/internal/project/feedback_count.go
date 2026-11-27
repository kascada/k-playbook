package project

import (
	"context"
	"sort"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/kephalaion"
)

// Die Zählung über alle lesbaren Collections (Task 198, Etappe 3): Je
// Collection aller Hubs, die der Node für den Account des jeweiligen Hubs als
// lesbar meldet, die Zahl offener Meldungen unter feedback/ ohne done/,
// aufgeteilt nach betrifft. Eine Prüfung, wer der Aufrufer ist, gibt es
// nicht: Die Sichtbarkeit folgt allein den Leserechten in Kephalaion. Die
// eigene Collection fehlt (sie listet ListFeedback ganz), die öffentliche
// <hub>:k-playbook ebenso — dort meldet niemand außer dem Admin.
//
// Zu den Verzeichnis-Scopes (dirs in whoami): Sie erweitern in Kephalaion nur
// das Schreiben unter einem Verzeichnis und schränken nichts ein; gelesen
// wird eine Collection mit dem Recht read als Ganzes. Ob feedback/ lesbar ist,
// entscheidet deshalb read auf die Collection — und scheitert das Lesen
// trotzdem, steht die Collection mit ihrem Fehler da, nicht mit 0.

// FeedbackCollectionCount ist die Zählung einer anderen Collection.
type FeedbackCollectionCount struct {
	// Address ist <hub>:<collection>; bei einem Hub, der als Ganzes
	// scheitert, <hub>: ohne Collection.
	Address string `json:"address"`
	// Open ist die Zahl der Meldungen mit status: offen unter feedback/.
	Open int `json:"open"`
	// ByBetrifft teilt Open nach betrifft auf; ein fehlender Wert zählt als
	// „unbekannt“.
	ByBetrifft map[string]int `json:"byBetrifft"`
	// Unreadable ist die Zahl der Meldungen, deren Kopf der Node nicht lesen
	// konnte; sie stecken in keiner Zahl darüber.
	Unreadable int `json:"unreadable,omitempty"`
	// Error nennt, woran das Lesen scheiterte; dann sind die Zahlen leer.
	Error string `json:"error,omitempty"`
}

// feedbackLister ist der Aufruf, den die Zählung an einer Collection braucht.
type feedbackLister interface {
	List(ctx context.Context, req kephalaion.ListRequest) ([]kephalaion.Entry, error)
}

// FeedbackSource ist eine Collection, die gezählt wird: Adresse und Zugang,
// oder der Fehler, an dem schon das Öffnen scheiterte.
type FeedbackSource struct {
	Address string
	Store   feedbackLister
	Err     error
}

// feedbackTarget ist eine Collection, die nach dem Abgleich mit den
// Leserechten gezählt werden soll.
type feedbackTarget struct {
	hub, collection, address string
}

// feedbackTargets wählt aus den Hubs der Auskunft die Collections, die
// gezählt werden: alle lesbaren außer der eigenen und der öffentlichen
// k-playbook. Ein Hub, der nicht angemeldet ist, kommt als Fehler.
func feedbackTargets(probes []kephalaion.HubProbe, own string) ([]feedbackTarget, []FeedbackCollectionCount) {
	var targets []feedbackTarget
	var failed []FeedbackCollectionCount
	for _, hub := range probes {
		if hub.State != kephalaion.StateOK {
			failed = append(failed, FeedbackCollectionCount{Address: hub.Hub + ":", Error: hub.Message})
			continue
		}
		for _, col := range hub.Collections {
			if col.Address == own || col.Collection == DefaultsCollectionName {
				continue
			}
			readable := false
			for _, right := range col.Rights {
				if right == "read" {
					readable = true
				}
			}
			if readable {
				targets = append(targets, feedbackTarget{hub: hub.Hub, collection: col.Collection, address: col.Address})
			}
		}
	}
	return targets, failed
}

// FeedbackSources liefert die Collections, die gezählt werden — je Hub mit
// dessen Account, wie ProbeHubs ihn wählt; ein einzelnes whoami nennt die
// anderen Hubs nicht. Austauschbar, damit Tests den Node nicht fragen.
var FeedbackSources = func(ctx context.Context, own string) ([]FeedbackSource, error) {
	info, probes, err := kephalaion.ProbeHubs(ctx)
	if err != nil {
		return nil, err
	}
	targets, failed := feedbackTargets(probes, own)
	sources := make([]FeedbackSource, 0, len(targets)+len(failed))
	for _, entry := range failed {
		sources = append(sources, FeedbackSource{Address: entry.Address, Err: errorText(entry.Error)})
	}
	accesses := map[string]kephalaion.Access{}
	for _, target := range targets {
		access, ok := accesses[target.hub]
		if !ok {
			resolved, err := kephalaion.ResolveInfo(info, target.hub)
			if err != nil {
				sources = append(sources, FeedbackSource{Address: target.address, Err: err})
				continue
			}
			access, accesses[target.hub] = resolved, resolved
		}
		client, err := kephalaion.NewClient(kephalaion.Binding{Hub: target.hub, Collection: target.collection}, access)
		if err != nil {
			sources = append(sources, FeedbackSource{Address: target.address, Err: err})
			continue
		}
		sources = append(sources, FeedbackSource{Address: target.address, Store: client})
	}
	return sources, nil
}

// errorText ist ein Fehler aus einem fertigen Satz.
type errorText string

func (e errorText) Error() string { return string(e) }

// countFeedback zählt die offenen Meldungen jeder Quelle. Eine Quelle, die
// scheitert, steht mit ihrem Fehler da, ohne den Rest abzubrechen; eine ohne
// feedback/ zählt 0 (list liefert für ein fehlendes Verzeichnis eine leere
// Liste).
func countFeedback(ctx context.Context, sources []FeedbackSource) []FeedbackCollectionCount {
	kind := DocKind{Kind: FeedbackKindName, Dir: "feedback", Done: true}
	for _, candidate := range DocKinds {
		if candidate.Kind == FeedbackKindName {
			kind = candidate
		}
	}
	counts := make([]FeedbackCollectionCount, 0, len(sources))
	for _, source := range sources {
		count := FeedbackCollectionCount{Address: source.Address, ByBetrifft: map[string]int{}}
		if source.Err != nil {
			count.Error = source.Err.Error()
			counts = append(counts, count)
			continue
		}
		entries, err := source.Store.List(ctx, kephalaion.ListRequest{Path: kind.Dir, Recursive: true, Frontmatter: true})
		if err != nil {
			count.Error = kephError(err).Error()
			counts = append(counts, count)
			continue
		}
		for _, entry := range entries {
			m, ok := kind.match(entry.Name)
			if !ok || m.done {
				continue
			}
			if _, ok := docNumber(m.file); !ok {
				continue
			}
			if entry.FrontmatterError != "" {
				count.Unreadable++
				continue
			}
			if status, _ := entry.Frontmatter["status"].(string); strings.TrimSpace(status) != FeedbackStatusOffen {
				continue
			}
			betrifft, _ := entry.Frontmatter["betrifft"].(string)
			if betrifft = strings.TrimSpace(betrifft); betrifft == "" {
				betrifft = "unbekannt"
			}
			count.Open++
			count.ByBetrifft[betrifft]++
		}
		counts = append(counts, count)
	}
	sort.SliceStable(counts, func(i, j int) bool { return counts[i].Address < counts[j].Address })
	return counts
}

// CountFeedbackElsewhere zählt die offenen Meldungen in allen lesbaren
// Collections außer own und der öffentlichen k-playbook. Scheitert schon die
// Auskunft über die Hubs, ist das der Fehler; sonst steht jeder Fehler an
// seiner Collection.
func CountFeedbackElsewhere(ctx context.Context, own string) ([]FeedbackCollectionCount, error) {
	sources, err := FeedbackSources(ctx, own)
	if err != nil {
		return nil, err
	}
	return countFeedback(ctx, sources), nil
}
