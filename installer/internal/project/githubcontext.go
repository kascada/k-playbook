package project

import (
	"context"
	"strings"
	"time"

	"github.com/kascada/k-playbook/installer/internal/github"
	"github.com/kascada/k-playbook/installer/internal/githubaccess"
)

// ContextGitHub ist das Feld github der Kontextausgabe (Task 119): welcher
// Dienst der persönlichen Zugangsdatei (docs/github-access.md) zum Repo des
// Arbeitsstands gehört und welche Zugänge `k-playbook github run` dafür
// wählt, dazu die übrigen Dienste. Dieselbe Quelle und dieselbe Auswahl wie
// `github show`: Path, Peek statt Load, Judge mit selected.
type ContextGitHub struct {
	// File ist der Ort der Datei nach githubaccess.Path, Present, ob sie
	// dort liegt. Fehlt sie, legt das Feld keine Vorlage an. Im Briefing
	// läuft aber die Umgebungsprüfung vorher (healthGitHub → Load) und legt
	// sie an; dort ist Present deshalb nur falsch, wenn das nicht ging.
	File    string `json:"file"`
	Present bool   `json:"present"`
	// Problems sind die Probleme des Lesers in der Form von `github check
	// --json`. Bei einem error sind Services leer und ProjectServices auch.
	Problems []githubaccess.Problem `json:"problems"`
	// Remote ist der rohe Eintrag des zählenden Remotes (Upstream des
	// Branches, sonst origin) ohne Zugangsdaten, Repo daraus <owner>/<repo>.
	// Beide leer ohne Remote oder ohne eine GitHub-Form.
	Remote string `json:"remote"`
	Repo   string `json:"repo"`
	// ProjectServices sind die Dienste, deren repo gleich Repo ist —
	// Groß- und Kleinschreibung zählen nicht —, in Dateireihenfolge.
	ProjectServices []string `json:"projectServices"`
	// Services ist dasselbe Objekt wie bei `github show --json`, mit state,
	// reason und selected.
	Services githubaccess.Services `json:"services"`
}

// githubContextTimeout begrenzt die drei git-Aufrufe, mit denen das Remote
// gelesen wird. Sie lesen nur die lokale Konfiguration.
const githubContextTimeout = 5 * time.Second

// contextGitHub baut das Feld github. gitDir ist das Verzeichnis, ab dem git
// das Repo sucht — das Startverzeichnis des Aufrufs, damit ein eigenes Repo
// unterhalb des Projekts (etwa k-playbook-local/) sein eigenes Remote
// bekommt. Kein Netz, kein Token-Wert, keine Datei geschrieben; die Kosten
// sind Judge (gh-Konfiguration, Dateien, `ssh -G` je ssh host, Frist 3 s je
// Host), drei git-Aufrufe und höchstens ein weiteres `ssh -G` für einen
// Remote-Alias, den die Datei nicht nennt.
func contextGitHub(gitDir string) *ContextGitHub {
	field := &ContextGitHub{Problems: []githubaccess.Problem{}, ProjectServices: []string{}, Services: githubaccess.Services{}}
	path, err := githubaccess.Path()
	if err != nil {
		field.Problems = append(field.Problems, githubaccess.Problem{
			Level:   githubaccess.LevelError,
			Path:    githubaccess.PathFile,
			Message: "Der Ort der GitHub-Zugangsdatei ist unbekannt: " + err.Error() + ".",
			Hint:    "HOME oder XDG_CONFIG_HOME setzen oder den Pfad in " + githubaccess.EnvConfig + " nennen.",
		})
	} else {
		result, present := githubaccess.Peek(path)
		field.File, field.Present, field.Problems = path, present, result.Problems
		if present && !result.HasErrors() && result.Services != nil {
			field.Services = githubaccess.Judge(result.Services)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), githubContextTimeout)
	defer cancel()
	remote := projectRemoteURL(ctx, gitDir)
	field.Repo = githubRemoteRepo(remote, field.Services)
	if field.Repo == "" {
		return field
	}
	field.Remote = githubaccess.RemoteWithoutCredentials(remote)
	for _, service := range field.Services {
		if strings.EqualFold(service.Repo, field.Repo) {
			field.ProjectServices = append(field.ProjectServices, service.Name)
		}
	}
	return field
}

// githubSSHHostIsGitHub ist der Blick in die SSH-Konfiguration für einen
// Alias, den die Zugangsdatei nicht nennt; austauschbar, damit Tests zählen
// können, wann er nötig ist.
var githubSSHHostIsGitHub = githubaccess.SSHHostIsGitHub

// githubRemoteRepo liest <owner>/<repo> aus einem rohen Remote-Eintrag
// (projectRemoteURL) in den Formen, die `k-playbook github run` umlenkt:
// https, git@github.com:, ssh://git@github.com/ und git@<alias>:. Leer ohne
// Eintrag, für einen fremden Host und für jede andere Form.
//
// Den Host erkennt github.SSHAliasOf; leer heißt https oder github.com.
// Ob ein anderer Host ein Alias für GitHub ist, entscheidet zuerst die
// Zugangsdatei: Nennt ein Dienst ihn als ssh host, lenkt run genau diese Form
// um (formsOf), und sie gilt ohne Unterprozess. Erst ein Alias, den die Datei
// nicht nennt, wird mit `ssh -G` aufgelöst und zählt, wenn er nach github.com
// führt. Nur nach der Form zu entscheiden (<alias>:<owner>/<repo>) reicht
// nicht: git@gitlab.com:o/r hat dieselbe Form und ist kein GitHub-Repo. Nur
// ssh -G zu fragen, kostete bei jedem Briefing einen Unterprozess für einen
// Alias, den die Datei schon kennt — Judge löst ihn ohnehin auf.
func githubRemoteRepo(remote string, services githubaccess.Services) string {
	if remote == "" {
		return ""
	}
	alias := github.SSHAliasOf(remote)
	repo := githubaccess.RepoOfRemote(remote, alias)
	// Die Form zuerst: Was ohnehin keine ist, braucht kein ssh -G.
	if repo == "" || alias == "" || services.NamesSSHHost(alias) || githubSSHHostIsGitHub(alias) {
		return repo
	}
	return ""
}
