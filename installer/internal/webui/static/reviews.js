"use strict";

// Seite "Reviews": die bisherigen Läufe aus k-playbook-local/results/.
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
  runsList: document.getElementById("runs-list"),
  runsMessage: document.getElementById("runs-message"),
};

load();

async function load() {
  try {
    const response = await fetch("/api/reviews", { cache: "no-store" });
    render(await response.json());
  } catch {
    elements.runsMessage.textContent = "Läufe konnten nicht geladen werden.";
  }
}

function render(data) {
  elements.runsList.replaceChildren();
  elements.runsMessage.textContent = data.message || "";

  if (!data.available) {
    elements.runsPill.className = "pill muted";
    elements.runsPill.textContent = "Unbekannt";
    elements.runsMessage.textContent = "Keine Projektkonfiguration gefunden.";
    return;
  }

  if (data.message) {
    elements.runsPill.className = "pill warn";
    elements.runsPill.textContent = "Nicht lesbar";
    return;
  }

  const runs = data.runs || [];
  elements.runsPill.className = runs.length ? "pill ok" : "pill muted";
  elements.runsPill.textContent = runs.length ? `${runs.length}` : "keine";

  if (runs.length === 0) {
    elements.runsMessage.textContent = "Noch kein Lauf angelegt.";
    return;
  }

  for (const run of runs) {
    const row = document.createElement("div");
    const term = document.createElement("dt");
    term.textContent = run.name;
    const detail = document.createElement("dd");
    // Ein Verzeichnis ohne run.json stammt aus der Zeit vor diesem Modell. Das
    // gehört gesagt, statt es als leeren Lauf auszugeben.
    detail.textContent = run.hasRunFile
      ? `${run.state} — ${run.entryCount} Einträge`
      : "ohne run.json";
    row.append(term, detail);
    elements.runsList.append(row);
  }
}
