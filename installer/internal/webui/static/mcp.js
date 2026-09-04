// Seite "k-playbook-MCP": der Registrierungszustand je Assistent und die
// Werkzeuge, die der Server tatsächlich anbietet.
//
// Gelesen wird nur. Eingerichtet wird im Block auf der Startseite — dort steht
// der Knopf, hier stehen die Belege.

const elements = {
  registrationPill: document.getElementById("registration-pill"),
  registrationCommand: document.getElementById("registration-command"),
  registrationWorkdir: document.getElementById("registration-workdir"),
  registrationEntries: document.getElementById("registration-entries"),
  registrationMessage: document.getElementById("registration-message"),
  toolsPill: document.getElementById("tools-pill"),
  toolsFacts: document.getElementById("tools-facts"),
  toolsList: document.getElementById("tools-list"),
  toolsMessage: document.getElementById("tools-message"),
};

// Was der Zustand bedeutet — ausführlicher als im Block der Startseite: hier
// steht der Platz dafür, und wer hierher kommt, will den Grund wissen.
const STATE_TEXTS = {
  ok: "eingetragen",
  "no-command": "kein installiertes k-playbook gefunden",
  "missing-file": "Datei nicht vorhanden",
  "missing-entry": "Eintrag fehlt",
  outdated: "zeigt auf den abgelösten Wrapper",
  stale: "zeigt woandershin",
  unreadable: "kein lesbares JSON",
  "ambiguous-target": "zwei Konfigurationen",
};

const STATE_NOTES = {
  ok: "Der Assistent liest den Eintrag beim nächsten Start.",
  "no-command":
    "Auf diesem Rechner ließ sich kein installiertes k-playbook auflösen. Solange das so ist, wird nichts geschrieben: ein Eintrag, der auf nichts zeigt, ist schlechter als keiner. Erst den Bootstrap ausführen.",
  "missing-file": "Die Datei entsteht beim Einrichten neu.",
  "missing-entry": "Die Datei bleibt erhalten, es kommt nur der eigene Eintrag hinzu.",
  outdated:
    "Der Eintrag stammt aus dem abgelösten Wrapper-Modell. Das ist der eine Fall, den k-playbook von sich aus richtigstellt — beim Clone-Update und beim nächsten Start, ohne Klick auf Einrichten.",
  stale:
    "Der Schlüssel k-playbook gehört k-playbook. Ein abweichender Wert ist kein Konflikt, sondern ein falscher Stand — er wird beim Einrichten überschrieben, aber nicht von selbst.",
  unreadable:
    "Die Datei wird nicht angefasst, damit keine Handarbeit verlorengeht. Erst reparieren, dann einrichten. Kommentare und Trailing Commas sind kein Grund dafür — die werden gelesen.",
  "ambiguous-target":
    "opencode.json und opencode.jsonc liegen nebeneinander. OpenCode führt beide zusammen; welcher Eintrag am Ende wirkt, ist von außen nicht zu sehen. Geschrieben wird nur opencode.json — eine der beiden Dateien gehört aufgelöst.",
};

// Die Zustände, bei denen jemand eingreifen muss statt nur einen Knopf zu
// drücken.
const STATE_WARNINGS = ["unreadable", "no-command", "ambiguous-target"];

// Muss vor den Ladefunktionen laufen: die blenden Blöcke ein, und das Menü
// zieht das nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();
// Das Lebenszeichen dieses Fensters: es hält den Dienst aus dem Leerlauf und
// merkt, wenn er weg ist.
startSession((message) => {
  elements.registrationMessage.textContent = message;
});
loadRegistration();
loadTools();

async function loadRegistration() {
  try {
    const response = await fetch("/api/mcp", { cache: "no-store" });
    renderRegistration(await response.json());
  } catch {
    elements.registrationMessage.textContent = "Zustand konnte nicht geladen werden.";
  }
}

// Der Selbsttest läuft erst beim Aufruf dieser Seite, nicht beim Laden der
// Startseite: dahinter steht ein Subprozess.
async function loadTools() {
  try {
    const response = await fetch("/api/mcp/tools", { cache: "no-store" });
    renderTools(await response.json());
  } catch {
    elements.toolsPill.className = "pill warn";
    elements.toolsPill.textContent = "Nicht messbar";
    elements.toolsMessage.textContent = "Der Selbsttest konnte nicht ausgeführt werden.";
  }
}

function renderRegistration(data) {
  elements.registrationEntries.replaceChildren();
  elements.registrationMessage.textContent = data.message || "";

  if (data.command) {
    elements.registrationCommand.textContent = data.command;
  }
  elements.registrationWorkdir.classList.toggle("hidden", !data.workdirMismatch);
  elements.registrationWorkdir.classList.toggle("warn", Boolean(data.workdirMismatch));

  const entries = data.entries || [];
  for (const entry of entries) {
    elements.registrationEntries.append(registrationBox(entry));
  }

  const environment = data.environment || {};
  if (!environment.installed || !environment.playbookPresent) {
    elements.registrationPill.className = "pill muted";
    elements.registrationPill.textContent = "Nicht anwendbar";
    return;
  }
  elements.registrationPill.className = data.ok ? "pill ok" : "pill warn";
  elements.registrationPill.textContent = data.ok ? "Eingerichtet" : "Fehlt";
}

// Ein Ziel: Datei und Assistent, der gemessene Zustand, der Befund im Klartext
// und was er bedeutet.
function registrationBox(entry) {
  const box = document.createElement("div");
  box.className = "setting";

  const head = document.createElement("div");
  head.className = "setting-head";

  const titles = document.createElement("div");
  const title = document.createElement("div");
  title.className = "setting-title";
  title.textContent = entry.path;
  const state = document.createElement("div");
  const warning = STATE_WARNINGS.includes(entry.state);
  state.className = warning ? "setting-state warn" : "setting-state";
  state.textContent = STATE_TEXTS[entry.state] || entry.state;
  titles.append(title, state);
  head.append(titles);
  box.append(head);

  const facts = document.createElement("dl");
  facts.className = "facts";
  if (entry.assistant) {
    addFact(facts, "Assistent", entry.assistant);
  }
  if (entry.schema) {
    addFact(facts, "Schlüssel in der Datei", entry.schema);
  }
  if (entry.detail) {
    addFact(facts, "Befund", entry.detail);
  }
  box.append(facts);

  const note = STATE_NOTES[entry.state];
  if (note) {
    const paragraph = document.createElement("p");
    paragraph.className = warning ? "setting-note warn" : "setting-note";
    paragraph.textContent = note;
    box.append(paragraph);
  }
  return box;
}

function renderTools(data) {
  elements.toolsFacts.replaceChildren();
  elements.toolsList.replaceChildren();
  elements.toolsMessage.textContent = data.message || "";

  if (data.command) {
    addFact(elements.toolsFacts, "Gestartet", data.command);
  }

  // Ein Fehlfall ist das Ergebnis dieser Messung, keine Störung: er beantwortet
  // die Frage, ob der Server läuft, mit nein — und nennt den Grund.
  if (!data.available) {
    elements.toolsPill.className = "pill warn";
    elements.toolsPill.textContent = "Antwortet nicht";
    return;
  }

  if (data.serverName) {
    addFact(elements.toolsFacts, "Server", `${data.serverName} ${data.serverVersion || ""}`.trim());
  }
  if (data.protocolVersion) {
    addFact(elements.toolsFacts, "Protokoll", data.protocolVersion);
  }

  const tools = data.tools || [];
  elements.toolsPill.className = "pill ok";
  elements.toolsPill.textContent = tools.length === 1 ? "1 Werkzeug" : `${tools.length} Werkzeuge`;

  for (const tool of tools) {
    elements.toolsList.append(toolBox(tool));
  }
  if (tools.length === 0) {
    elements.toolsMessage.textContent = "Der Server antwortet, bietet aber kein Werkzeug an.";
  }
}

// Ein Werkzeug: Name, Beschreibung und die Parameter, die es entgegennimmt.
function toolBox(tool) {
  const box = document.createElement("div");
  box.className = "setting";

  const title = document.createElement("div");
  title.className = "setting-title";
  title.textContent = tool.name;
  box.append(title);

  if (tool.description) {
    const description = document.createElement("p");
    description.className = "setting-note";
    description.textContent = tool.description;
    box.append(description);
  }

  const parameters = tool.parameters || [];
  if (parameters.length === 0) {
    return box;
  }

  const facts = document.createElement("dl");
  facts.className = "facts";
  for (const parameter of parameters) {
    const term = `${parameter.name}${parameter.required ? "" : " (optional)"}`;
    const detail = parameter.description
      ? `${parameter.type || "?"} — ${parameter.description}`
      : parameter.type || "";
    addFact(facts, term, detail);
  }
  box.append(facts);
  return box;
}

// Legt eine Zeile in einer Faktenliste an. Dieselbe Form wie auf der
// Startseite; die Seiten teilen sich kein Skript außer session.js.
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
