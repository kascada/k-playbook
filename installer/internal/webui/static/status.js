"use strict";

// Seite "Status": die Startseite der Oberfläche. Sie zeigt, wie es um das
// Projekt steht, und ist als Gerüst angelegt, das später gefüllt wird.
//
// Nichts davon steht hier ein zweites Mal: Knöpfe und Sperrfläche bedient
// service.js, die Zahlen der Workflow-Karten holt workflows.js, und das Bild
// der Wissensablage holt und zeichnet docview.js. Klassisches Skript wie alle
// übrigen — jeder Name auf oberster Ebene teilt sich den Namensraum mit
// session.js, nav.js, service.js, docview.js und workflows.js.

// Die Datei, deren erstes Diagramm die Karte zeigt.
const STATUS_KNOWLEDGE_FILE = "knowledge-storage.md";

// Muss vor den Ladefunktionen laufen: die blenden Blöcke ein, und das Menü
// zieht das nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();

startSession(showClosed);

loadWorkflowCounts(document.getElementById("status-workflows-message"));
loadKnowledgePicture();

async function loadKnowledgePicture() {
  const viewer = document.getElementById("knowledge-picture");
  const message = document.getElementById("knowledge-picture-message");
  try {
    const data = await loadDocInto(viewer, STATUS_KNOWLEDGE_FILE, { firstDiagramOnly: true });
    message.textContent = data.available ? data.message || "" : "Keine Projektkonfiguration gefunden.";
  } catch {
    viewer.textContent = "";
    message.textContent = "Das Bild der Wissensablage konnte nicht geladen werden.";
  }
}
