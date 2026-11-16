"use strict";

// Seite "Reviews": Solange die Review-Wege pausieren, zeigt sie statt der
// Läufe die Meldung der Sperre aus /api/reviews.
//
// Gelesen wird nur. Ein Lauf wird über /k-audit oder /k-review angestoßen,
// nicht über die Oberfläche.

// Muss vor der Ladefunktion laufen: die blendet Blöcke ein, und das Menü zieht
// das nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();

startSession((message) => {
  document.getElementById("runs-message").textContent = message;
});

const elements = {
  runsPill: document.getElementById("runs-pill"),
  runsMessage: document.getElementById("runs-message"),
};

load();

async function load() {
  try {
    const response = await fetch("/api/reviews", { cache: "no-store" });
    render(await response.json());
  } catch {
    elements.runsMessage.textContent = "Der Stand der Reviews konnte nicht geladen werden.";
  }
}

function render(data) {
  if (!data.available) {
    elements.runsPill.className = "pill muted";
    elements.runsPill.textContent = "Unbekannt";
    elements.runsMessage.textContent = "Keine Projektkonfiguration gefunden.";
    return;
  }

  elements.runsPill.className = data.paused ? "pill muted" : "pill warn";
  elements.runsPill.textContent = data.paused ? "Pausiert" : "Nicht lesbar";
  elements.runsMessage.textContent = data.message || "";
}
