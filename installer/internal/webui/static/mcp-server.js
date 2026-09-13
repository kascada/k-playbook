// Detailseite eines MCP-Servers bei einem Assistenten: die Konfiguration aus
// der Projektdatei und, auf Knopfdruck, was der Server tatsächlich anbietet.
//
// Beim Laden wird nur die Konfiguration gelesen. Die Messung startet ein
// Kommando und läuft deshalb ausschließlich über POST …/probe, ausgelöst
// durch den Knopf — nie durch das Öffnen der Seite.

const elements = {
  configPill: document.getElementById("config-pill"),
  configFacts: document.getElementById("config-facts"),
  configNote: document.getElementById("config-note"),
  configMessage: document.getElementById("config-message"),
  ownLink: document.getElementById("own-link"),
  serverPill: document.getElementById("server-pill"),
  serverFacts: document.getElementById("server-facts"),
  serverCapabilities: document.getElementById("server-capabilities"),
  serverMessage: document.getElementById("server-message"),
  probe: document.getElementById("probe"),
  toolsCard: document.getElementById("tools-card"),
  toolsPill: document.getElementById("tools-pill"),
  toolsList: document.getElementById("tools-list"),
  toolsMessage: document.getElementById("tools-message"),
  promptsCard: document.getElementById("prompts-card"),
  promptsPill: document.getElementById("prompts-pill"),
  promptsList: document.getElementById("prompts-list"),
  resourcesCard: document.getElementById("resources-card"),
  resourcesPill: document.getElementById("resources-pill"),
  resourcesList: document.getElementById("resources-list"),
};

const TRANSPORT_TEXTS = {
  local: "lokal (Kommando über stdio)",
  remote: "remote (HTTP/SSE hinter einer URL)",
  unknown: "unbekannte Form",
};

// Assistent und Name kommen aus dem Pfad: /mcp-servers/<assistant>/<name>.
const [assistantId, serverName] = window.location.pathname
  .split("/")
  .slice(2, 4)
  .map((segment) => decodeURIComponent(segment));
const apiPath = `/api/mcp-servers/${encodeURIComponent(assistantId)}/${encodeURIComponent(serverName)}`;
// ?file= wählt unter gleichnamigen Einträgen den aus einer bestimmten Datei —
// bei OpenCode, wenn opencode.json und opencode.jsonc nebeneinander liegen.
// GET und POST bekommen ihn gleichermaßen, damit die Messung den Eintrag
// trifft, dessen Konfiguration die Seite zeigt.
const fileParam = new URLSearchParams(window.location.search).get("file");
const apiQuery = fileParam ? `?file=${encodeURIComponent(fileParam)}` : "";

// Muss vor den Ladefunktionen laufen: die blenden Blöcke ein, und das Menü
// zieht das nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();
// Das Lebenszeichen dieses Fensters: es hält den Dienst aus dem Leerlauf und
// merkt, wenn er weg ist.
startSession((message) => {
  elements.configMessage.textContent = message;
});
elements.probe.addEventListener("click", probe);
loadConfig();

async function loadConfig() {
  try {
    const response = await fetch(`${apiPath}${apiQuery}`, { cache: "no-store" });
    if (!response.ok) {
      elements.configPill.className = "pill warn";
      elements.configPill.textContent = "Nicht gefunden";
      elements.configMessage.textContent = "Diesen Server gibt es in den Projektdateien nicht.";
      return;
    }
    renderConfig(await response.json());
  } catch {
    elements.configPill.className = "pill warn";
    elements.configPill.textContent = "Nicht lesbar";
    elements.configMessage.textContent = "Die Konfiguration konnte nicht geladen werden.";
  }
}

function renderConfig(data) {
  const entry = data.entry || {};
  elements.configFacts.replaceChildren();

  addFact(elements.configFacts, "Assistent", entry.assistant || assistantId);
  addFact(elements.configFacts, "Datei", entry.file || "");
  addFact(elements.configFacts, "Transport", TRANSPORT_TEXTS[entry.transport] || entry.transport || "");
  if (entry.transport === "local") {
    addFact(elements.configFacts, "Befehl", [entry.command, ...(entry.args || [])].join(" "));
    if (data.resolvedCommand) {
      addFact(elements.configFacts, "Aufgelöst", data.resolvedCommand);
    }
  } else if (entry.transport === "remote") {
    addFact(elements.configFacts, "URL", entry.url || "");
  }
  addFact(elements.configFacts, "Aktiv", entry.enabled ? "ja" : "nein (deaktiviert)");
  // Ist die Pflichtliste nicht lesbar, ist required null: dann weiß die Seite
  // es nicht und sagt das, statt „nein" zu behaupten.
  if (data.requiredError) {
    addFact(elements.configFacts, "Pflicht", `unbekannt — ${data.requiredError}`).className = "missing";
  } else {
    addFact(elements.configFacts, "Pflicht", entry.required ? "ja (tools.mcp.required)" : "nein");
  }
  const envKeys = entry.envKeys || [];
  addFact(elements.configFacts, "Umgebung", envKeys.length > 0 ? envKeys.join(", ") : "keine Einträge");

  elements.configNote.classList.toggle("hidden", !data.note);
  elements.configNote.textContent = data.note || "";
  elements.ownLink.classList.toggle("hidden", !entry.own);

  if (entry.enabled) {
    elements.configPill.className = "pill ok";
    elements.configPill.textContent = entry.own ? "Eigener Server" : "Eingetragen";
  } else {
    elements.configPill.className = "pill muted";
    elements.configPill.textContent = "Deaktiviert";
  }

  // Der Knopf steht für jeden lokalen Eintrag — aktiviert, deaktiviert und
  // den eigenen Server gleichermaßen. Remote und unknown lassen sich nicht
  // messen; dort sagt die Karte, warum.
  elements.probe.disabled = !data.probeable;
  if (!data.probeable) {
    elements.serverMessage.textContent = data.note || "";
  }
}

// Die Messung: ein POST, sonst nichts. Während sie läuft, ist der Knopf
// gesperrt; ein zweiter Klick startet den Server nicht ein zweites Mal.
async function probe() {
  elements.probe.disabled = true;
  elements.serverPill.className = "pill muted";
  elements.serverPill.textContent = "Messen...";
  elements.serverMessage.textContent = "";

  try {
    const response = await fetch(`${apiPath}/probe${apiQuery}`, { method: "POST", cache: "no-store" });
    if (!response.ok) {
      throw new Error(`Status ${response.status}`);
    }
    renderProbe(await response.json());
  } catch {
    elements.serverPill.className = "pill warn";
    elements.serverPill.textContent = "Nicht messbar";
    elements.serverMessage.textContent = "Die Messung konnte nicht ausgeführt werden.";
  } finally {
    elements.probe.textContent = "Erneut messen";
    elements.probe.disabled = false;
  }
}

function renderProbe(data) {
  elements.serverFacts.replaceChildren();
  elements.serverCapabilities.replaceChildren();
  // Bei einer Antwort ist die Meldung ein Hinweis — etwa, dass prompts/list
  // scheiterte. Er steht unter den Serverdaten; die Werkzeuge erscheinen
  // trotzdem, denn initialize und tools/list sind angekommen.
  elements.serverMessage.textContent = data.message || "";

  // started kommt aus der Messung selbst: false, wenn kein Prozess lief —
  // Datei fehlt, Start scheiterte, remote oder unbekannte Form.
  if (data.command) {
    addFact(
      elements.serverFacts,
      data.started ? "Gestartet" : "Befehl",
      data.started ? data.command : `${data.command} (nicht gestartet)`,
    );
  }

  // Ein Fehlfall ist das Ergebnis dieser Messung, keine Störung: er
  // beantwortet die Frage, ob der Server läuft, mit nein — und nennt den Grund.
  if (!data.available) {
    elements.serverPill.className = data.started ? "pill warn" : "pill muted";
    elements.serverPill.textContent = data.started ? "Antwortet nicht" : "Nicht gemessen";
    hideCards();
    return;
  }

  if (data.serverName) {
    addFact(elements.serverFacts, "Server", `${data.serverName} ${data.serverVersion || ""}`.trim());
  }
  if (data.protocolVersion) {
    addFact(elements.serverFacts, "Protokoll", data.protocolVersion);
  }
  for (const capability of data.capabilities || []) {
    const pill = document.createElement("span");
    pill.className = "pill ok";
    pill.textContent = capability;
    elements.serverCapabilities.append(pill);
  }
  elements.serverPill.className = "pill ok";
  elements.serverPill.textContent = "Antwortet";

  renderTools(data.tools || []);
  renderPrompts(data.prompts);
  renderResources(data.resources);
}

function hideCards() {
  elements.toolsCard.classList.add("hidden");
  elements.promptsCard.classList.add("hidden");
  elements.resourcesCard.classList.add("hidden");
}

function renderTools(tools) {
  elements.toolsList.replaceChildren();
  elements.toolsCard.classList.remove("hidden");
  elements.toolsPill.className = "pill ok";
  elements.toolsPill.textContent = tools.length === 1 ? "1 Werkzeug" : `${tools.length} Werkzeuge`;
  elements.toolsMessage.textContent = tools.length === 0 ? "Der Server antwortet, bietet aber kein Werkzeug an." : "";
  for (const tool of tools) {
    elements.toolsList.append(toolBox(tool));
  }
}

// Vorlagen und Ressourcen erscheinen nur, wenn der Server die Fähigkeit
// gemeldet hat: ohne Meldung wird nicht gefragt, und die Karte bleibt weg.
function renderPrompts(prompts) {
  elements.promptsList.replaceChildren();
  if (!prompts) {
    elements.promptsCard.classList.add("hidden");
    return;
  }
  elements.promptsCard.classList.remove("hidden");
  elements.promptsPill.className = "pill ok";
  elements.promptsPill.textContent = prompts.length === 1 ? "1 Vorlage" : `${prompts.length} Vorlagen`;
  for (const prompt of prompts) {
    const box = document.createElement("div");
    box.className = "setting";
    const title = document.createElement("div");
    title.className = "setting-title";
    title.textContent = prompt.name;
    box.append(title);
    if (prompt.description) {
      const description = document.createElement("p");
      description.className = "setting-note";
      description.textContent = prompt.description;
      box.append(description);
    }
    const args = prompt.arguments || [];
    if (args.length > 0) {
      const facts = document.createElement("dl");
      facts.className = "facts";
      for (const argument of args) {
        addFact(facts, `${argument.name}${argument.required ? "" : " (optional)"}`, argument.description || "");
      }
      box.append(facts);
    }
    elements.promptsList.append(box);
  }
}

function renderResources(resources) {
  elements.resourcesList.replaceChildren();
  if (!resources) {
    elements.resourcesCard.classList.add("hidden");
    return;
  }
  elements.resourcesCard.classList.remove("hidden");
  elements.resourcesPill.className = "pill ok";
  elements.resourcesPill.textContent = resources.length === 1 ? "1 Ressource" : `${resources.length} Ressourcen`;
  for (const resource of resources) {
    const box = document.createElement("div");
    box.className = "setting";
    const title = document.createElement("div");
    title.className = "setting-title";
    title.textContent = resource.name || resource.uri;
    box.append(title);
    const facts = document.createElement("dl");
    facts.className = "facts";
    addFact(facts, "URI", resource.uri);
    if (resource.mimeType) {
      addFact(facts, "Typ", resource.mimeType);
    }
    if (resource.description) {
      addFact(facts, "Beschreibung", resource.description);
    }
    box.append(facts);
    elements.resourcesList.append(box);
  }
}

// Ein Werkzeug: Name, Beschreibung und die Parameter, die es entgegennimmt.
// Dieselbe Form wie auf /mcp.
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
