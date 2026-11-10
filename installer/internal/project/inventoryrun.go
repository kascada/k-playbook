package project

import (
	"context"
	"path"
	"strings"

	"github.com/kascada/k-playbook/installer/internal/inventory"
)

// Das Inventar und die README seines Verzeichnisses (Task 137). Das Paket
// internal/inventory schreibt nur sein eines Dokument; die README von
// docs/generated/versions/ kommt aus ihrer Vorlage über DocsReadmes, und die
// kennt nur dieses Paket — internal/project importiert internal/inventory,
// der Rückweg wäre ein Importzyklus. Deshalb erledigt RunInventory beides
// nacheinander, und jeder Weg zum Inventar ruft es: das Subkommando
// `k-playbook inventory` und die Oberfläche.

// inventoryTemplates ist die mitgelieferte Ebene mit den README-Vorlagen
// für RunInventory. Austauschbar, damit Tests den Lieferumfang selbst
// mitbringen; sonst NewProjectDefaultsSource.
var inventoryTemplates = NewProjectDefaultsSource

// InventoryReadmeDir ist das Verzeichnis des Inventars relativ zu docs/ —
// der Pfad seiner README-Vorlage k-playbook:/docs/generated/versions/README.md.
var InventoryReadmeDir = strings.TrimPrefix(path.Dir(inventory.DocName), docDocsRoot+"/")

// RunInventory erhebt das Inventar und schreibt es (inventory.Run), danach
// die README seines Verzeichnisses aus ihrer Vorlage, wenn sie fehlt oder
// abweicht. Scheitert der Lauf des Inventars, ist das der Fehler, und die
// README wird nicht versucht (readme nil). Scheitert nur die README — etwa
// weil die Vorlage auf dem Hub fehlt (template_missing, Task 133) —, steht
// das in readme, das Inventar ist trotzdem geschrieben.
func RunInventory(ctx context.Context, projectDir string, store inventory.Store, options inventory.Options) (inventory.Result, inventory.Outcome, *DocsReadmeItem, error) {
	result, outcome, err := inventory.Run(ctx, store, options)
	if err != nil {
		return result, outcome, nil, err
	}
	readme := NewDocsReadmes(store, inventoryTemplates(projectDir)).One(ctx, InventoryReadmeDir, DocsReadmeOptions{})
	return result, outcome, &readme, nil
}
