"use strict";

// Seite "Inventar": das Versionsinventar des Projekts.
//
// Stand und Quellenkonfiguration kommen von /api/inventory, die erzeugte Datei
// gerendert von /api/inventory/file. Der einzige schreibende Weg ist der
// Aktualisieren-Knopf: POST /api/inventory stößt die Erhebung an — dieselbe
// Fachlogik wie `k-playbook inventory`, nichts davon wird hier nachgebaut.
//
// Die Quellenkonfiguration wird nur gezeigt, nicht bearbeitet. Es gibt hier
// kein Formular und keinen Endpunkt, der in sie schreibt.

const elements = {
  statusPill: document.getElementById("status-pill"),
  statusFacts: document.getElementById("status-facts"),
  statusMessage: document.getElementById("status-message"),
  run: document.getElementById("inventory-run"),
  runProgress: document.getElementById("run-progress"),
  runProgressText: document.getElementById("run-progress-text"),
  runCard: document.getElementById("run-card"),
  runPill: document.getElementById("run-pill"),
  runMessage: document.getElementById("run-message"),
  runFacts: document.getElementById("run-facts"),
  runFindings: document.getElementById("run-findings"),
  sourcesPill: document.getElementById("sources-pill"),
  sourcesFacts: document.getElementById("sources-facts"),
  sourcesMessage: document.getElementById("sources-message"),
  filePath: document.getElementById("file-path"),
  fileViewer: document.getElementById("file-viewer"),
  fileMessage: document.getElementById("file-message"),
};

// Muss vor den Ladefunktionen laufen: die blenden Karten ein, und das Menü
// zieht das nur mit, wenn es sie schon beobachtet.
buildBlockNav();
// Das Lebenszeichen dieses Fensters: es hält den Dienst aus dem Leerlauf und
// merkt, wenn er weg ist.
startSession((message) => {
  elements.statusMessage.textContent = message;
  elements.run.disabled = true;
});
elements.run.addEventListener("click", runInventory);
loadInventory();
loadFile();

async function loadInventory() {
  try {
    const response = await fetch("/api/inventory", { cache: "no-store" });
    renderInventory(await response.json());
  } catch {
    elements.statusMessage.textContent = "Stand konnte nicht geladen werden.";
  }
}

async function loadFile() {
  elements.fileViewer.classList.add("empty");
  elements.fileViewer.textContent = "Wird geladen...";
  try {
    const response = await fetch("/api/inventory/file", { cache: "no-store" });
    renderFile(await response.json());
  } catch {
    elements.fileViewer.textContent = "";
    elements.fileMessage.textContent = "Inventardatei konnte nicht geladen werden.";
  }
}

// Der Anstoß. Der Knopf ist gesperrt, solange der Lauf steht, und der Ring
// daneben sagt, dass gearbeitet wird — ein Lauf liest jede Quelle des Projekts
// und kann spürbar dauern.
async function runInventory() {
  setRunning(true, "Erhebung läuft — Quellen werden gelesen, das Ergebnis wird mit dem Bestand verglichen...");
  try {
    const response = await fetch("/api/inventory", { method: "POST" });
    const data = await response.json();
    renderRun(data);
    if (data.available) {
      renderInventory(data);
      await loadFile();
    }
  } catch {
    elements.runCard.classList.remove("hidden");
    elements.runPill.className = "pill error";
    elements.runPill.textContent = "Fehlgeschlagen";
    elements.runMessage.textContent = "Die Erhebung konnte nicht angestoßen werden.";
  } finally {
    setRunning(false);
  }
}

function setRunning(running, text = "") {
  elements.run.disabled = running;
  elements.runProgress.classList.toggle("hidden", !running);
  elements.runProgressText.textContent = text;
}

// Stand und Quellenkonfiguration. Dieselbe Funktion zeichnet auch die Antwort
// des Anstoßes: sie trägt `status` und `displayPath` in derselben Form.
function renderInventory(data) {
  elements.statusFacts.replaceChildren();
  elements.sourcesFacts.replaceChildren();
  elements.statusMessage.textContent = "";
  elements.sourcesMessage.textContent = "";

  if (!data.available) {
    elements.statusPill.className = "pill muted";
    elements.statusPill.textContent = "Nicht anwendbar";
    elements.statusMessage.textContent = data.message || "Keine Projektkonfiguration gefunden.";
    elements.sourcesPill.className = "pill muted";
    elements.sourcesPill.textContent = "Nicht anwendbar";
    elements.run.disabled = true;
    return;
  }

  renderStatus(data.status || {}, data.displayPath || "");
  if (data.sources) {
    renderSources(data.sources);
  }
  elements.run.disabled = false;
}

function renderStatus(status, displayPath) {
  addFact(elements.statusFacts, "Datei", displayPath || status.path || "");

  if (!status.present) {
    elements.statusPill.className = "pill warn";
    elements.statusPill.textContent = "Fehlt";
    addFact(elements.statusFacts, "Stand", "noch nicht erhoben");
    return;
  }

  // Ein Befund zum Bestand — defektes oder unvollständiges Frontmatter — ist
  // sichtbar, kein stilles Nullergebnis. Der nächste Lauf schreibt neu.
  if (status.problem) {
    elements.statusPill.className = "pill error";
    elements.statusPill.textContent = "Befund";
    elements.statusMessage.textContent = `Bestand: ${status.problem}. Der nächste Lauf schreibt die Datei neu.`;
  } else {
    elements.statusPill.className = "pill ok";
    elements.statusPill.textContent = "Vorhanden";
  }

  addFact(elements.statusFacts, "Zuletzt inhaltlich geändert", status.generatedAt || "unbekannt");
  addFact(elements.statusFacts, "Erzeuger", status.generatedBy || "unbekannt");
  addFact(elements.statusFacts, "Quellen gelesen", String(status.sourcesRead ?? 0));
  addFact(elements.statusFacts, "Quellen konfiguriert", String(status.sourcesConfigured ?? 0));
  addFact(elements.statusFacts, "Einträge", String(status.entries ?? 0));
  addFact(elements.statusFacts, "Abweichungen", String(status.deviations ?? 0));
  addFact(elements.statusFacts, "Abgelehnte Quellen", String(status.rejected ?? 0));
  addFact(elements.statusFacts, "Nicht durchsuchte Quellen", String(status.sourcesExcluded ?? 0));
}

// Die Quellenkonfiguration: Ort, Zustand, Zahlen. Kein Formular.
function renderSources(sources) {
  addFact(elements.sourcesFacts, "Datei", sources.displayPath || sources.path || "");

  if (!sources.present) {
    elements.sourcesPill.className = "pill muted";
    elements.sourcesPill.textContent = "Fehlt";
    elements.sourcesMessage.textContent =
      "Kein Fehler: es gelten die Standardquellen unterhalb der Projektwurzel. Das Einrichten auf der Startseite legt die leere Vorlage an.";
    return;
  }

  if (sources.error) {
    elements.sourcesPill.className = "pill error";
    elements.sourcesPill.textContent = "Defekt";
    elements.sourcesMessage.textContent = `${sources.error}\nEine Erhebung bricht damit ab, bis die Datei repariert ist.`;
    return;
  }

  elements.sourcesPill.className = "pill ok";
  elements.sourcesPill.textContent = "Vorhanden";
  addFact(elements.sourcesFacts, "Zusätzliche Wurzeln", String(sources.roots ?? 0));
  addFact(elements.sourcesFacts, "Zusätzliche Quellen", String(sources.sources ?? 0));
  addFact(elements.sourcesFacts, "Ausschlussmuster", String(sources.exclude ?? 0));
}

// Das Ergebnis eines Anstoßes: Erfolg oder Abbruch, die Zahlen des Laufs, und
// jede Ablehnung, jeder greifende Ausschluss und jeder Hinweis im Wortlaut.
// Eine Ablehnung ist eine Lücke im Inventar; sie hier wegzulassen wäre der
// Fehler.
function renderRun(data) {
  elements.runCard.classList.remove("hidden");
  elements.runFacts.replaceChildren();
  elements.runFindings.replaceChildren();
  elements.runMessage.textContent = "";

  if (!data.available) {
    elements.runPill.className = "pill muted";
    elements.runPill.textContent = "Nicht anwendbar";
    elements.runMessage.textContent = data.message || "Keine Projektkonfiguration gefunden.";
    return;
  }

  if (!data.ok) {
    elements.runPill.className = "pill error";
    elements.runPill.textContent = "Abgebrochen";
    elements.runMessage.textContent = `${data.message || "Erhebung abgebrochen."}\nEs wurde nichts geschrieben.`;
    return;
  }

  const outcome = data.outcome || {};
  const summary = data.summary || {};
  const rejected = summary.rejected || 0;

  elements.runPill.className = rejected > 0 ? "pill warn" : "pill ok";
  elements.runPill.textContent = rejected > 0 ? "Mit Ablehnungen" : "Erfolgreich";
  elements.runMessage.textContent = outcome.written
    ? `Geschrieben: ${data.displayPath || outcome.path} (erhoben ${outcome.at || ""}).`
    : `Unverändert: ${data.displayPath || outcome.path} — die Erhebung ist inhaltlich dieselbe (erhoben ${outcome.at || ""}).`;
  if (outcome.problem) {
    elements.runMessage.textContent += `\nHinweis zum Bestand: ${outcome.problem}`;
  }

  addFact(elements.runFacts, "Ausgewertete Quellen", String(summary.sources || 0));
  addFact(elements.runFacts, "Konfigurierte Zusatzquellen", String(summary.configuredSources || 0));
  addFact(elements.runFacts, "Einträge", String(summary.entries || 0));
  addFact(elements.runFacts, "Abweichungen", describeDeviations(summary));
  addFact(elements.runFacts, "Abgelehnte Quellen", String(rejected));
  addFact(elements.runFacts, "Nicht durchsuchte Quellen", String(summary.excluded || 0));
  addFact(elements.runFacts, "Hinweise", String(summary.notes || 0));

  for (const rejection of data.rejections || []) {
    elements.runFindings.append(findingBox("Abgelehnt", describeRejection(rejection), true));
  }
  for (const exclusion of data.exclusions || []) {
    if (!exclusion.skipped) {
      continue;
    }
    elements.runFindings.append(
      findingBox(
        "Nicht durchsucht",
        `${exclusion.pattern} (${exclusion.origin}): ${exclusion.skipped} Quellen übergangen — ${exclusion.reason}`,
        false,
      ),
    );
  }
  for (const note of data.notes || []) {
    elements.runFindings.append(findingBox("Hinweis", note.source ? `${note.source}: ${note.text}` : note.text, false));
  }
}

// Gruppen, nicht Zeilen; die widersprüchlichen daneben, weil sie die Frage
// aufwerfen: dieselbe Umgebung sagt Verschiedenes.
function describeDeviations(summary) {
  const total = summary.deviations || 0;
  const conflicting = summary.conflicting || 0;
  return conflicting > 0 ? `${total} (davon ${conflicting} widersprüchlich)` : String(total);
}

// Angefragter und aufgelöster Pfad, damit erkennbar ist, was tatsächlich
// gelesen worden wäre — dieselbe Form wie in der Ausgabe des Subkommandos.
function describeRejection(rejection) {
  if (!rejection.resolved || rejection.resolved === rejection.requested) {
    return `${rejection.requested}: ${rejection.reason}`;
  }
  return `${rejection.requested} → ${rejection.resolved}: ${rejection.reason}`;
}

function findingBox(kind, text, warning) {
  const box = document.createElement("div");
  box.className = "setting";

  const title = document.createElement("div");
  title.className = warning ? "setting-state warn" : "setting-state";
  title.textContent = kind;
  const body = document.createElement("p");
  body.className = "setting-note";
  body.textContent = text;
  box.append(title, body);
  return box;
}

function renderFile(data) {
  elements.fileMessage.textContent = data.message || "";
  elements.filePath.textContent = data.path || "";

  if (!data.available) {
    elements.fileViewer.textContent = "";
    elements.fileMessage.textContent = "Keine Projektkonfiguration gefunden.";
    return;
  }
  if (!data.present || !data.html) {
    elements.fileViewer.textContent = "";
    return;
  }

  elements.fileViewer.classList.remove("empty");
  // Das HTML kommt aus dem eigenen Backend. Gerendert wird dort mit
  // abgeschaltetem Roh-HTML, es steht also nichts darin, was nicht aus der
  // Markdown-Struktur der Datei stammt.
  elements.fileViewer.innerHTML = data.html;
}

// Legt eine Zeile in einer Faktenliste an. Dieselbe Form wie auf der
// Startseite; die Seiten teilen sich kein Skript außer session.js und nav.js.
function addFact(list, term, detail) {
  const row = document.createElement("div");
  const dt = document.createElement("dt");
  dt.textContent = term;
  const dd = document.createElement("dd");
  dd.textContent = detail;
  row.append(dt, dd);
  list.append(row);
  return row;
}
