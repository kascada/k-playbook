"use strict";

// Seite "Docs": die mitgelieferte Doku aus k-playbook/docs.
//
// Der Index steht links im Menü, der gerenderte Text rechts. Anders als auf
// den übrigen Seiten entsteht das Menü nicht aus den Karten daneben, sondern
// aus der geladenen Dateiliste — dieser Bereich hat genau einen Inhalt.
//
// Quelle ist /api/docs; gelesen wird nur. Das Anzeigen selbst — Anker,
// Querverweise, Mermaid — steht in docview.js und wird mit /knowledge geteilt;
// hier steht, welche Datei geöffnet wird.

const elements = {
  blockNav: document.getElementById("block-nav"),
  docCard: document.getElementById("doc-card"),
  docTitle: document.getElementById("doc-title"),
  docPath: document.getElementById("doc-path"),
  docViewer: document.getElementById("doc-viewer"),
  docsMessage: document.getElementById("docs-message"),
};

// Die offene Datei; Verweise darin werden relativ zu ihr aufgelöst.
let currentDocPath = "";

// Das Lebenszeichen dieses Fensters: es hält den Dienst aus dem Leerlauf und
// merkt, wenn er weg ist.
startSession((message) => {
  elements.docsMessage.textContent = message;
});

// Ein Verweis im Text führt in dieselbe Karte: dieser Bereich zeigt die Doku,
// und das Menü daneben zieht mit.
setUpDocLinks(elements.docViewer, (target, anchor) => {
  openDoc(resolveDocPath(currentDocPath, target), "", anchor);
});
loadDocs();

async function loadDocs() {
  elements.docViewer.classList.add("empty");
  elements.docViewer.textContent = "Wird geladen...";

  try {
    const response = await fetch("/api/docs", { cache: "no-store" });
    renderDocs(await response.json());
  } catch {
    elements.docViewer.textContent = "";
    elements.docsMessage.textContent = "Doku konnte nicht geladen werden.";
  }
}

function renderDocs(data) {
  elements.blockNav.replaceChildren();
  elements.docsMessage.textContent = data.message || "";

  if (!data.available) {
    elements.docViewer.textContent = "";
    elements.docsMessage.textContent = "Keine Projektkonfiguration gefunden.";
    return;
  }

  // Fehlt das Verzeichnis, steht der Grund in der Meldung; eine leere Liste
  // wäre dafür die falsche Auskunft.
  if (data.message) {
    elements.docViewer.textContent = "";
    return;
  }

  const docs = data.docs || [];
  if (docs.length === 0) {
    elements.docViewer.textContent = "";
    elements.docsMessage.textContent = "Keine Markdown-Dateien vorhanden.";
    return;
  }

  for (const doc of docs) {
    elements.blockNav.append(docNavItem(doc));
  }

  // Eine andere Seite kann eine bestimmte Datei anfordern:
  // /docs?file=task-flow.md, wahlweise mit Anker dahinter. Das ist der Weg,
  // auf dem die Hilfe-Verweise der übrigen Seiten hierher zeigen — sie
  // brauchen dafür kein eigenes Skript und keine zweite Ansicht.
  //
  // Steht die angeforderte Datei nicht im Index, wird sie trotzdem geöffnet:
  // die Antwort des Servers sagt dann, dass es sie nicht gibt. Ersatzweise die
  // README zu zeigen wäre die falsche Auskunft — die Installation daneben kann
  // einen anderen Stand tragen, und ein Verweis geht dort ins Leere.
  const requested = new URLSearchParams(window.location.search).get("file");
  if (requested) {
    const doc = docs.find((entry) => entry.path === requested);
    openDoc(requested, doc ? doc.title : "", window.location.hash.slice(1));
    return;
  }

  // Ohne Auswahl steht die README da: sie ist der Einstieg und steht deshalb
  // auch in der Liste vorn.
  const start = docs.find((doc) => doc.path === "README.md") || docs[0];
  openDoc(start.path, start.title || start.path);
}

// Derselbe Eintrag wie im kartenbasierten Menü der übrigen Seiten — nur ohne
// Statuspunkt: eine Doku-Datei hat keinen Zustand, den er spiegeln könnte.
function docNavItem(doc) {
  const item = document.createElement("button");
  item.type = "button";
  item.className = "block-nav-item";
  item.dataset.path = doc.path;
  // Der Titel steht auf dem Eintrag, der Dateiname gehört trotzdem dazu.
  item.title = doc.path;

  const label = document.createElement("span");
  label.textContent = doc.title || doc.path;
  item.append(label);

  item.addEventListener("click", () => openDoc(doc.path, doc.title || doc.path));
  return item;
}

async function openDoc(path, title, anchor = "") {
  currentDocPath = path;
  elements.docTitle.textContent = title || path;
  elements.docPath.textContent = path;
  setActiveDocPath(path);
  elements.docViewer.classList.add("empty");
  elements.docViewer.textContent = "Wird geladen...";

  try {
    const response = await fetch(`/api/docs/file?path=${encodeURIComponent(path)}`, { cache: "no-store" });
    const data = await response.json();
    // Wurde inzwischen etwas anderes geöffnet, gehört diese Antwort nicht
    // mehr in die Karte.
    if (currentDocPath !== path) {
      return;
    }
    if (data.message) {
      elements.docViewer.textContent = data.message;
      return;
    }

    elements.docTitle.textContent = data.title || title || path;
    elements.docPath.textContent = data.path || path;
    showDoc(elements.docViewer, data.html, anchor);
  } catch {
    elements.docViewer.textContent = "Datei konnte nicht geladen werden.";
  }
}

// Markiert den Eintrag der offenen Datei. Ein Verweis im Text kann in eine
// Datei führen, die nicht angeklickt wurde — das Menü zieht dann mit.
function setActiveDocPath(path) {
  for (const item of elements.blockNav.querySelectorAll(".block-nav-item")) {
    if (item.dataset.path === path) {
      markBlockNavItem(item);
      return;
    }
  }

  // Kein Eintrag passt: ein Verweis kann in eine Datei außerhalb des Index
  // führen. Bliebe die alte Markierung stehen, zeigte das Menü auf eine Datei,
  // die gar nicht mehr offen ist.
  clearBlockNavMarking();
}
