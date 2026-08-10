"use strict";

// Haelt fest, ob das Backend noch antwortet. Sobald es weg ist, wird die
// Oberflaeche gesperrt und nicht weiter gepollt.
let serverAvailable = true;

const elements = {
  shutdown: document.getElementById("shutdown"),
  update: document.getElementById("update"),
  closed: document.getElementById("closed"),
  closedTitle: document.getElementById("closed-title"),
  closedMessage: document.getElementById("closed-message"),
  configCard: document.getElementById("config-card"),
  localCard: document.getElementById("local-card"),
  assistantCard: document.getElementById("assistant-card"),
  remediationCard: document.getElementById("remediation-card"),
  remediationPill: document.getElementById("remediation-pill"),
  remediationChoices: document.getElementById("remediation-choices"),
  remediationMessage: document.getElementById("remediation-message"),
  toolsCard: document.getElementById("tools-card"),
  toolsPill: document.getElementById("tools-pill"),
  toolsFacts: document.getElementById("tools-facts"),
  toolsMessage: document.getElementById("tools-message"),
  toolsCommand: document.getElementById("tools-command"),
  toolsCommandText: document.getElementById("tools-command-text"),
  localPill: document.getElementById("local-pill"),
  localFacts: document.getElementById("local-facts"),
  localMessage: document.getElementById("local-message"),
  localCreate: document.getElementById("local-create"),
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
elements.update.addEventListener("click", onUpdateClick);
elements.configCreate.addEventListener("click", createConfig);
elements.localCreate.addEventListener("click", createLocal);
elements.assistantApply.addEventListener("click", applyAssistant);
window.addEventListener("pagehide", notifyClientGone);
startHealthChecks();
// Der Assistenten-Block folgt erst, wenn die Konfiguration steht; loadConfig
// blendet ihn dann ein und laedt ihn nach.
loadConfig();
// Die Update-Pruefung braucht das Netz. Sie laeuft nebenher, damit die Seite
// nicht auf einen langsamen Remote wartet.
checkUpdate();

// updateAvailable steuert, was ein Klick auf den Button tut: pruefen oder
// tatsaechlich aktualisieren.
let updateAvailable = false;

async function checkUpdate() {
  elements.update.disabled = true;
  elements.update.textContent = "Pruefe...";
  try {
    const response = await fetch("/api/update", { cache: "no-store" });
    renderUpdate(await response.json());
  } catch {
    resetUpdateButton("Update pruefen");
  }
}

async function onUpdateClick() {
  if (!updateAvailable) {
    await checkUpdate();
    return;
  }

  elements.update.disabled = true;
  elements.update.textContent = "Aktualisiere...";
  try {
    const response = await fetch("/api/update", { method: "POST" });
    const data = await response.json();
    renderUpdate(data);
    if (data.restartRequired) {
      showClosed(
        "Das Programm wurde aktualisiert. Dieses Fenster schliessen und " +
          "bin/k-playbook neu starten, um die neue Version zu verwenden."
      );
    }
  } catch {
    resetUpdateButton("Update pruefen");
  }
}

function renderUpdate(data) {
  updateAvailable = Boolean(data.available);

  if (updateAvailable) {
    // Hervorgehoben, solange etwas anliegt.
    elements.update.className = "primary attention-highlight";
    elements.update.textContent = "Update verfuegbar";
    elements.update.title = `${data.local} -> ${data.remote} (${data.branch})`;
    elements.update.disabled = false;
    return;
  }

  // Ohne Meldung ist der Stand geprueft und gleich; mit Meldung konnte nicht
  // geprueft werden, dann bleibt es bei der Aufforderung.
  resetUpdateButton(data.message ? "Update pruefen" : "Version ist aktuell");
  elements.update.title = data.message || `Stand ${data.local || "unbekannt"} (${data.branch || "?"})`;
}

function resetUpdateButton(label) {
  updateAvailable = false;
  elements.update.className = "secondary";
  elements.update.textContent = label;
  elements.update.disabled = false;
}

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
    elements.localCard.classList.remove("hidden");
    elements.assistantCard.classList.remove("hidden");
    elements.remediationCard.classList.remove("hidden");
    elements.toolsCard.classList.remove("hidden");
    loadLocal();
    loadAssistant();
    loadRemediation();
    loadTools();
    return;
  }

  // Solange sie fehlt, ist dies der einzige Schritt, der zur Wahl steht.
  elements.configCard.classList.remove("hidden");
  elements.localCard.classList.add("hidden");
  elements.assistantCard.classList.add("hidden");
  elements.remediationCard.classList.add("hidden");
  elements.toolsCard.classList.add("hidden");

  const suggestion = data.suggestion || {};
  elements.configForm.classList.remove("hidden");
  elements.configProjectDir.value = suggestion.projectDir || "";
  elements.configRepoRoot.value = suggestion.repoRoot || ".";

  // Kommt mehr als ein Ort in Frage, muss der Nutzer sehen, welche das sind —
  // der Vorschlag steht bereits im Feld.
  const projectCandidates = suggestion.projectCandidates || [];
  if (projectCandidates.length > 1) {
    addFact(elements.configFacts, "Weitere moegliche Orte", projectCandidates.slice(1).join(", "));
  }

  const repoCandidates = suggestion.repoCandidates || [];
  if (repoCandidates.length > 1) {
    addFact(elements.configFacts, "Gefundene Repositories", repoCandidates.join(", "));
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

const LOCAL_LABELS = { doneLabel: "Ergaenzen", todoLabel: "Anlegen" };

async function loadLocal() {
  setBlockState(elements.localPill, elements.localCreate, "busy");
  try {
    const response = await fetch("/api/local", { cache: "no-store" });
    renderLocal(await response.json());
  } catch {
    elements.localMessage.textContent = "Status konnte nicht geladen werden.";
  }
}

async function createLocal() {
  setBlockState(elements.localPill, elements.localCreate, "busy");
  try {
    const response = await fetch("/api/local", { method: "POST" });
    renderLocal(await response.json());
  } catch {
    elements.localMessage.textContent = "Anlegen fehlgeschlagen.";
    setBlockState(elements.localPill, elements.localCreate, "todo", LOCAL_LABELS);
  }
}

function renderLocal(data) {
  elements.localFacts.replaceChildren();
  elements.localMessage.textContent = data.message || "";

  const missing = (data.entries || []).filter((entry) => !entry.present);
  if (data.dir) {
    addFact(elements.localFacts, "Verzeichnis", data.dir);
  }
  if (missing.length > 0) {
    addFact(elements.localFacts, "Fehlt", missing.map((entry) => entry.path).join(", "));
  }

  setBlockState(elements.localPill, elements.localCreate, data.ok ? "ok" : "todo", LOCAL_LABELS);
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
    const term = entry.assistant ? `${entry.path} — ${entry.assistant}` : entry.path;
    addFact(elements.assistantFacts, term, entry.detail ? `${label} (${entry.detail})` : label);
  }

  elements.assistantMessage.textContent = data.message || "";

  // Ohne Installation gibt es nichts einzurichten, der Button bleibt grau.
  const installed = data.environment && data.environment.installed;
  const state = !installed ? "blocked" : data.ok ? "ok" : "todo";
  setBlockState(elements.assistantPill, elements.assistantApply, state);
}

// Der Remediation-Block ist eine Einstellung, kein Einrichtungsschritt: die
// Auswahl wird sofort gespeichert, ein eigener Button waere ein Zwischenschritt
// ohne Nutzen.
async function loadRemediation() {
  elements.remediationPill.className = "pill muted";
  elements.remediationPill.textContent = "Pruefen...";
  try {
    const response = await fetch("/api/remediation", { cache: "no-store" });
    renderRemediation(await response.json());
  } catch {
    elements.remediationMessage.textContent = "Status konnte nicht geladen werden.";
  }
}

async function setRemediation(mode) {
  elements.remediationPill.className = "pill muted";
  elements.remediationPill.textContent = "Speichern...";
  try {
    const response = await fetch("/api/remediation", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ mode }),
    });
    renderRemediation(await response.json());
  } catch {
    elements.remediationMessage.textContent = "Speichern fehlgeschlagen.";
  }
}

function renderRemediation(data) {
  elements.remediationChoices.replaceChildren();
  elements.remediationMessage.textContent = data.message || "";

  const current = data.current || {};
  for (const choice of data.choices || []) {
    // current.mode traegt auch ohne Eintrag in der Datei den Standard.
    const selected = current.mode === choice.mode;

    const label = document.createElement("label");
    label.className = selected ? "choice selected" : "choice";

    const input = document.createElement("input");
    input.type = "radio";
    input.name = "remediation-mode";
    input.value = choice.mode;
    input.checked = selected;
    input.addEventListener("change", () => setRemediation(choice.mode));

    const text = document.createElement("div");
    const title = document.createElement("div");
    title.className = "choice-label";
    title.textContent = choice.label;
    const description = document.createElement("p");
    description.className = "choice-description";
    description.textContent = choice.description;
    text.append(title, description);

    label.append(input, text);
    elements.remediationChoices.append(label);
  }

  elements.remediationPill.className = "pill ok";
  elements.remediationPill.textContent = current.prRequired ? "Nur ueber PR" : "Direkte Fixes moeglich";

  // Der Standard gilt auch ohne Eintrag; das sollte sichtbar sein, damit
  // niemand einen ausdruecklich gewaehlten Wert vermutet.
  if (!current.configured && !data.message) {
    elements.remediationMessage.textContent =
      "Standard, noch nicht in der Konfiguration festgehalten. Eine Auswahl schreibt sie fest.";
  }
}

// Der Tool-Block hat keinen Button: installiert wird im Terminal, weil das den
// Host veraendert. Die Pill zeigt nur den Zustand.
async function loadTools() {
  elements.toolsPill.className = "pill muted";
  elements.toolsPill.textContent = "Pruefen...";
  try {
    const response = await fetch("/api/tools", { cache: "no-store" });
    renderTools(await response.json());
  } catch {
    elements.toolsMessage.textContent = "Status konnte nicht geladen werden.";
  }
}

function renderTools(data) {
  elements.toolsFacts.replaceChildren();
  elements.toolsMessage.textContent = data.message || "";
  elements.toolsCommand.classList.add("hidden");

  if (!data.available || data.message) {
    elements.toolsPill.className = "pill muted";
    elements.toolsPill.textContent = "Unbekannt";
    return;
  }

  for (const tool of data.tools || []) {
    const label = tool.required ? tool.name : `${tool.name} (optional)`;
    const detail = tool.status === "ok" ? tool.version || "vorhanden" : `fehlt — ${tool.role}`;
    addFact(elements.toolsFacts, label, detail);
  }
  if (data.binDir) {
    addFact(elements.toolsFacts, "Installationsort", data.binDir);
  }

  if (data.ok) {
    elements.toolsPill.className = "pill ok";
    elements.toolsPill.textContent = "Vollstaendig";
    return;
  }

  elements.toolsPill.className = "pill warn";
  elements.toolsPill.textContent = `${data.missing} fehlt`;
  if (data.command) {
    elements.toolsCommandText.textContent = data.command;
    elements.toolsCommand.classList.remove("hidden");
  }
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
