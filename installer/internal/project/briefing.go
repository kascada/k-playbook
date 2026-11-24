package project

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/health"
	"github.com/kascada/k-playbook/installer/internal/inventory"
	"github.com/kascada/k-playbook/installer/internal/kephalaion"
	"github.com/kascada/k-playbook/installer/internal/yamllite"
)

// Das Briefing (Task 086): ein Aufruf zu Beginn einer Sitzung, der als Daten
// auflistet, welche Anweisungen, Reviews und Leitlinien für dieses
// Projekt gelten und was in der Projekt-Collection für die eigene Arbeit
// ansteht. Verweise im Präfixschema, keine Pfade; gelesen wird über
// k_playbook_doc_read bzw. auf dem Weg des Präfixes. Inhalte stehen nicht
// darin — mit einer Ausnahme: den Text der immer geltenden Anweisungen
// (k-playbook:/instructions/always/) trägt das Briefing selbst
// (Documents.Always.Content), denn jede Sitzung muss ihn sofort lesen; ein
// zweiter Aufruf dafür wäre nur Umweg. Was an der Umgebung nicht stimmt,
// steht bewertet unter Health (Task 098).
//
// Seit Task 187 trägt das Briefing keinen Rest der Kontextausgabe mehr. Was
// jede Sitzung braucht, hat eigene Abschnitte: Now (Datum), Project (Wurzel,
// Repo, Sprachen), Kephalaion (ob die Collection erreichbar ist, Adresse,
// User), GitHub (die Dienste dieses Repos) und Links (was die Selbstheilung
// der Assistenten-Verlinkung geändert hat). Was nur einzelne Commands lesen —
// gh, die GitHub-Zugangsdatei, Basis-Werkzeuge, Quellenkonfiguration,
// docs.exclude, Remediation —, liefert der Teilabruf environment
// (BuildBriefingEnvironment), nicht das Briefing. Das Unterkommando und das
// MCP-Werkzeug, die die Kontextausgabe lieferten, sind mit Task 188 entfernt.
//
// Die Listen kommen aus den vorhandenen Funktionen: die Dokumente über
// dieselbe Überlagerung wie die Verweise und die Kataloge (overlayDocDir,
// assembleDocDir), die eigene Arbeit über UserDocs.List — die Funktion hinter
// k_playbook_doc_list —, die Abschnitte der Umgebung aus denselben Lesern wie
// die Kontextausgabe (buildNow, ReadConfig, KephalaionStatus, contextGitHub,
// HealLinks).

// BriefingSchemaVersion ist die Fassung der Briefing-Ausgabe. Fassung 2 hat den
// Abschnitt context, Fassung 3 den Text der immer geltenden Anweisungen in
// documents.always.content, Fassung 4 den Abschnitt health, Fassung 5 die
// Sitzungskennung session; in Fassung 6 entfällt documents.rules, die Sorte
// rules gibt es nicht mehr; in Fassung 7 entfällt context.playbook, den Clone
// k-playbook/ im Projekt gibt es nicht mehr; in Fassung 8 entfällt
// context.local, Projekteigenes liegt in der Collection und unter
// .k-playbook/. In Fassung 9 (Task 187) entfällt der Abschnitt context ganz:
// now, project, kephalaion, github und links sind eigene Abschnitte, die
// übrigen Umgebungsangaben liefert der Teilabruf environment; work.tasks und
// work.workbench sind auf je zehn Einträge gedeckelt, work.total nennt die
// Zahl aller.
const BriefingSchemaVersion = "9"

// BriefingPartEnvironment ist der Name des Teilabrufs environment — der Wert
// von --part und von part im MCP-Werkzeug.
const BriefingPartEnvironment = "environment"

// BriefingOriginDisabled ist die Herkunft eines Eintrags, den ein leerer
// lokaler Eintrag abgeschaltet hat. Im Briefing ersetzt sie die Herkunft, denn
// für die Sitzung zählt nur: diesen Eintrag gibt es nicht zu lesen.
const BriefingOriginDisabled = "disabled"

// briefingAlwaysDir ist das zusammengebaute Verzeichnis der immer geltenden
// Anweisungen, briefingOnDemandDir das der Anweisungen auf Abruf.
const (
	briefingAlwaysDir   = docInstructionsRoot + "/always"
	briefingOnDemandDir = docInstructionsRoot + "/on-demand"
)

// Briefing ist die Antwort von `k-playbook briefing` und k_playbook_briefing.
type Briefing struct {
	SchemaVersion string `json:"schemaVersion"`
	// Session ist die Kennung dieser Sitzung, je Briefing neu vergeben. Eine
	// Sitzung lädt das Briefing einmal; was sie schreibt und später
	// wiederfinden muss — ein Befund in queue/findings/ —, trägt diese
	// Kennung. Vergeben wird sie hier, weil nur Claude Code eine eigene
	// Kennung in die Umgebung legt, OpenCode keine (gemessen 2026-10-24).
	Session string `json:"session"`
	// Now ist der Zeitpunkt des Briefings: das Datum, das eine Sitzung in
	// Protokolle und Namen schreibt, statt es zu raten.
	Now ContextNow `json:"now"`
	// Project ist das Projekt: seine Wurzel, die Wurzel des Repos, das
	// Versionssystem und die Sprachen.
	Project BriefingProject `json:"project"`
	// Kephalaion sagt, ob die Collection des Projekts erreichbar ist, und
	// nennt ihre Adresse und den eigenen User.
	Kephalaion BriefingKephalaion `json:"kephalaion"`
	// Documents sind die Dokumente, die für dieses Projekt gelten — lokal und
	// mitgeliefert schon gegeneinander aufgelöst.
	Documents BriefingDocuments `json:"documents"`
	// Work ist, was in der Projekt-Collection für die eigene Arbeit ansteht.
	Work BriefingWork `json:"work"`
	// ProjectDocs ist der Einstieg der Projektdokumentation: der
	// Verzeichnis-Verweis keph:/docs/ in der Collection des Projekts, gelesen
	// mit Kephalaion list (recursive, frontmatter), solange Kephalaion dort
	// keine index.md erzeugt (Task 101). Kein Rückfall auf die Platte.
	ProjectDocs string `json:"projectDocs"`
	// GitHub nennt die Dienste der persönlichen Zugangsdatei, die zum Repo
	// des Arbeitsverzeichnisses gehören — jede Sitzung braucht den Dienst vor
	// dem ersten GitHub-Zugriff. Das volle Objekt liegt im Teilabruf
	// environment.
	GitHub BriefingGitHub `json:"github"`
	// Health ist die Umgebungsprüfung (Task 098): Fehler und Warnungen über
	// die Umgebung des Projekts, alle auf einmal, je Eintrag Stufe, Bereich,
	// Satz und Abhilfe — dieselbe Antwort wie `k-playbook health --json` und
	// k_playbook_health. Die Sitzung nennt die Einträge in ihrer ersten
	// Antwort; health bewertet und nennt Abhilfen, der Teilabruf environment
	// liefert die Rohwerte dazu.
	Health health.Report `json:"health"`
	// Links meldet die Selbstheilung der Assistenten-Verlinkung, die das
	// Briefing nebenbei anstößt (HealLinks): was nachgezogen wurde und was
	// offen blieb. Fehlt das Feld, stimmte alles — oder das Projekt ließ sich
	// nicht lesen (Project.Available false), dann wird nichts eingerichtet.
	Links *ContextLinks `json:"links,omitempty"`
}

// BriefingProject ist der Abschnitt project. Dir steht immer da: Das Projekt
// ist gefunden, sonst gäbe es kein Briefing. Lässt sich die Konfiguration
// nicht lesen — unlesbare Datei, unbekannte Fassung, unzulässiger
// Sprachname —, ist das ein Zustand wie bei BriefingWork: Available false mit
// Message, die übrigen Felder fehlen, das Briefing bleibt.
type BriefingProject struct {
	// Available sagt, ob die Konfiguration gelesen werden konnte.
	Available bool `json:"available"`
	// Message nennt, warum nicht; leer, wenn Available.
	Message string `json:"message,omitempty"`
	// Dir ist die Wurzel des Projekts, das Verzeichnis mit .k-playbook/.
	Dir string `json:"dir"`
	// RepoRoot ist die Wurzel des Repos, VCS das Versionssystem.
	RepoRoot string `json:"repoRoot,omitempty"`
	VCS      string `json:"vcs,omitempty"`
	// Languages sind die Sprachen, für die das Projekt Werkzeuge braucht —
	// aus der Konfiguration, sonst erkannt, sonst die Standardliste.
	Languages []string `json:"languages,omitempty"`
}

// BriefingKephalaion ist der Abschnitt kephalaion: von der Anbindung an die
// Collection des Projekts nur, was Texte lesen. Hub, Collection, Account,
// Rechte, Node und Stand des Abgleichs stehen nicht mehr im Briefing; wer sie
// braucht, fragt Kephalaion (whoami) oder liest die Oberfläche.
type BriefingKephalaion struct {
	// OK sagt, ob keph:-Verweise erreichbar sind.
	OK bool `json:"ok"`
	// Message ist der Satz zum Zustand, bei einem Fehler samt Abhilfe.
	Message string `json:"message"`
	// Address ist <hub>:<collection>, der Wert für collection in den
	// Kephalaion-Werkzeugen.
	Address string `json:"address"`
	// User ist der eigene Kephalaion-User — das, wofür ~ in einem
	// keph:-Verweis steht. Nur bei OK.
	User string `json:"user,omitempty"`
}

// BriefingGitHub ist der Abschnitt github des Briefings.
type BriefingGitHub struct {
	// ProjectServices sind die Dienste der Zugangsdatei, deren repo das Repo
	// des Arbeitsverzeichnisses ist, in Dateireihenfolge; leer ohne Datei,
	// ohne Remote oder ohne passenden Dienst.
	ProjectServices []string `json:"projectServices"`
	// Access ist das volle Objekt, aus dem der Klartext die Zeile „GitHub: …“
	// baut. Es steht nicht im JSON des Briefings — dort liegt es im
	// Teilabruf environment unter github.
	Access *ContextGitHub `json:"-"`
}

// BriefingEnvironment ist der Teilabruf environment (Task 187): die
// Umgebungsangaben, die nicht jede Sitzung braucht, sondern einzelne Commands
// an der Stelle, an der sie sie brauchen — unter denselben Namen und mit
// denselben Unterfeldern wie in der Kontextausgabe, ohne die Felder, die kein
// Text liest (von gh bleiben ready und account, baseTools verliert matrix).
// Abgrenzung zu Health: health bewertet und nennt Abhilfen, environment
// liefert die Rohwerte.
//
// Der Teilabruf ist kein Briefing: Er vergibt keine Sitzungskennung, liefert
// weder Anweisungen noch Arbeitsliste, zieht keine Verlinkung nach und gilt
// nicht als Neuladen. Er wird bei Bedarf frisch geholt, auch mehrmals je
// Sitzung.
//
// Lässt sich die Konfiguration nicht lesen, ist das ein Zustand: Available
// false mit Message, die Felder fehlen.
type BriefingEnvironment struct {
	// Available sagt, ob der Teil gebaut werden konnte.
	Available bool `json:"available"`
	// Message nennt, warum nicht; leer, wenn Available.
	Message string `json:"message,omitempty"`
	// GH ist der Befund zur GitHub CLI: ob sie da und angemeldet ist, und der
	// aktive Account.
	GH *BriefingGH `json:"gh,omitempty"`
	// GitHub ist die persönliche Zugangsdatei gegen das Repo des
	// Arbeitsverzeichnisses, wie `k-playbook github show --json`.
	GitHub *ContextGitHub `json:"github,omitempty"`
	// BaseTools ist der Host-Befund zu den Basis-Werkzeugen.
	BaseTools *BriefingBaseTools `json:"baseTools,omitempty"`
	// VersionSources ist der Zustand der Quellenkonfiguration des
	// Versionsinventars.
	VersionSources *VersionSources `json:"versionSources,omitempty"`
	// Docs ist der Block docs: — was die Docs-Erzeuger zusätzlich auslassen.
	Docs *ContextDocs `json:"docs,omitempty"`
	// Remediation ist die Remediation-Policy des Projekts.
	Remediation *Remediation `json:"remediation,omitempty"`
}

// BriefingGH ist gh im Teilabruf environment: was Commands lesen.
type BriefingGH struct {
	// Ready: gh ist da und angemeldet.
	Ready bool `json:"ready"`
	// Account ist der aktive Account; leer bei Anmeldung über die Umgebung.
	Account string `json:"account"`
}

// BriefingBaseTools ist baseTools im Teilabruf environment: BaseTools ohne
// den Ort der Matrix, den kein Text liest.
type BriefingBaseTools struct {
	Present        bool       `json:"present"`
	Error          string     `json:"error,omitempty"`
	Missing        []BaseTool `json:"missing"`
	OK             bool       `json:"ok"`
	InstallCommand string     `json:"installCommand"`
}

// BriefingDocuments sind die Dokumentlisten je Sorte, in Lesereihenfolge.
type BriefingDocuments struct {
	// Always ist das zusammengebaute Verzeichnis instructions/always/ mit
	// seinen Teilen.
	Always BriefingAssembled `json:"always"`
	// OnDemand sind die Anweisungen auf Abruf, Reviews und Guidelines die
	// Einträge der Sorten — jede als Datei einzeln zu lesen.
	OnDemand   []BriefingEntry `json:"onDemand"`
	Reviews    []BriefingEntry `json:"reviews"`
	Guidelines []BriefingEntry `json:"guidelines"`
}

// BriefingAssembled ist ein zusammengebautes Verzeichnis: der Verweis, der es
// als einen Text liefert, der Text selbst und seine Teile in der Reihenfolge
// des Texts.
type BriefingAssembled struct {
	Ref string `json:"ref"`
	// Content ist der zusammengebaute Text — derselbe, den `k-playbook read`
	// und k_playbook_doc_read für Ref liefern (joinDocDirParts): die Teile
	// ohne Frontmatter, durch eine Leerzeile getrennt, abgeschaltete
	// übergangen. Leer ohne aufgenommene Teile.
	Content string          `json:"content"`
	Parts   []BriefingEntry `json:"parts"`
	// Problems nennt, was übergangen wurde — eine Datei ohne Ziffer vorn.
	Problems []string `json:"problems"`
}

// BriefingEntry ist ein Dokument im Briefing.
type BriefingEntry struct {
	// Ref ist der Verweis, über den das Dokument gelesen wird.
	Ref string `json:"ref"`
	// Title ist der Titel: Frontmatter title, sonst die erste Überschrift,
	// sonst der Dateiname.
	Title string `json:"title"`
	// Description ist die Beschreibung aus dem Frontmatter, sonst leer.
	Description string `json:"description"`
	// Origin ist die Herkunft: dist, local, override oder disabled.
	Origin string `json:"origin"`
	// Location ist bei local, override und disabled der Ort des
	// projekteigenen Teils als keph:-Verweis (keph:/k-playbook/<pfad>), wenn
	// er in der Collection liegt; sonst leer.
	Location string `json:"location,omitempty"`
}

// BriefingWork ist die eigene Arbeit aus der Projekt-Collection. Ist Kephalaion
// nicht nutzbar, steht das in Available und Message, die Listen bleiben leer.
//
// Die Listen sind gedeckelt (Task 187): je höchstens BriefingWorkLimit
// Einträge, damit das Briefing auch bei viel offener Arbeit in einem Stück
// ankommt. Wie viele es insgesamt sind, steht unter Total; den Rest liefert
// k_playbook_doc_list (kind tasks bzw. workbench).
type BriefingWork struct {
	// Available sagt, ob die Collection gelesen werden konnte.
	Available bool `json:"available"`
	// Message nennt, warum nicht; leer, wenn Available.
	Message string `json:"message,omitempty"`
	// User ist der eigene Kephalaion-User, dessen Arbeit hier steht.
	User string `json:"user,omitempty"`
	// Tasks sind die eigenen offenen Tasks, nach Nummer; Workbench die eigenen
	// aktiven Arbeitsstände, nach letzter Änderung, die neueste zuerst — offen
	// und aktiv heißt: nicht in done/. Je höchstens BriefingWorkLimit.
	Tasks     []BriefingDoc `json:"tasks"`
	Workbench []BriefingDoc `json:"workbench"`
	// Total nennt je Liste die Zahl aller Dokumente, auch der nicht gezeigten.
	Total BriefingWorkTotal `json:"total"`
	// OpenFindings ist die Zahl der offenen Befunde in queue/findings/.
	OpenFindings int `json:"openFindings"`
}

// BriefingWorkLimit ist der Deckel je Liste der eigenen Arbeit.
const BriefingWorkLimit = 10

// BriefingWorkTotal ist die Zahl aller eigenen offenen Tasks und aktiven
// Arbeitsstände — auch derer, die der Deckel aus den Listen hält.
type BriefingWorkTotal struct {
	Tasks     int `json:"tasks"`
	Workbench int `json:"workbench"`
}

// BriefingWorkDirRef ist der Verzeichnis-Verweis der Ablage einer Sorte mit
// Userbereich für einen User, etwa keph:/user/kamran/tasks/ — die Kopfzeile
// der Liste im Klartext nennt ihn, die Einträge darunter nur den Dateinamen.
// Leer für eine unbekannte Sorte.
func BriefingWorkDirRef(kind string, user string) string {
	for _, item := range DocKinds {
		if item.Kind == kind {
			return DocSchemeKeph + ":/" + item.dirFor(user) + "/"
		}
	}
	return ""
}

// BriefingDoc ist ein Dokument der Collection im Briefing.
type BriefingDoc struct {
	Ref     string `json:"ref"`
	Title   string `json:"title"`
	Updated string `json:"updated,omitempty"`
	// Problem ist DocListItem.Problem: unlesbares Frontmatter oder ein
	// vermutlich abgeschnittener Titel; leer, wenn nichts auffiel.
	Problem string `json:"problem,omitempty"`
}

// OpenBriefingDocs ist der Zugang zur Collection; austauschbar, damit Tests
// den Node nicht fragen — exportiert wie OpenDefaultsStore, weil auch der
// Klartext des Unterkommandos gegen eine Collection im Speicher geprüft wird.
var OpenBriefingDocs = OpenUserDocs

// BuildBriefing stellt das Briefing eines Projekts zusammen. Ein Fehler ist
// nur die Umgebung — eine Ebene, die sich nicht lesen lässt; eine nicht
// nutzbare Collection ist ein Zustand in Work, eine nicht lesbare
// Konfiguration einer in Project, beides kein Fehler. Die mitgelieferte
// Ebene liegt in <hub>:k-playbook unter defaults/: Ist sie nicht lesbar,
// scheitert das Briefing als Ganzes mit DocCodeKephalaionUnavailable — die
// lokale Ebene steht nie allein da.
func BuildBriefing(ctx context.Context, projectDir string) (Briefing, error) {
	return BuildBriefingFrom(ctx, projectDir, projectDir)
}

// BuildBriefingFrom ist BuildBriefing mit dem Verzeichnis, aus dem der Aufruf
// kommt (startDir; projectDir ist das daraus gefundene Projekt): github liest
// das Remote des Repos, in dem startDir liegt — ein eigenes Repo unterhalb des
// Projekts bekommt sein eigenes.
//
// Nebenwirkung: Das Briefing zieht die Assistenten-Verlinkung nach (HealLinks)
// und meldet unter Links, was sich geändert hat. Das gehört hierher, weil das
// Briefing am Anfang jeder Sitzung steht und die Verlinkung dem Katalog dieses
// Projekts folgt, nicht dem Weg, auf dem das Programm zu seinem Stand kam.
// Bis Task 187 kam die Heilung nur nebenbei über die Kontextausgabe des
// inzwischen entfernten Unterkommandos mit.
func BuildBriefingFrom(ctx context.Context, projectDir string, startDir string) (Briefing, error) {
	layers := ProjectDocLayers(projectDir)
	briefing := Briefing{
		SchemaVersion: BriefingSchemaVersion,
		Session:       newSessionID(),
		Now:           buildNow(),
		ProjectDocs:   DocSchemeKeph + ":/docs/",
	}

	always, err := briefingAssembled(ctx, layers, briefingAlwaysDir)
	if err != nil {
		return Briefing{}, briefingError(err)
	}
	briefing.Documents.Always = always

	markdown := catalogKind{name: "on-demand", dirName: briefingOnDemandDir, suffix: ".md"}
	for _, item := range []struct {
		kind   catalogKind
		target *[]BriefingEntry
	}{
		{markdown, &briefing.Documents.OnDemand},
		{catalogKindNamed("reviews"), &briefing.Documents.Reviews},
		{guidelinesKind, &briefing.Documents.Guidelines},
	} {
		entries, err := briefingEntries(layers, item.kind)
		if err != nil {
			return Briefing{}, briefingError(err)
		}
		*item.target = entries
	}

	briefing.Work = briefingWork(ctx, projectDir)
	briefing.Health = health.Check(ctx, projectDir)
	briefing.Project = briefingProject(projectDir)
	briefing.Kephalaion = briefingKephalaion(KephalaionStatus(projectDir))
	briefing.GitHub = briefingGitHub(contextGitHub(startDir))
	// Erst lesen, dann heilen: Bricht das Lesen an
	// einer unlesbaren oder zu neuen Konfiguration ab, wird auch nichts
	// eingerichtet. Ein Werkzeug, das die Fassung nicht versteht, soll die
	// Registrierung nicht nach seinen Regeln umschreiben.
	if briefing.Project.Available {
		briefing.Links = contextLinks(HealLinks(projectDir))
	}
	return briefing, nil
}

// briefingProject baut den Abschnitt project aus der Konfiguration. Jeder
// Fehler ist ein Zustand; Dir steht trotzdem da.
func briefingProject(projectDir string) BriefingProject {
	section := BriefingProject{Dir: projectDir}
	config, err := readBriefingConfig(projectDir)
	if err != nil {
		section.Message = err.Error()
		return section
	}
	// Ein unzulässiger Sprachname ist ein Zustand, kein stilles Verschwinden:
	// Die Werkzeugauswahl verschöbe sich sonst unbemerkt.
	languages, _, err := ReadLanguages(projectDir)
	if err != nil {
		section.Message = err.Error()
		return section
	}
	section.Available = true
	section.RepoRoot = RepoRootDir(projectDir, config)
	section.VCS = config.VCS
	section.Languages = languages
	return section
}

// readBriefingConfig liest die Konfiguration und prüft ihre Fassung. Bei
// unbekannter Fassung wird nichts geraten: Die Werte ließen sich lesen,
// bedeuteten aber etwas anderes.
func readBriefingConfig(projectDir string) (Config, error) {
	config, err := ReadConfig(projectDir)
	if err != nil {
		return Config{}, err
	}
	if err := CheckSchema(config); err != nil {
		return Config{}, err
	}
	return config, nil
}

// briefingKephalaion nimmt aus dem Zustand der Anbindung, was Texte lesen.
func briefingKephalaion(status kephalaion.Status) BriefingKephalaion {
	return BriefingKephalaion{OK: status.OK, Message: status.Message, Address: status.Address, User: status.User}
}

// briefingGitHub macht aus dem vollen Objekt den Abschnitt github.
func briefingGitHub(access *ContextGitHub) BriefingGitHub {
	section := BriefingGitHub{ProjectServices: []string{}, Access: access}
	if access != nil {
		section.ProjectServices = append(section.ProjectServices, access.ProjectServices...)
	}
	return section
}

// BuildBriefingEnvironment baut den Teilabruf environment für ein Projekt.
// startDir ist das Verzeichnis, aus dem der Aufruf kommt: github liest das
// Remote des Repos, in dem es liegt. Geschrieben wird nichts — keine
// Verlinkung, keine Zugangsdatei. Jeder Fehler ist ein Zustand im Ergebnis.
func BuildBriefingEnvironment(projectDir string, startDir string) BriefingEnvironment {
	config, err := readBriefingConfig(projectDir)
	if err != nil {
		return BriefingEnvironment{Message: err.Error()}
	}
	remediation, err := ReadRemediation(projectDir)
	if err != nil {
		return BriefingEnvironment{Message: err.Error()}
	}
	gh := DetectGH()
	tools := DetectBaseTools(projectDir)
	docs := contextDocs(projectDir, config)
	return BriefingEnvironment{
		Available: true,
		GH:        &BriefingGH{Ready: gh.Ready, Account: gh.Account},
		GitHub:    contextGitHub(startDir),
		BaseTools: &BriefingBaseTools{Present: tools.Present, Error: tools.Error, Missing: tools.Missing, OK: tools.OK,
			InstallCommand: tools.InstallCommand},
		VersionSources: versionSourcesWithStatus(projectDir, KephalaionStatus(projectDir)),
		Docs:           &docs,
		Remediation:    &remediation,
	}
}

// briefingError macht aus dem Scheitern an einer nicht erreichbaren
// Collection eine Meldung, die sich ohne Vorwissen lesen lässt: dass
// Kephalaion nicht erreichbar ist, der Grund aus der Meldung von Kephalaion,
// die Abhilfe wie in der Gesundheitsprüfung (kephalaionHint) und als
// nächster Schritt TroubleshootingCommand. Der Code bleibt
// DocCodeKephalaionUnavailable, der ursprüngliche Fehler bleibt umhüllt;
// jeder andere Fehler bleibt, wie er ist. Ohne die mitgelieferten Texte gibt
// es kein Briefing — es scheitert weiter als Ganzes (Task 131), nur
// verständlich.
func briefingError(err error) error {
	if DocRefErrorCode(err) != DocCodeKephalaionUnavailable {
		return err
	}
	reason, remedy := err.Error(), ""
	var notReady *KephalaionNotReadyError
	if errors.As(err, &notReady) {
		sentence, hint := notReady.Status.SplitRemedy()
		if sentence != "" {
			reason = sentence
		}
		if hint == "" {
			hint = kephalaionHint(notReady.Status.State)
		}
		remedy = hint
	}
	return &DocRefError{Code: DocCodeKephalaionUnavailable, err: &briefingUnavailableError{reason: reason, remedy: remedy, cause: err}}
}

// briefingUnavailableError ist der Fehler des Briefings ohne erreichbares
// Kephalaion.
type briefingUnavailableError struct {
	reason string
	// remedy ist die Abhilfe aus dem Zustand der Anbindung, leer, wenn der
	// Fehler keinen Zustand trägt (dann steht sie im Grund).
	remedy string
	cause  error
}

func (e *briefingUnavailableError) Error() string {
	text := "Kephalaion ist nicht erreichbar — ohne Kephalaion gibt es kein Briefing.\n" +
		"Grund: " + e.reason + "\n"
	if e.remedy != "" {
		text += "Abhilfe: " + e.remedy + "\n"
	}
	return text + "Nächster Schritt: " + TroubleshootingCommand
}

func (e *briefingUnavailableError) Unwrap() error { return e.cause }

// newSessionID vergibt die Sitzungskennung: Zeitpunkt des Briefings, damit
// sich Kennungen ordnen lassen, und sechs Zeichen Zufall, damit zwei
// Sitzungen in derselben Sekunde verschieden bleiben. Ausgelagert, damit Tests
// eine feste Kennung setzen können.
var newSessionID = func() string {
	random := make([]byte, 3)
	// Seit Go 1.24 kehrt crypto/rand.Read nie mit einem Fehler zurück.
	_, _ = rand.Read(random)
	return now().Format("20060102-150405") + "-" + hex.EncodeToString(random)
}

// briefingAssembled liefert ein zusammengebautes Verzeichnis: seine Teile
// mit Herkunft und den Text daraus, Includes eingesetzt wie bei
// `k-playbook read`; ein Teil, dessen Include scheitert, steht mit Kette
// unter problems und fehlt — das Briefing kommt trotzdem zustande.
// Gibt es das Verzeichnis in keiner Ebene, bleibt die Liste leer: der Verweis
// steht trotzdem da, damit die Sitzung weiß, worüber sie es lesen würde.
func briefingAssembled(ctx context.Context, layers DocLayers, dir string) (BriefingAssembled, error) {
	assembled := BriefingAssembled{Ref: DocSchemeKPlaybook + ":/" + dir + "/", Parts: []BriefingEntry{}, Problems: []string{}}
	parts, _, problems, _, err := assembleDocDir(ctx, layers, dir, DocReadOptions{})
	if err != nil {
		return BriefingAssembled{}, err
	}
	for _, part := range parts {
		assembled.Parts = append(assembled.Parts, briefingEntry(dir, docDirEntry{
			Name: part.Name, Origin: part.Origin, Location: part.Location, Disabled: part.Disabled, Content: part.content,
		}))
	}
	assembled.Content = joinDocDirParts(parts)
	assembled.Problems = append(assembled.Problems, problems...)
	return assembled, nil
}

// briefingEntries listet die Einträge einer Sorte, nach Namen sortiert, mit
// dem Muster der Sorte gefiltert wie im Katalog.
func briefingEntries(layers DocLayers, kind catalogKind) ([]BriefingEntry, error) {
	resolved, _, err := overlayDocDir(layers, kind.dirName, func(name string) bool { return isCatalogEntry(name, kind) })
	if err != nil {
		return nil, err
	}
	entries := make([]BriefingEntry, 0, len(resolved))
	for _, item := range resolved {
		entries = append(entries, briefingEntry(kind.dirName, item))
	}
	return entries, nil
}

// briefingEntry macht aus einem aufgelösten Eintrag die Zeile im Briefing.
func briefingEntry(dir string, item docDirEntry) BriefingEntry {
	entry := BriefingEntry{
		Ref:      DocSchemeKPlaybook + ":/" + path.Join(dir, item.Name),
		Origin:   item.Origin,
		Location: item.Location,
	}
	if item.Disabled {
		entry.Origin = BriefingOriginDisabled
	}
	entry.Title, entry.Description = docTitleAndDescription(item.Name, item.Content)
	return entry
}

// docTitleAndDescription liest Titel und Beschreibung eines Dokuments: beide
// aus dem Frontmatter, der Titel ersatzweise aus der ersten Überschrift
// beliebiger Tiefe, sonst der Dateiname. Ein Frontmatter, das yamllite nicht
// versteht, ist kein Fehler: dann gilt die Überschrift.
func docTitleAndDescription(name string, content []byte) (string, string) {
	title, description := "", ""
	if block, ok := inventory.FrontmatterBlock(content); ok {
		if root, err := yamllite.Parse([]byte(block)); err == nil {
			title = strings.TrimSpace(root.Get("title").Str())
			description = strings.TrimSpace(root.Get("description").Str())
		}
	}
	if title == "" {
		title = firstHeading(inventory.Body(content))
	}
	if title == "" {
		title = name
	}
	return title, description
}

// firstHeading liefert den Text der ersten Markdown-Überschrift eines Rumpfs,
// leer ohne. Überschriften in Code-Blöcken zählen nicht.
func firstHeading(body []byte) string {
	inFence := false
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.HasPrefix(trimmed, "#") {
			continue
		}
		text := strings.TrimLeft(trimmed, "#")
		if text == "" || !strings.HasPrefix(text, " ") {
			continue
		}
		return strings.TrimSpace(text)
	}
	return ""
}

// briefingWork liest die eigene Arbeit aus der Collection: offene Tasks,
// aktive Arbeitsstände, die Zahl der offenen Befunde — das, was
// k_playbook_doc_list ohne done liefert, sortiert und je Liste auf
// BriefingWorkLimit gedeckelt; die volle Zahl steht unter Total. Jeder Fehler
// dabei ist ein Zustand: Available false mit Message, die Listen leer.
func briefingWork(ctx context.Context, projectDir string) BriefingWork {
	work := BriefingWork{Tasks: []BriefingDoc{}, Workbench: []BriefingDoc{}}
	docs, err := OpenBriefingDocs(projectDir)
	if err != nil {
		work.Message = err.Error()
		return work
	}
	user := docs.User()

	tasks, err := briefingDocs(ctx, docs, "tasks")
	if err != nil {
		work.Message = err.Error()
		return work
	}
	workbench, err := briefingDocs(ctx, docs, "workbench")
	if err != nil {
		work.Message = err.Error()
		return work
	}
	findings, err := docs.List(ctx, DocListOptions{Kind: "findings"})
	if err != nil {
		work.Message = err.Error()
		return work
	}

	sortBriefingDocsByNumber(tasks)
	sortBriefingDocsByUpdated(workbench)

	work.Available = true
	work.User = user
	work.Total = BriefingWorkTotal{Tasks: len(tasks), Workbench: len(workbench)}
	work.Tasks = capBriefingDocs(tasks)
	work.Workbench = capBriefingDocs(workbench)
	work.OpenFindings = len(findings.Items)
	return work
}

// briefingDocs listet die eigenen Dokumente einer Sorte, die nicht in done/
// liegen.
func briefingDocs(ctx context.Context, docs *UserDocs, kind string) ([]BriefingDoc, error) {
	list, err := docs.List(ctx, DocListOptions{Kind: kind})
	if err != nil {
		return nil, err
	}
	items := make([]BriefingDoc, 0, len(list.Items))
	for _, item := range list.Items {
		items = append(items, BriefingDoc{Ref: item.Ref, Title: item.Title, Updated: item.Updated, Problem: item.Problem})
	}
	return items, nil
}

// capBriefingDocs kürzt eine sortierte Liste auf BriefingWorkLimit.
func capBriefingDocs(items []BriefingDoc) []BriefingDoc {
	if len(items) > BriefingWorkLimit {
		return items[:BriefingWorkLimit]
	}
	return items
}

// sortBriefingDocsByNumber ordnet nach der Nummer im Dateinamen, die kleinste
// zuerst; bei gleicher Nummer und ohne Nummer nach dem Namen, Dokumente ohne
// Nummer am Ende.
func sortBriefingDocsByNumber(items []BriefingDoc) {
	sort.SliceStable(items, func(i, j int) bool {
		left, right := path.Base(items[i].Ref), path.Base(items[j].Ref)
		leftNumber, leftOK := docNumber(left)
		rightNumber, rightOK := docNumber(right)
		switch {
		case leftOK != rightOK:
			return leftOK
		case leftOK && leftNumber != rightNumber:
			return leftNumber < rightNumber
		}
		return left < right
	})
}

// sortBriefingDocsByUpdated ordnet nach der letzten Änderung, die neueste
// zuerst; bei gleichem Zeitpunkt bleibt die Reihenfolge der Liste (nach
// Namen). Ein Zeitpunkt, der sich nicht lesen lässt, zählt als der älteste.
func sortBriefingDocsByUpdated(items []BriefingDoc) {
	stamp := func(doc BriefingDoc) time.Time {
		at, err := time.Parse(time.RFC3339Nano, doc.Updated)
		if err != nil {
			return time.Time{}
		}
		return at
	}
	sort.SliceStable(items, func(i, j int) bool {
		return stamp(items[i]).After(stamp(items[j]))
	})
}
