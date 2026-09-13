// Seite "MCP-Server": alle Server, die die drei Assistenten in den
// Projektdateien kennen, die Pflichtliste aus K-PLAYBOOK.yaml und die
// gelesenen Dateien.
//
// Gelesen wird nur. Gemessen wird auf der Detailseite je Server, und auch
// dort erst auf Knopfdruck — diese Seite startet nichts.

const elements = {
  serversPill: document.getElementById("servers-pill"),
  serversRows: document.getElementById("servers-rows"),
  serversMessage: document.getElementById("servers-message"),
  requiredPill: document.getElementById("required-pill"),
  requiredFacts: document.getElementById("required-facts"),
  requiredGaps: document.getElementById("required-gaps"),
  requiredSnippet: document.getElementById("required-snippet"),
  requiredMessage: document.getElementById("required-message"),
  filesPill: document.getElementById("files-pill"),
  filesList: document.getElementById("files-list"),
  filesMessage: document.getElementById("files-message"),
};

// Die Spalten der Matrix, in dieser Reihenfolge. Die IDs sind die
// Pfadsegmente der Detailseite.
const ASSISTANTS = [
  { id: "claude-code", name: "Claude Code" },
  { id: "opencode", name: "OpenCode" },
  { id: "cursor", name: "Cursor" },
];

// Der Schnipsel für ein Projekt, das noch keine Pflichtliste hat.
const REQUIRED_SNIPPET = "tools:\n  mcp:\n    required:\n      - k-playbook\n";

// Muss vor den Ladefunktionen laufen: die blenden Blöcke ein, und das Menü
// zieht das nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();
// Das Lebenszeichen dieses Fensters: es hält den Dienst aus dem Leerlauf und
// merkt, wenn er weg ist.
startSession((message) => {
  elements.serversMessage.textContent = message;
});
loadServers();

async function loadServers() {
  try {
    const response = await fetch("/api/mcp-servers", { cache: "no-store" });
    render(await response.json());
  } catch {
    for (const pill of [elements.serversPill, elements.requiredPill, elements.filesPill]) {
      pill.className = "pill warn";
      pill.textContent = "Nicht lesbar";
    }
    elements.serversMessage.textContent = "Die Übersicht konnte nicht geladen werden.";
  }
}

function render(data) {
  const environment = data.environment || {};
  if (!environment.installed) {
    for (const pill of [elements.serversPill, elements.requiredPill, elements.filesPill]) {
      pill.className = "pill muted";
      pill.textContent = "Nicht anwendbar";
    }
    elements.serversMessage.textContent = data.message || "";
    return;
  }

  renderMatrix(data);
  renderRequired(data);
  renderFiles(data);
}

// Die Matrix: eine Zeile je Servername, eine Spalte je Assistent. Ein Name
// kann bei OpenCode zweimal stehen, wenn beide Endungen nebeneinander liegen;
// dann zeigt die Zelle beide Einträge.
function renderMatrix(data) {
  elements.serversRows.replaceChildren();
  elements.serversMessage.textContent = "";

  const servers = data.servers || [];
  const required = new Set(data.required || []);
  const names = [...new Set([...servers.map((server) => server.name), ...required])].sort((a, b) =>
    a.localeCompare(b, "de"),
  );

  for (const name of names) {
    const row = document.createElement("tr");

    const nameCell = document.createElement("th");
    nameCell.scope = "row";
    nameCell.textContent = name;
    row.append(nameCell);

    const requiredCell = document.createElement("td");
    requiredCell.textContent = required.has(name) ? "Pflicht" : "";
    if (!required.has(name)) {
      requiredCell.className = "muted";
    }
    row.append(requiredCell);

    for (const assistant of ASSISTANTS) {
      row.append(matrixCell(name, assistant, servers, required.has(name)));
    }
    elements.serversRows.append(row);
  }

  if (names.length === 0) {
    elements.serversMessage.textContent = "In keiner der Projektdateien ist ein MCP-Server eingetragen.";
  } else if (data.message && !data.requiredError) {
    elements.serversMessage.textContent = data.message;
  }

  // Ist die Pflichtliste nicht lesbar, fehlen der Matrix die Pflichtmarken.
  // Die Pille sagt das; den Fehlertext trägt die Pflichtkarte.
  if (data.requiredError) {
    elements.serversPill.className = "pill warn";
    elements.serversPill.textContent = "Pflichtliste nicht lesbar";
    return;
  }

  const missing = data.missing || [];
  const broken = (data.files || []).filter((file) => file.error).length;
  if (missing.length > 0 || broken > 0) {
    elements.serversPill.className = "pill warn";
    elements.serversPill.textContent = missing.length > 0 ? "Pflichtserver fehlt" : "Datei nicht lesbar";
    return;
  }
  elements.serversPill.className = "pill ok";
  elements.serversPill.textContent = servers.length === 1 ? "1 Eintrag" : `${servers.length} Einträge`;
}

// Eine Zelle: der Eintrag dieses Servers bei diesem Assistenten, verlinkt auf
// die Detailseite. Fehlt er, steht ein Strich — oder „fehlt", wenn der Name
// Pflicht ist.
function matrixCell(name, assistant, servers, isRequired) {
  const cell = document.createElement("td");
  const entries = servers.filter((server) => server.name === name && server.assistantId === assistant.id);

  if (entries.length === 0) {
    if (isRequired) {
      cell.className = "warn";
      cell.textContent = "fehlt";
    } else {
      cell.className = "muted";
      cell.textContent = "—";
    }
    return cell;
  }

  for (const [index, entry] of entries.entries()) {
    if (index > 0) {
      cell.append(document.createElement("br"));
    }
    const link = document.createElement("a");
    // Die Datei kommt nur bei mehreren gleichnamigen Einträgen mit: ohne sie
    // träfe jeder Link den ersten, und der zweite wäre unerreichbar.
    link.href = detailPath(assistant.id, name, entries.length > 1 ? entry.file : "");
    link.textContent = describeEntry(entry);
    cell.append(link);
    if (!entry.enabled) {
      const note = document.createElement("span");
      note.className = "muted";
      note.textContent = " deaktiviert";
      cell.append(note);
    }
    if (entries.length > 1) {
      const file = document.createElement("span");
      file.className = "muted";
      file.textContent = ` (${entry.file})`;
      cell.append(file);
    }
  }
  if (entries.every((entry) => !entry.enabled)) {
    cell.className = "muted";
  }
  return cell;
}

// Der Eintrag in einer Zeile: Transport und das, was ihn ausmacht.
function describeEntry(entry) {
  switch (entry.transport) {
    case "local":
      return `lokal: ${[entry.command, ...(entry.args || [])].join(" ")}`;
    case "remote":
      return `remote: ${entry.url}`;
    default:
      return "unbekannte Form";
  }
}

function detailPath(assistantId, name, file) {
  const path = `/mcp-servers/${encodeURIComponent(assistantId)}/${encodeURIComponent(name)}`;
  return file ? `${path}?file=${encodeURIComponent(file)}` : path;
}

// Die Pflichtliste: was verlangt ist, wo es fehlt — oder der Schnipsel, mit
// dem ein Projekt anfängt.
function renderRequired(data) {
  elements.requiredFacts.replaceChildren();
  elements.requiredGaps.replaceChildren();
  elements.requiredMessage.textContent = "";
  elements.requiredSnippet.classList.add("hidden");

  if (data.requiredError) {
    // Ein Lesefehler an der Pflichtliste ist ihr Befund, nicht der der Matrix.
    // Erkannt am eigenen Feld: der Block steht ja da, requiredConfigured ist
    // dabei true.
    elements.requiredPill.className = "pill warn";
    elements.requiredPill.textContent = "Nicht lesbar";
    elements.requiredMessage.textContent = data.message || data.requiredError;
    return;
  }

  if (!data.requiredConfigured) {
    elements.requiredPill.className = "pill muted";
    elements.requiredPill.textContent = "Nicht festgelegt";
    elements.requiredMessage.textContent =
      "Kein Block tools.mcp.required in K-PLAYBOOK.yaml. So sähe einer aus:";
    elements.requiredSnippet.textContent = REQUIRED_SNIPPET;
    elements.requiredSnippet.classList.remove("hidden");
    return;
  }

  const required = data.required || [];
  addFact(elements.requiredFacts, "Verlangt", required.length > 0 ? required.join(", ") : "nichts (leere Liste)");

  const missing = data.missing || [];
  for (const assistant of ASSISTANTS) {
    const gaps = missing.filter((gap) => gap.assistantId === assistant.id).map((gap) => gap.name);
    const row = addFact(elements.requiredFacts, assistant.name, gaps.length > 0 ? `fehlt: ${gaps.join(", ")}` : "vollständig");
    if (gaps.length > 0) {
      row.className = "missing";
    }
  }

  elements.requiredPill.className = missing.length > 0 ? "pill warn" : "pill ok";
  elements.requiredPill.textContent = missing.length > 0 ? `${missing.length} fehlt` : "Vollständig";
}

// Die Dateien: Pfad, Assistent, Zustand, Zahl der Einträge und der Hinweis auf
// die Doppelung bei OpenCode.
function renderFiles(data) {
  elements.filesList.replaceChildren();
  elements.filesMessage.textContent = "";

  const files = data.files || [];
  let broken = 0;
  for (const file of files) {
    if (file.error) {
      broken++;
    }
    elements.filesList.append(fileBox(file));
  }

  elements.filesPill.className = broken > 0 ? "pill warn" : "pill ok";
  elements.filesPill.textContent = broken > 0 ? "Nicht lesbar" : `${files.length} Dateien`;
}

function fileBox(file) {
  const box = document.createElement("div");
  box.className = "setting";

  const head = document.createElement("div");
  head.className = "setting-head";
  const titles = document.createElement("div");
  const title = document.createElement("div");
  title.className = "setting-title";
  title.textContent = file.path;
  const state = document.createElement("div");
  state.className = file.error ? "setting-state warn" : "setting-state";
  state.textContent = file.error ? "nicht lesbar" : file.exists ? "vorhanden" : "nicht vorhanden";
  titles.append(title, state);
  head.append(titles);
  box.append(head);

  const facts = document.createElement("dl");
  facts.className = "facts";
  addFact(facts, "Assistent", file.assistant);
  addFact(facts, "Schlüssel in der Datei", file.schema);
  if (file.exists && !file.error) {
    addFact(facts, "Server", String(file.count));
  }
  if (file.error) {
    addFact(facts, "Befund", file.error);
  }
  box.append(facts);

  if (file.ambiguous) {
    const note = document.createElement("p");
    note.className = "setting-note warn";
    note.textContent =
      "opencode.json und opencode.jsonc liegen nebeneinander. OpenCode führt beide zusammen; welcher Eintrag am Ende wirkt, ist von außen nicht zu sehen. Beide werden gelesen.";
    box.append(note);
  }
  return box;
}

// Legt eine Zeile in einer Faktenliste an. Dieselbe Form wie auf den anderen
// Seiten; die Seiten teilen sich kein Skript außer session.js und nav.js.
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
