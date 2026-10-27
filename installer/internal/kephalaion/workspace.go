package kephalaion

// PublicCollection ist die öffentliche Collection mit dem Mitgelieferten von
// k-playbook. Jeder Hub führt sie unter diesem Namen; die Anbindung des
// Projekts nennt sie deshalb nicht.
const PublicCollection = "k-playbook"

// WorkspaceFolder ist ein Ordner einer VS-Code-Workspace-Datei, wie die
// Kephalaion-Erweiterung eine Collection öffnet: keph://<hub>/<collection>.
type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

// WorkspaceFolders nennt die Collections, die ein VS-Code-Fenster des
// Projekts eingebunden haben soll: zuerst die öffentliche, dann die des
// Projekts. Ist die Collection des Projekts die öffentliche, steht sie einmal
// da.
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
	folders := []WorkspaceFolder{workspaceFolder(b.Hub, PublicCollection)}
	if b.Collection != PublicCollection {
		folders = append(folders, workspaceFolder(b.Hub, b.Collection))
	}
	return folders, nil
}

func workspaceFolder(hub, collection string) WorkspaceFolder {
	return WorkspaceFolder{
		URI:  "keph://" + hub + "/" + collection,
		Name: "Keph " + hub + ":" + collection,
	}
}
