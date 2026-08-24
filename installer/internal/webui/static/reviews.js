// Seite "Reviews": nur die bisherigen Läufe. Gestartet wird ein Lauf im
// Assistenten über /k-audit oder /k-review, nicht hier — die Oberfläche zeigt
// ausschließlich, was bereits vorliegt.

const elements = {
  runsPill: document.getElementById("runs-pill"),
  runsList: document.getElementById("runs-list"),
  runsMessage: document.getElementById("runs-message"),
};

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

// Ohne Lebenszeichen von dieser Seite beendet sich der Server wenige Sekunden
// nach dem Wechsel hierher — der Weg zurück führte dann ins Leere.
startSession((message) => {
  elements.runsMessage.textContent = message;
});
load();
