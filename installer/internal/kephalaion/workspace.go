package kephalaion

// WorkspaceFolder ist ein Ordner einer VS-Code-Workspace-Datei, wie die
// Kephalaion-Erweiterung eine Collection öffnet: keph://<hub>/<collection>.
type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// WorkspaceFolders nennt die Collections, die ein VS-Code-Fenster des
// Projekts eingebunden haben soll: nur die des Projekts. Die öffentliche
// Collection k-playbook gehört nicht dazu; ihr Inhalt kommt über k-playbook
// selbst (k-playbook:-Verweise).
//
// Geprüft werden nur Hub und Collection, denn nur sie fließen in die Adresse.
// Der Account gehört zur Anmeldung, die die Erweiterung selbst führt.
func WorkspaceFolders(b Binding) ([]WorkspaceFolder, error) {
	if err := CheckName("Hub", b.Hub); err != nil {
		return nil, err
	}
	if err := CheckName("Collection", b.Collection); err != nil {
		return nil, err
	}
	return []WorkspaceFolder{workspaceFolder(b.Hub, b.Collection)}, nil
}

func workspaceFolder(hub, collection string) WorkspaceFolder {
	return WorkspaceFolder{
		URI:  "keph://" + hub + "/" + collection,
		Name: "Keph " + hub + ":" + collection,
	}
}
