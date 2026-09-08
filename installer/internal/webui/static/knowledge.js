"use strict";

// Seite "Knowledge": vorerst genau eine Datei, die Wissensablage aus der
// mitgelieferten Doku. Die Auflistung der abgelegten Einträge kommt später als
// weiterer Block darunter.
//
// Angezeigt wird mit demselben Betrachter wie im Bereich Docs (docview.js) —
// die Datei besteht im Wesentlichen aus einem Mermaid-Diagramm, und das soll
// hier wie dort gezeichnet werden.

// Die Datei liegt in der mitgelieferten Doku und wird über deren Endpunkt
// gelesen. Ein eigener Endpunkt käme erst mit der Auflistung infrage.
const KNOWLEDGE_FILE = "knowledge-storage.md";

const elements = {
  title: document.getElementById("knowledge-title"),
  path: document.getElementById("knowledge-path"),
  viewer: document.getElementById("knowledge-viewer"),
  message: document.getElementById("knowledge-message"),
};

// Muss vor dem Laden laufen: die Karte trägt einen Eintrag im Blockmenü.
buildBlockNav();

startSession((message) => {
  elements.message.textContent = message;
});

// Ein Verweis im Text führt in den Bereich Docs: dort steht der Index daneben,
// und diese Seite zeigt bewusst nur die eine Datei.
setUpDocLinks(elements.viewer, (target, anchor) => {
  const path = resolveDocPath(KNOWLEDGE_FILE, target);
  window.location.href = `/docs?file=${encodeURIComponent(path)}${anchor ? `#${anchor}` : ""}`;
});

load();

async function load() {
  elements.viewer.textContent = "Wird geladen...";

  try {
    const response = await fetch(`/api/docs/file?path=${encodeURIComponent(KNOWLEDGE_FILE)}`, { cache: "no-store" });
    render(await response.json());
  } catch {
    elements.viewer.textContent = "";
    elements.message.textContent = "Die Wissensablage konnte nicht geladen werden.";
  }
}

function render(data) {
  elements.message.textContent = data.message || "";

  if (!data.available) {
    elements.viewer.textContent = "";
    elements.message.textContent = "Keine Projektkonfiguration gefunden.";
    return;
  }

  // Die Installation daneben kann einen älteren Stand tragen, in dem es die
  // Datei noch nicht gibt. Dann steht der Grund in der Meldung — eine leere
  // Karte wäre dafür die falsche Auskunft.
  if (data.message) {
    elements.viewer.textContent = "";
    return;
  }

  elements.title.textContent = data.title || "Knowledge";
  elements.path.textContent = data.path || KNOWLEDGE_FILE;
  showDoc(elements.viewer, data.html);
}
