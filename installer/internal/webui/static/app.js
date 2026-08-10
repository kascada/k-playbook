"use strict";

// Haelt fest, ob das Backend noch antwortet. Sobald es weg ist, wird die
// Oberflaeche gesperrt und nicht weiter gepollt.
let serverAvailable = true;

const elements = {
  shutdown: document.getElementById("shutdown"),
  closed: document.getElementById("closed"),
  closedTitle: document.getElementById("closed-title"),
  closedMessage: document.getElementById("closed-message"),
  assistantPill: document.getElementById("assistant-pill"),
  assistantFacts: document.getElementById("assistant-facts"),
  assistantMessage: document.getElementById("assistant-message"),
  assistantApply: document.getElementById("assistant-apply"),
};

const HEALTH_INTERVAL = 1800;

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
elements.assistantApply.addEventListener("click", applyAssistant);
window.addEventListener("pagehide", notifyClientGone);
startHealthChecks();
loadAssistant();

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
    const row = document.createElement("div");
    const term = document.createElement("dt");
    term.textContent = entry.path;
    const detail = document.createElement("dd");
    const label = STATE_LABELS[entry.state] || entry.state;
    detail.textContent = entry.detail ? `${label} (${entry.detail})` : label;
    row.append(term, detail);
    elements.assistantFacts.append(row);
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
