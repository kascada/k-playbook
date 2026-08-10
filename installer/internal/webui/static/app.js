"use strict";

// Haelt fest, ob das Backend noch antwortet. Sobald es weg ist, wird die
// Oberflaeche gesperrt und nicht weiter gepollt.
let serverAvailable = true;

const elements = {
  shutdown: document.getElementById("shutdown"),
  closed: document.getElementById("closed"),
  closedTitle: document.getElementById("closed-title"),
  closedMessage: document.getElementById("closed-message"),
  configCard: document.getElementById("config-card"),
  assistantCard: document.getElementById("assistant-card"),
  configPill: document.getElementById("config-pill"),
  configFacts: document.getElementById("config-facts"),
  configForm: document.getElementById("config-form"),
  configProjectDir: document.getElementById("config-project-dir"),
  configRepoRoot: document.getElementById("config-repo-root"),
  configMessage: document.getElementById("config-message"),
  configCreate: document.getElementById("config-create"),
  assistantPill: document.getElementById("assistant-pill"),
  assistantFacts: document.getElementById("assistant-facts"),
  assistantMessage: document.getElementById("assistant-message"),
  assistantApply: document.getElementById("assistant-apply"),
};

const HEALTH_INTERVAL = 1800;

const CONFIG_LABELS = { doneLabel: "Angelegt", todoLabel: "Anlegen" };

// Lesbare Beschreibung je Zustand aus dem Backend.
const STATE_LABELS = {
  ok: "eingerichtet",
  missing: "nicht vorhanden",
  stale: "zeigt woandershin",
  "own-directory": "eigenes Verzeichnis, unvollstaendig",
  blocked: "blockiert",
  "no-source": "Quelle fehlt",
};

elements.shutdown.addEventListener("click", shutdown);
elements.configCreate.addEventListener("click", createConfig);
elements.assistantApply.addEventListener("click", applyAssistant);
window.addEventListener("pagehide", notifyClientGone);
startHealthChecks();
// Der Assistenten-Block folgt erst, wenn die Konfiguration steht; loadConfig
// blendet ihn dann ein und laedt ihn nach.
loadConfig();

// Legt eine Zeile in einer Faktenliste an.
function addFact(list, term, detail) {
  const row = document.createElement("div");
  const dt = document.createElement("dt");
  dt.textContent = term;
  const dd = document.createElement("dd");
  dd.textContent = detail;
  row.append(dt, dd);
  list.append(row);
}

async function loadConfig() {
  setBlockState(elements.configPill, elements.configCreate, "busy");
  try {
    const response = await fetch("/api/config", { cache: "no-store" });
    renderConfig(await response.json());
  } catch {
    elements.configMessage.textContent = "Status konnte nicht geladen werden.";
  }
}

async function createConfig() {
  setBlockState(elements.configPill, elements.configCreate, "busy");
  const body = JSON.stringify({
    projectDir: elements.configProjectDir.value.trim(),
    repoRoot: elements.configRepoRoot.value.trim(),
  });

  try {
    const response = await fetch("/api/config", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body,
    });
    const data = await response.json();
    if (data.installed) {
      // Die Kopfzeile wird serverseitig gefuellt und muss den neuen Ort zeigen.
      window.location.reload();
      return;
    }
    renderConfig(data);
  } catch {
    elements.configMessage.textContent = "Anlegen fehlgeschlagen.";
    setBlockState(elements.configPill, elements.configCreate, "todo", CONFIG_LABELS);
  }
}

function renderConfig(data) {
  elements.configFacts.replaceChildren();
  elements.configMessage.textContent = data.message || "";

  // Steht die Konfiguration, ist dieser Schritt erledigt und verschwindet. Wo
  // alles liegt, zeigt die Kopfzeile.
  if (data.installed) {
    elements.configCard.classList.add("hidden");
    elements.assistantCard.classList.remove("hidden");
    loadAssistant();
    return;
  }

  // Solange sie fehlt, ist dies der einzige Schritt, der zur Wahl steht.
  elements.configCard.classList.remove("hidden");
  elements.assistantCard.classList.add("hidden");

  const suggestion = data.suggestion || {};
  elements.configForm.classList.remove("hidden");
  elements.configProjectDir.value = suggestion.projectDir || "";
  elements.configRepoRoot.value = suggestion.repoRoot || ".";

  const candidates = suggestion.repoCandidates || [];
  if (candidates.length > 1) {
    addFact(elements.configFacts, "Gefundene Repositories", candidates.join(", "));
  }
  if (!suggestion.derived) {
    addFact(elements.configFacts, "Hinweis", "Ort nicht aus dem Programmpfad ableitbar, bitte pruefen");
  }

  setBlockState(elements.configPill, elements.configCreate, "todo", CONFIG_LABELS);
}

// Setzt Pill und Button eines Blocks aus einem Zustand. Einheitliche Regel fuer
// alle Bloecke: der Button kann immer ausgeloest werden und tut immer dasselbe,
// nur Beschriftung und Hervorhebung wechseln.
function setBlockState(pill, button, state, labels = {}) {
  const { doneLabel = "Aktualisieren", todoLabel = "Einrichten" } = labels;

  if (state === "busy") {
    pill.className = "pill muted";
    pill.textContent = "Pruefen...";
    button.disabled = true;
    return;
  }

  if (state === "blocked") {
    pill.className = "pill muted";
    pill.textContent = "Nicht anwendbar";
    button.disabled = true;
    button.className = "secondary";
    button.textContent = doneLabel;
    return;
  }

  const ok = state === "ok";
  pill.className = ok ? "pill ok" : "pill warn";
  pill.textContent = ok ? "Eingerichtet" : "Fehlt";
  button.disabled = false;
  button.className = ok ? "secondary" : "primary attention-highlight";
  button.textContent = ok ? doneLabel : todoLabel;
}

async function loadAssistant() {
  setBlockState(elements.assistantPill, elements.assistantApply, "busy");
  try {
    const response = await fetch("/api/assistant", { cache: "no-store" });
    renderAssistant(await response.json());
  } catch {
    elements.assistantMessage.textContent = "Status konnte nicht geladen werden.";
  }
}

async function applyAssistant() {
  setBlockState(elements.assistantPill, elements.assistantApply, "busy");
  try {
    const response = await fetch("/api/assistant", { method: "POST" });
    renderAssistant(await response.json());
  } catch {
    elements.assistantMessage.textContent = "Einrichten fehlgeschlagen.";
    setBlockState(elements.assistantPill, elements.assistantApply, "todo");
  }
}

function renderAssistant(data) {
  elements.assistantFacts.replaceChildren();

  for (const entry of data.entries || []) {
    const label = STATE_LABELS[entry.state] || entry.state;
    addFact(elements.assistantFacts, entry.path, entry.detail ? `${label} (${entry.detail})` : label);
  }

  elements.assistantMessage.textContent = data.message || "";

  // Ohne Installation gibt es nichts einzurichten, der Button bleibt grau.
  const installed = data.environment && data.environment.installed;
  const state = !installed ? "blocked" : data.ok ? "ok" : "todo";
  setBlockState(elements.assistantPill, elements.assistantApply, state);
}

async function shutdown() {
  elements.shutdown.disabled = true;
  try {
    await fetch("/api/shutdown", { method: "POST" });
  } catch {
    // Das Backend darf die Antwort schuldig bleiben, wenn es sofort zumacht.
  }
  showClosed();
}

function startHealthChecks() {
  window.setInterval(checkHealth, HEALTH_INTERVAL);
}

// Erkennt ein weggefallenes Backend: schlaegt der Aufruf fehl, ist der Server
// beendet worden.
async function checkHealth() {
  if (!serverAvailable) {
    return;
  }

  try {
    await fetch("/api/health", { cache: "no-store" });
  } catch {
    showClosed("Verbindung zu k-playbook verloren.");
  }
}

// Meldet dem Backend, dass dieses Fenster verschwindet. sendBeacon ueberlebt
// das Entladen der Seite, fetch nicht zuverlaessig.
function notifyClientGone() {
  if (!serverAvailable) {
    return;
  }

  if (navigator.sendBeacon) {
    navigator.sendBeacon("/api/client-gone");
    return;
  }

  fetch("/api/client-gone", { method: "POST", keepalive: true }).catch(() => {});
}

function showClosed(message = "") {
  serverAvailable = false;
  elements.closedTitle.textContent = "Dieses Browserfenster kann jetzt geschlossen werden.";
  elements.closedMessage.textContent = message;
  elements.closedMessage.classList.toggle("hidden", !message);
  elements.closed.classList.remove("hidden");
}
