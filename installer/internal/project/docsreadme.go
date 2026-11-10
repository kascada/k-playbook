package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Die README.md der Verzeichnisse unter keph:/docs/ kommen aus Vorlagen
// (Task 137): Vorlage für keph:/docs/<pfad>/README.md ist
// k-playbook:/docs/<pfad>/README.md, die Datei defaults/docs/<pfad>/README.md
// im Quell-Repo, mit dem Release auf dem Hub unter <hub>:k-playbook/defaults/.
// Die Liste der Vorlagen ist der Baum unter docs/ der mitgelieferten Ebene —
// eine zweite Liste im Code gibt es nicht. Eine lokale Fassung gibt es nicht
// (docDistOnlyKinds). docs/README.md ganz oben ist Kephalaions und hat keine
// Vorlage; ein Verzeichnis ohne Vorlage fasst k-playbook nicht an, dort mahnt
// Kephalaion selbst.

// docsReadmeTemplateName ist der Dateiname einer Vorlage und der README, die
// aus ihr entsteht.
const docsReadmeTemplateName = "README.md"

// DocsTemplateRef ist der Verweis auf die Vorlage eines Verzeichnisses unter
// docs/ — dir relativ zu docs/, etwa generated/code.
func DocsTemplateRef(dir string) string {
	return DocSchemeKPlaybook + ":/" + docDocsRoot + "/" + dir + "/" + docsReadmeTemplateName
}

// docsReadmeDocName ist der Name der README in der Collection zu einem
// Verzeichnis unter docs/ — dir relativ zu docs/.
func docsReadmeDocName(dir string) string {
	return docDocsRoot + "/" + dir + "/" + docsReadmeTemplateName
}

// ListDocsTemplates nennt die Verzeichnisse unter docs/, für die die
// mitgelieferte Ebene dist eine Vorlage trägt, relativ zu docs/ und sortiert,
// etwa generated, generated/code, manual. Eine README.md unmittelbar unter
// docs/ zählt nicht: docs/README.md ist Kephalaions. Andere Dateien als
// README.md sind keine Vorlagen und werden übergangen. Fehlt docs/ in der
// mitgelieferten Ebene, ist die Liste leer, kein Fehler — der Hub trägt dann
// einen Lieferumfang ohne Vorlagen (Versatz Binary/Hub, Task 133). Ein Fehler
// der Ebene selbst, etwa kephalaion_unavailable, kommt zurück.
func ListDocsTemplates(dist DocSource) ([]string, error) {
	if dist == nil {
		return []string{}, nil
	}
	dirs := []string{}
	var walk func(rel string) error
	walk = func(rel string) error {
		listed := docDocsRoot
		if rel != "" {
			listed += "/" + rel
		}
		entries, err := dist.List(listed)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s:/%s/ auflisten: %w", DocSchemeKPlaybook, listed, err)
		}
		for _, entry := range entries {
			child := entry.Name
			if rel != "" {
				child = rel + "/" + entry.Name
			}
			switch {
			case entry.Dir:
				if err := walk(child); err != nil {
					return err
				}
			case entry.Name == docsReadmeTemplateName && rel != "":
				dirs = append(dirs, rel)
			}
		}
		return nil
	}
	if err := walk(""); err != nil {
		return nil, err
	}
	sort.Strings(dirs)
	return dirs, nil
}

// CleanDocsDir macht aus dem Argument von `docs readme` das Verzeichnis
// relativ zu docs/: ein Pfad relativ zu docs/ (generated/code, auch mit „/“
// am Ende) oder ein Verweis keph:/docs/… bzw. k-playbook:/docs/… auf das
// Verzeichnis oder seine README.md. Geprüft wird mit derselben Formregel wie
// ein Verweis. docs/ selbst ist ungültig: docs/README.md ist Kephalaions und
// hat keine Vorlage.
func CleanDocsDir(arg string) (string, error) {
	raw := strings.TrimSpace(arg)
	full := DocSchemeKPlaybook + ":/" + docDocsRoot + "/" + raw
	if scheme, rest, found := strings.Cut(raw, ":"); found && !strings.Contains(scheme, "/") {
		if scheme != DocSchemeKeph && scheme != DocSchemeKPlaybook {
			return "", docRefErrorf(DocCodeInvalidInput,
				"%q: erwartet ein Pfad unter docs/ oder ein Verweis keph:/docs/… bzw. k-playbook:/docs/…", arg)
		}
		full = DocSchemeKPlaybook + ":" + rest
	}
	ref, err := ParseDocRef(full)
	if err != nil {
		return "", err
	}
	rel, ok := strings.CutPrefix(ref.Path, docDocsRoot)
	if ok && !ref.Dir {
		rel = strings.TrimSuffix(rel, "/"+docsReadmeTemplateName)
	}
	if !ok || !strings.HasPrefix(rel, "/") {
		return "", docRefErrorf(DocCodeInvalidInput,
			"%q ist kein Verzeichnis unter docs/ — erwartet etwa generated/code oder keph:/docs/generated/code/; "+
				"docs/README.md ganz oben ist Kephalaions und hat keine Vorlage", arg)
	}
	return rel[1:], nil
}

// DocCodeTemplateMissing: die Vorlage k-playbook:/docs/<pfad>/README.md gibt
// es in der mitgelieferten Ebene nicht — etwa weil das Binary neuer ist als
// der Lieferumfang auf dem Hub (Task 133). Nichts wurde geschrieben, auch
// keine leere README.
const DocCodeTemplateMissing = "template_missing"

// Die Zustände einer README über die von publish hinaus (DocsCreated,
// DocsWritten, DocsUnchanged, DocsFailed): nur mit Check, wo nichts
// geschrieben wird.
const (
	// DocsReadmeMissing: die README fehlt in der Collection.
	DocsReadmeMissing = "missing"
	// DocsReadmeDiffers: die README weicht von der Vorlage ab — von Hand
	// geändert, älter als die Vorlage oder Kephalaions Mahnung (generated).
	DocsReadmeDiffers = "differs"
)

// DocsReadmeStore sind die Aufrufe an Kephalaion, die das Schreiben einer
// README braucht. *kephalaion.Client erfüllt sie, ebenso DocsStore und der
// Zugang des Inventars; Tests setzen kephtest.Store ein. Read liest die
// Replica des Nodes, Create und Write gehen über den Hub.
type DocsReadmeStore interface {
	Read(ctx context.Context, name string) (kephalaion.ReadResult, error)
	Create(ctx context.Context, name, content string) (kephalaion.Document, error)
	Write(ctx context.Context, name, content string, baseRevision int64) (kephalaion.Document, error)
}

// DocsReadmeItem ist eine README im Ergebnis: das Verzeichnis relativ zu
// docs/, die README als keph:-Verweis, die Vorlage als k-playbook:-Verweis,
// der Zustand und die Revision danach (bei Check die aktuelle); Base ist die
// Revision vor dem Schreiben. Code und Message bei failed.
type DocsReadmeItem struct {
	Dir      string `json:"dir"`
	Ref      string `json:"ref"`
	Template string `json:"template"`
	Status   string `json:"status"`
	Revision int64  `json:"revision,omitempty"`
	Base     int64  `json:"base,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
}

// Failed meldet eine README, die nicht geschrieben (oder bei Check nicht
// geprüft) werden konnte.
func (i DocsReadmeItem) Failed() bool { return i.Status == DocsFailed }

// DocsReadmeResult ist das Ergebnis über alle oder eine Vorlage. Hint nennt,
// warum nichts zu tun war — etwa keine Vorlagen auf dem Hub.
type DocsReadmeResult struct {
	Check   bool             `json:"check,omitempty"`
	Readmes []DocsReadmeItem `json:"readmes"`
	Hint    string           `json:"hint,omitempty"`
}

// Failed zählt die READMEs mit failed.
func (r DocsReadmeResult) Failed() int {
	n := 0
	for _, item := range r.Readmes {
		if item.Failed() {
			n++
		}
	}
	return n
}

// DocsReadmeOptions sind die Schalter. Check schreibt nichts und meldet je
// Vorlage missing, differs oder unchanged.
type DocsReadmeOptions struct {
	Check bool
}

// DocsReadmes schreibt die README.md unter keph:/docs/ aus den Vorlagen —
// die eine Stelle dafür, gerufen von docs readme, publish, dem Inventar und
// dem Einrichten.
type DocsReadmes struct {
	store     DocsReadmeStore
	templates DocSource
}

// NewDocsReadmes baut den Zugang: store ist die Projekt-Collection,
// templates die mitgelieferte Ebene hinter k-playbook: (nur docs/ wird
// gelesen).
func NewDocsReadmes(store DocsReadmeStore, templates DocSource) *DocsReadmes {
	return &DocsReadmes{store: store, templates: templates}
}

// OpenDocsReadmes prüft die Anbindung des Projekts und baut den Zugang —
// derselbe Weg wie OpenDocsWork, Fehler mit DocCodeKephalaionUnavailable;
// die Vorlagen kommen aus <hub>:k-playbook/defaults/docs/
// (NewProjectDefaultsSource). Austauschbar, damit Subkommando und
// MCP-Tests den Node nicht fragen.
var OpenDocsReadmes = func(projectDir string) (*DocsReadmes, error) {
	client, _, err := openKephClient(projectDir)
	if err != nil {
		return nil, err
	}
	return NewDocsReadmes(client, NewProjectDefaultsSource(projectDir)), nil
}

// All schreibt die README jedes Verzeichnisses mit Vorlage
// (ListDocsTemplates), die fehlt oder inhaltlich abweicht. Lassen sich die
// Vorlagen nicht auflisten, ist das der Fehler; scheitert eine einzelne
// README, steht das in ihrem Eintrag, die übrigen laufen weiter.
func (r *DocsReadmes) All(ctx context.Context, opts DocsReadmeOptions) (DocsReadmeResult, error) {
	result := DocsReadmeResult{Check: opts.Check, Readmes: []DocsReadmeItem{}}
	dirs, err := ListDocsTemplates(r.templates)
	if err != nil {
		return result, err
	}
	if len(dirs) == 0 {
		result.Hint = "keine Vorlagen unter " + DocSchemeKPlaybook + ":/" + docDocsRoot + "/ — der Lieferumfang auf dem Hub " +
			"ist älter als dieses Programm; nach dem nächsten fetch_release des Hubs noch einmal"
		return result, nil
	}
	for _, dir := range dirs {
		result.Readmes = append(result.Readmes, r.One(ctx, dir, opts))
	}
	return result, nil
}

// One schreibt die README eines Verzeichnisses (dir relativ zu docs/, schon
// geprüft mit CleanDocsDir), wenn sie fehlt oder inhaltlich abweicht:
// create, sonst write mit der gelesenen Revision, sonst nichts. Fehlt die
// Vorlage oder ist sie unbrauchbar, wird nichts geschrieben, auch keine
// leere README.
func (r *DocsReadmes) One(ctx context.Context, dir string, opts DocsReadmeOptions) DocsReadmeItem {
	name := docsReadmeDocName(dir)
	item := DocsReadmeItem{Dir: dir, Ref: DocSchemeKeph + ":/" + name, Template: DocsTemplateRef(dir)}
	fail := func(code string, err error) DocsReadmeItem {
		item.Status, item.Code, item.Message = DocsFailed, code, err.Error()
		return item
	}
	want, err := r.template(ctx, dir)
	if err != nil {
		return fail(DocRefErrorCode(err), err)
	}

	read, err := r.store.Read(ctx, name)
	if err != nil {
		mapped := docsReadmeError(err)
		return fail(DocRefErrorCode(mapped), mapped)
	}
	var doc kephalaion.Document
	switch read.Kind {
	case kephalaion.ReadKindNone:
		if opts.Check {
			item.Status = DocsReadmeMissing
			return item
		}
		item.Status = DocsCreated
		doc, err = r.store.Create(ctx, name, want)
	case kephalaion.ReadKindDocument:
		item.Base, item.Revision = read.Revision, read.Revision
		if read.Content != nil && *read.Content == want {
			item.Status = DocsUnchanged
			return item
		}
		if opts.Check {
			item.Status = DocsReadmeDiffers
			return item
		}
		item.Status = DocsWritten
		doc, err = r.store.Write(ctx, name, want, read.Revision)
	default:
		return fail(DocCodeInvalidInput, fmt.Errorf("%s ist in der Collection ein Verzeichnis, keine README", item.Ref))
	}
	if err != nil {
		mapped := docsReadmeError(err)
		return fail(DocRefErrorCode(mapped), mapped)
	}
	item.Revision = doc.Revision
	return item
}

// template liest die Vorlage eines Verzeichnisses über dieselbe Auflösung
// wie k-playbook read (nur mitgeliefert, Includes eingesetzt) und prüft sie:
// Frontmatter mit title und description, kein generated — eine Vorlage mit
// generated hielte Kephalaion für seine Mahnung. Fehlt sie, ist das
// template_missing; ist sie unbrauchbar, invalid_input.
func (r *DocsReadmes) template(ctx context.Context, dir string) (string, error) {
	ref := DocsTemplateRef(dir)
	resolved, err := ResolveDocRefWith(ctx, DocLayers{Dist: r.templates}, ref, DocReadOptions{})
	if DocRefErrorCode(err) == DocCodeNotFound {
		return "", docRefErrorf(DocCodeTemplateMissing,
			"die Vorlage %s fehlt in der mitgelieferten Ebene — der Lieferumfang auf dem Hub ist älter als dieses "+
				"Programm oder kennt das Verzeichnis nicht; nichts geschrieben", ref)
	}
	if err != nil {
		return "", err
	}
	content := resolved.Content
	if strings.TrimSpace(content) == "" {
		return "", docRefErrorf(DocCodeInvalidInput, "die Vorlage %s ist leer; nichts geschrieben", ref)
	}
	if _, problems := parseDocsMeta([]byte(content)); len(problems) > 0 {
		return "", docRefErrorf(DocCodeInvalidInput, "die Vorlage %s ist unbrauchbar: %s; nichts geschrieben",
			ref, strings.Join(problems, "; "))
	}
	if docsHasGenerated([]byte(content)) {
		return "", docRefErrorf(DocCodeInvalidInput,
			"die Vorlage %s trägt generated — daran erkennt Kephalaion seine Mahnung; nichts geschrieben", ref)
	}
	return content, nil
}

// docsHasGenerated meldet, ob das Frontmatter eines Dokuments generated trägt.
func docsHasGenerated(content []byte) bool {
	block, ok := inventory.FrontmatterBlock(content)
	if !ok {
		return false
	}
	root, err := yamllite.Parse([]byte(block))
	return err == nil && root.Get("generated") != nil
}

// docsReadmeError übersetzt einen Fehler des Stores in einen Code der
// doc-Werkzeuge, wenn er noch keinen trägt — der Zugang des Inventars
// übersetzt schon selbst.
func docsReadmeError(err error) error {
	if DocRefErrorCode(err) != "" {
		return err
	}
	return kephError(err)
}

// docsReadmeSetupTimeout begrenzt den README-Schritt des Einrichtens
// (ApplyAssistantSetup) als Ganzes: Prüfen der Anbindung, Öffnen der
// mitgelieferten Ebene, Lesen der Vorlagen und je README Lesen und Schreiben.
// Am lokalen Node dauert das weit unter einer Sekunde; die Grenze greift nur,
// wenn Node oder Hub nicht antworten. Sie liegt zwischen der Grenze einer
// Prüfung der Anbindung (kephalaion.DefaultTimeout, 3 s) und der eines
// einzelnen Werkzeugaufrufs (kephalaion.ToolTimeout, 15 s) und entspricht
// den git-Aufrufen des Einrichtens (privateGitTimeout, checkIgnoreTimeout).
// Austauschbar für Tests.
var docsReadmeSetupTimeout = 10 * time.Second

// SetupReadmes ist das README-Ergebnis des Einrichtens: je Vorlage der
// Eintrag aus DocsReadmes.All, oder ein Hinweis, warum nichts versucht oder
// nicht fertig wurde. Es ist kein Fehler des Einrichtens.
type SetupReadmes struct {
	Readmes []DocsReadmeItem `json:"readmes,omitempty"`
	Hint    string           `json:"hint,omitempty"`
}

// setupDocsReadmes schreibt die README beim Einrichten; austauschbar, damit
// Tests des Einrichtens den Schritt ersetzen können.
var setupDocsReadmes = writeSetupReadmes

// openSetupDocsReadmes ist OpenDocsReadmes für das Einrichten: Die
// mitgelieferte Ebene liest mit der Frist des Schritts. Austauschbar, damit
// Tests den Node nicht fragen.
var openSetupDocsReadmes = func(ctx context.Context, projectDir string) (*DocsReadmes, error) {
	client, _, err := openKephClient(projectDir)
	if err != nil {
		return nil, err
	}
	return NewDocsReadmes(client, newProjectDefaultsSourceContext(ctx, projectDir)), nil
}

// writeSetupReadmes schreibt beim Einrichten alle README aus den Vorlagen
// (Task 137, Etappe 5) — so stehen auch die von manual/ und sources/ da, die
// kein Erzeuger mitschreibt, bevor dort das erste Dokument entsteht. Ist
// Kephalaion nicht bereit, wird nichts versucht (OpenDocsReadmes prüft die
// Anbindung zuerst). Der ganze Schritt endet spätestens nach
// docsReadmeSetupTimeout: Ein langsamer oder unerreichbarer Hub hält das
// Einrichten nur so lange auf. Die Aufrufe am Node tragen dieselbe Frist und
// brechen danach ab; die Prüfung der Anbindung hat ihre eigene, kürzere.
func writeSetupReadmes(projectDir string) SetupReadmes {
	ctx, cancel := context.WithTimeout(context.Background(), docsReadmeSetupTimeout)
	defer cancel()
	done := make(chan SetupReadmes, 1)
	go func() {
		readmes, err := openSetupDocsReadmes(ctx, projectDir)
		if err != nil {
			done <- SetupReadmes{Hint: "nicht versucht, " + strings.TrimRight(err.Error(), ".")}
			return
		}
		result, err := readmes.All(ctx, DocsReadmeOptions{})
		if err != nil {
			done <- SetupReadmes{Hint: "nicht geschrieben, die Vorlagen sind nicht lesbar: " + strings.TrimRight(err.Error(), ".")}
			return
		}
		done <- SetupReadmes{Readmes: result.Readmes, Hint: result.Hint}
	}()
	select {
	case result := <-done:
		return result
	case <-ctx.Done():
		return SetupReadmes{Hint: fmt.Sprintf("nach %s abgebrochen, Kephalaion antwortet nicht rechtzeitig — "+
			"nachholen mit k-playbook docs readme", docsReadmeSetupTimeout)}
	}
}

// DescribeSetupReadmes fasst das README-Ergebnis des Einrichtens in einen
// Satz für die Ausgabe von upgrade und Oberfläche: wie viele angelegt,
// überschrieben und unverändert sind, und jede README, die scheiterte, mit
// Code. Leer, wenn es nichts zu sagen gibt.
func DescribeSetupReadmes(setup SetupReadmes) string {
	prefix := "README unter keph:/docs/: "
	if len(setup.Readmes) == 0 {
		if setup.Hint == "" {
			return ""
		}
		return prefix + setup.Hint + "."
	}
	counts := map[string]int{}
	failed := []string{}
	for _, item := range setup.Readmes {
		counts[item.Status]++
		if item.Failed() {
			failed = append(failed, item.Ref+" ("+item.Code+")")
		}
	}
	parts := []string{}
	for _, part := range []struct{ status, label string }{
		{DocsCreated, "angelegt"}, {DocsWritten, "aus der Vorlage überschrieben"}, {DocsUnchanged, "unverändert"},
	} {
		if counts[part.status] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[part.status], part.label))
		}
	}
	text := prefix + strings.Join(parts, ", ")
	if len(failed) > 0 {
		if len(parts) > 0 {
			text += "; "
		}
		text += "nicht geschrieben: " + strings.Join(failed, ", ")
	}
	return text + "."
}

// Changed meldet, ob der Schritt eine README angelegt oder geschrieben hat.
func (s SetupReadmes) Changed() bool {
	for _, item := range s.Readmes {
		if item.Status == DocsCreated || item.Status == DocsWritten {
			return true
		}
	}
	return false
}
