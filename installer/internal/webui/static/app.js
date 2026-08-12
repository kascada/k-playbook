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
  pathCard: document.getElementById("path-card"),
  pathPill: document.getElementById("path-pill"),
  pathMessage: document.getElementById("path-message"),
  pathCommand: document.getElementById("path-command"),
  pathCommandText: document.getElementById("path-command-text"),
  cleanCard: document.getElementById("clean-card"),
  cleanPill: document.getElementById("clean-pill"),
  cleanFacts: document.getElementById("clean-facts"),
  cleanMessage: document.getElementById("clean-message"),
  cleanCommand: document.getElementById("clean-command"),
  cleanCommandText: document.getElementById("clean-command-text"),
  configCard: document.getElementById("config-card"),
  localCard: document.getElementById("local-card"),
  assistantCard: document.getElementById("assistant-card"),
  remediationCard: document.getElementById("remediation-card"),
  remediationPill: document.getElementById("remediation-pill"),
  remediationChoices: document.getElementById("remediation-choices"),
  remediationMessage: document.getElementById("remediation-message"),
  ghCard: document.getElementById("gh-card"),
  ghPill: document.getElementById("gh-pill"),
  ghChoices: document.getElementById("gh-choices"),
  ghFacts: document.getElementById("gh-facts"),
  ghMessage: document.getElementById("gh-message"),
  ghCommand: document.getElementById("gh-command"),
  ghCommandHint: document.getElementById("gh-command-hint"),
  ghCommandText: document.getElementById("gh-command-text"),
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
  contextCard: document.getElementById("context-card"),
  contextPill: document.getElementById("context-pill"),
  contextFacts: document.getElementById("context-facts"),
  contextCatalogs: document.getElementById("context-catalogs"),
  contextMessage: document.getElementById("context-message"),
  assistantPill: document.getElementById("assistant-pill"),
  assistantFacts: document.getElementById("assistant-facts"),
  assistantMessage: document.getElementById("assistant-message"),
  assistantApply: document.getElementById("assistant-apply"),
  docsCard: document.getElementById("docs-card"),
  docsPill: document.getElementById("docs-pill"),
  docsList: document.getElementById("docs-list"),
  docsMessage: document.getElementById("docs-message"),
  docOverlay: document.getElementById("doc-overlay"),
  docTitle: document.getElementById("doc-title"),
  docPath: document.getElementById("doc-path"),
  docViewer: document.getElementById("doc-viewer"),
  closeDoc: document.getElementById("close-doc"),
};

const HEALTH_INTERVAL = 1800;

const CONFIG_LABELS = { doneLabel: "Angelegt", todoLabel: "Anlegen" };

// Lesbare Beschreibung je Zustand aus dem Backend.
const STATE_LABELS = {
  ok: "eingerichtet",
  missing: "nicht vorhanden",
  stale: "zeigt woandershin",
  incomplete: "weicht vom Katalog ab",
  blocked: "blockiert",
  "no-source": "Quelle fehlt",
};

// Die Abweichungen eines Katalog-Eintrags, in der Reihenfolge, in der sie
// jemanden interessieren: erst was fehlt, zuletzt was ohnehin dem Projekt
// gehoert.
const REGISTRY_DEVIATIONS = [
  ["missing", "fehlt"],
  ["wrong", "zeigt woandershin"],
  ["stale", "verwaist"],
  ["blocked", "projekteigen, bleibt liegen"],
];

elements.shutdown.addEventListener("click", shutdown);
elements.update.addEventListener("click", onUpdateClick);
elements.configCreate.addEventListener("click", createConfig);
elements.localCreate.addEventListener("click", createLocal);
elements.assistantApply.addEventListener("click", applyAssistant);
elements.contextCard.addEventListener("toggle", onContextToggle);
elements.closeDoc.addEventListener("click", closeDocOverlay);
elements.docViewer.addEventListener("click", onDocViewerClick);
// Ein Klick neben das Fenster schliesst es; einer darin darf nicht durchgreifen.
elements.docOverlay.addEventListener("click", (event) => {
  if (event.target === elements.docOverlay) {
    closeDocOverlay();
  }
});
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && !elements.docOverlay.classList.contains("hidden")) {
    closeDocOverlay();
  }
});
window.addEventListener("pagehide", notifyClientGone);
startHealthChecks();
// Der Assistenten-Block folgt erst, wenn die Konfiguration steht; loadConfig
// blendet ihn dann ein und laedt ihn nach.
loadConfig();
// Der PATH betrifft den Host, nicht das Projekt — deshalb unabhaengig davon,
// ob eine Konfiguration gefunden wurde.
loadPath();
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
  renderCleanliness(data.cleanliness);

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

// Die Karte erscheint nur, wenn in der Installation lokal gearbeitet wurde.
// Sie kommt bei jeder Update-Pruefung mit, also auch ohne anstehendes Update —
// die Verschmutzung entsteht unabhaengig davon.
//
// Bewusst kein Knopf, der zuruecksetzt: das waere `git checkout -- .` in einem
// fremden Verzeichnis, und die Oberflaeche kann nicht wissen, ob dort jemand
// absichtlich entwickelt. Der Befehl steht zum Kopieren da, ausgefuehrt wird er
// vom Nutzer.
function renderCleanliness(state) {
  if (!state || state.clean) {
    elements.cleanCard.classList.add("hidden");
    return;
  }

  const blocking = (state.modified && state.modified.length > 0) || state.ahead > 0;
  elements.cleanPill.className = "pill warn";
  elements.cleanPill.textContent = blocking ? "Veraendert" : "Zusaetzliche Dateien";
  elements.cleanMessage.textContent = state.message || "";

  elements.cleanFacts.replaceChildren();
  if (state.ahead > 0) {
    addFact(elements.cleanFacts, "Lokale Commits", String(state.ahead));
  }
  for (const path of state.modified || []) {
    addFact(elements.cleanFacts, "Veraendert", path);
  }
  for (const path of state.untracked || []) {
    addFact(elements.cleanFacts, "Zusaetzlich", path);
  }

  // Lokale Commits sind mit Verwerfen nicht aufzuloesen; dafuer gibt es keinen
  // Befehl, den man blind vorschlagen koennte.
  if (state.ahead > 0) {
    elements.cleanCommand.classList.add("hidden");
  } else {
    elements.cleanCommandText.textContent = state.modified && state.modified.length > 0
      ? "git -C k-playbook checkout -- ."
      : "git -C k-playbook clean -nd";
    elements.cleanCommand.classList.remove("hidden");
  }

  elements.cleanCard.classList.remove("hidden");
}

// Legt eine Zeile in einer Faktenliste an und gibt sie zurueck, damit der
// Aufrufer sie noch kennzeichnen kann.
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

// Die Karte erscheint nur, solange etwas fehlt. Steht der Aufruf, ist sie
// nichts, was man wissen muesste — und wuerde die Schritte nur verdecken.
async function loadPath() {
  let data;
  try {
    const response = await fetch("/api/path", { cache: "no-store" });
    data = await response.json();
  } catch {
    return;
  }

  if (data.ok || !data.dir) {
    elements.pathCard.classList.add("hidden");
    return;
  }

  elements.pathPill.className = "pill warn";
  elements.pathPill.textContent = data.linked ? "Nicht im PATH" : "Fehlt";
  elements.pathMessage.textContent = data.message || "";

  if (data.export) {
    elements.pathCommandText.textContent = data.export;
    elements.pathCommand.classList.remove("hidden");
  } else {
    elements.pathCommand.classList.add("hidden");
  }
  elements.pathCard.classList.remove("hidden");
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
    elements.ghCard.classList.remove("hidden");
    elements.toolsCard.classList.remove("hidden");
    elements.docsCard.classList.remove("hidden");
    // Der Kontext-Block wird nur sichtbar, nicht geladen: das passiert erst
    // beim Aufklappen.
    elements.contextCard.classList.remove("hidden");
    loadLocal();
    loadAssistant();
    loadRemediation();
    loadGH();
    loadTools();
    loadDocs();
    return;
  }

  // Solange sie fehlt, ist dies der einzige Schritt, der zur Wahl steht.
  elements.configCard.classList.remove("hidden");
  elements.localCard.classList.add("hidden");
  elements.assistantCard.classList.add("hidden");
  elements.remediationCard.classList.add("hidden");
  elements.ghCard.classList.add("hidden");
  elements.toolsCard.classList.add("hidden");
  elements.docsCard.classList.add("hidden");
  elements.contextCard.classList.add("hidden");

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
    // "Fehlende Eintraege" statt "Fehlt": in dieser Liste steht links immer die
    // Bezeichnung und rechts der Wert. Ein Statuswort links liest sich, als
    // waere die Liste rechts der Zustand.
    addFact(elements.localFacts, "Fehlende Eintraege", missing.map((entry) => entry.path).join(", "));
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

  // Die Wurzeldatei zuerst: sie ist der Einstiegspunkt, alles andere haengt
  // daran.
  const root = data.root || {};
  if (root.path) {
    const detail = !root.present
      ? "nicht vorhanden"
      : root.hasMarker
        ? "enthaelt den Anstoss"
        : "vorhanden, Anstoss fehlt";
    addFact(elements.assistantFacts, "AGENTS.md — Einstieg", detail);
  }

  for (const entry of data.entries || []) {
    const label = STATE_LABELS[entry.state] || entry.state;
    const term = entry.assistant ? `${entry.path} — ${entry.assistant}` : entry.path;
    addFact(elements.assistantFacts, term, entry.detail ? `${label} (${entry.detail})` : label);

    // Bei einem Katalog-Eintrag zaehlt nicht nur, dass etwas nicht stimmt,
    // sondern welcher Command oder Skill gemeint ist.
    for (const [field, description] of REGISTRY_DEVIATIONS) {
      const names = entry[field] || [];
      if (names.length) {
        addFact(elements.assistantFacts, `↳ ${description}`, names.join(", "));
      }
    }
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

// Der gh-Block trennt zwei Dinge, die leicht verwechselt werden: die Entscheidung
// des Projekts steht in der Konfiguration und wird hier gesetzt; ob gh auf diesem
// Rechner liegt und angemeldet ist, ist ein Befund und wird nur gezeigt.
// Installation und Anmeldung bleiben im Terminal, wie bei den Security-Tools.
async function loadGH() {
  elements.ghPill.className = "pill muted";
  elements.ghPill.textContent = "Pruefen...";
  try {
    const response = await fetch("/api/gh", { cache: "no-store" });
    renderGH(await response.json());
  } catch {
    elements.ghMessage.textContent = "Status konnte nicht geladen werden.";
  }
}

async function setGH(status) {
  elements.ghPill.className = "pill muted";
  elements.ghPill.textContent = "Speichern...";
  try {
    const response = await fetch("/api/gh", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ status }),
    });
    renderGH(await response.json());
  } catch {
    elements.ghMessage.textContent = "Speichern fehlgeschlagen.";
  }
}

function renderGH(data) {
  elements.ghChoices.replaceChildren();
  elements.ghFacts.replaceChildren();
  elements.ghMessage.textContent = data.message || "";
  elements.ghCommand.classList.add("hidden");

  const current = data.current || {};
  const commands = data.commands || {};

  for (const choice of data.choices || []) {
    const selected = current.status === choice.status;

    const label = document.createElement("label");
    label.className = selected ? "choice selected" : "choice";

    const input = document.createElement("input");
    input.type = "radio";
    input.name = "gh-status";
    input.value = choice.status;
    input.checked = selected;
    input.addEventListener("change", () => setGH(choice.status));

    const text = document.createElement("div");
    const title = document.createElement("div");
    title.className = "choice-label";
    title.textContent = choice.label;
    const description = document.createElement("p");
    description.className = "choice-description";
    description.textContent = choice.description;
    text.append(title, description);

    label.append(input, text);
    elements.ghChoices.append(label);
  }

  addFact(elements.ghFacts, "gh", current.installed ? current.path : "nicht im PATH gefunden");
  if (current.installed) {
    addFact(elements.ghFacts, "Anmeldung", ghAccountLabel(current));
    if ((current.accounts || []).length > 1) {
      addFact(elements.ghFacts, "Hinterlegte Accounts", current.accounts.join(", "));
    }
  }

  // Die offene Entscheidung ist ein Fehler und kein Hinweis: ohne sie wissen die
  // Commands nicht, ob ein fehlendes gh ein Problem oder gewollt ist.
  if (current.status !== "enabled" && current.status !== "disabled") {
    elements.ghPill.className = "pill error";
    elements.ghPill.textContent = "Nicht entschieden";
    if (!data.message) {
      elements.ghMessage.textContent =
        "Noch nicht entschieden. Commands, die gh brauchen, brechen ab, bis hier eine Wahl steht.";
    }
    return;
  }

  if (current.status === "disabled") {
    elements.ghPill.className = "pill muted";
    elements.ghPill.textContent = "Nicht genutzt";
    return;
  }

  if (!current.installed) {
    elements.ghPill.className = "pill warn";
    elements.ghPill.textContent = "gh fehlt";
    showGHCommand("gh ist auf diesem Rechner nicht installiert. Anleitung fuer den passenden Paketmanager:", commands.install);
    return;
  }

  if (!current.loggedIn) {
    elements.ghPill.className = "pill warn";
    elements.ghPill.textContent = "Nicht angemeldet";
    showGHCommand("Die Anmeldung laeuft ueber den Browser und gehoert ins Terminal:", commands.login);
    return;
  }

  elements.ghPill.className = "pill ok";
  elements.ghPill.textContent = current.account ? `Angemeldet als ${current.account}` : "Angemeldet";

  // Der Wechsel gilt fuer jedes Terminal und jedes Projekt auf diesem Rechner.
  // Deshalb steht er hier als Befehl und nicht als Knopf: wer ihn ausfuehrt, tut
  // es bewusst und sieht danach das Ergebnis in seiner Shell.
  if ((current.accounts || []).length > 1) {
    showGHCommand(
      "Umschalten gilt fuer alle Terminals und Projekte auf diesem Rechner, nicht nur fuer dieses:",
      commands.switch,
    );
  }
}

function ghAccountLabel(current) {
  if (current.tokenFromEnv) {
    return current.account
      ? `${current.account}; zusaetzlich ein Token in GH_TOKEN/GITHUB_TOKEN, das sticht`
      : "ueber GH_TOKEN/GITHUB_TOKEN, ohne Accountnamen";
  }
  if (!current.loggedIn) {
    return "kein Account hinterlegt";
  }
  // Gelesen aus der gh-Konfiguration, nicht beim Server geprueft.
  return `${current.account || "unbekannt"} (aus der gh-Konfiguration, Token nicht geprueft)`;
}

function showGHCommand(hint, command) {
  if (!command) {
    return;
  }
  elements.ghCommandHint.textContent = hint;
  elements.ghCommandText.textContent = command;
  elements.ghCommand.classList.remove("hidden");
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

// Der Doku-Block listet die mitgelieferten Markdown-Dateien; gelesen wird eine
// davon erst auf Klick, in einem Fenster ueber der Seite.
async function loadDocs() {
  elements.docsPill.className = "pill muted";
  elements.docsPill.textContent = "Laden...";
  try {
    const response = await fetch("/api/docs", { cache: "no-store" });
    renderDocs(await response.json());
  } catch {
    elements.docsPill.className = "pill warn";
    elements.docsPill.textContent = "Fehlgeschlagen";
    elements.docsMessage.textContent = "Doku konnte nicht geladen werden.";
  }
}

function renderDocs(data) {
  elements.docsList.replaceChildren();
  elements.docsMessage.textContent = data.message || "";

  if (!data.available) {
    elements.docsList.classList.add("empty");
    elements.docsPill.className = "pill muted";
    elements.docsPill.textContent = "Nicht installiert";
    return;
  }

  // Fehlt das Verzeichnis, steht der Grund in der Meldung; eine leere Liste
  // waere dafuer die falsche Auskunft.
  if (data.message) {
    elements.docsList.classList.add("empty");
    elements.docsPill.className = "pill warn";
    elements.docsPill.textContent = "Nicht lesbar";
    return;
  }

  const docs = data.docs || [];
  elements.docsList.classList.toggle("empty", docs.length === 0);
  if (docs.length === 0) {
    elements.docsList.textContent = "Keine Markdown-Dateien vorhanden.";
    elements.docsPill.className = "pill muted";
    elements.docsPill.textContent = "Leer";
    return;
  }

  for (const doc of docs) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "doc-link";
    button.textContent = doc.title || doc.path;
    // Der Titel steht auf dem Knopf, der Dateiname gehoert trotzdem dazu.
    button.title = doc.path;
    button.dataset.path = doc.path;
    button.addEventListener("click", () => openDoc(doc.path, doc.title || doc.path));
    elements.docsList.append(button);
  }

  elements.docsPill.className = "pill ok";
  elements.docsPill.textContent = docs.length === 1 ? "1 Datei" : `${docs.length} Dateien`;
}

// Die offene Datei; Verweise darin werden relativ zu ihr aufgeloest.
let currentDocPath = "";

async function openDoc(path, title, anchor = "") {
  currentDocPath = path;
  openDocOverlay(title || path, path);
  setActiveDocPath(path);
  elements.docViewer.classList.add("empty");
  elements.docViewer.textContent = "Wird geladen...";

  try {
    const response = await fetch(`/api/docs/file?path=${encodeURIComponent(path)}`, { cache: "no-store" });
    const data = await response.json();
    // Wurde inzwischen etwas anderes geoeffnet, gehoert diese Antwort nicht
    // mehr ins Fenster.
    if (currentDocPath !== path) {
      return;
    }
    if (data.message) {
      elements.docViewer.textContent = data.message;
      return;
    }

    elements.docTitle.textContent = data.title || title || path;
    elements.docPath.textContent = data.path || path;
    elements.docViewer.classList.remove("empty");
    // Das HTML kommt aus dem eigenen Backend. Gerendert wird dort mit
    // abgeschaltetem Roh-HTML, es steht also nichts darin, was nicht aus der
    // Markdown-Struktur der Datei stammt.
    elements.docViewer.innerHTML = data.html || "";
    scrollToAnchor(anchor);
    renderMermaidDiagrams(elements.docViewer);
  } catch {
    elements.docViewer.textContent = "Datei konnte nicht geladen werden.";
  }
}

function setActiveDocPath(path) {
  for (const button of elements.docsList.querySelectorAll(".doc-link")) {
    button.classList.toggle("active", button.dataset.path === path);
  }
}

// Verweise in der Doku zeigen ueberwiegend auf andere Dateien der Doku. Ohne
// eigene Behandlung wuerde ein Klick die Oberflaeche verlassen — und mit ihr
// den Server, der an ihr haengt.
function onDocViewerClick(event) {
  const link = event.target.closest("a[href]");
  if (!link) {
    return;
  }

  const href = link.getAttribute("href");
  if (href.startsWith("#")) {
    event.preventDefault();
    scrollToAnchor(href.slice(1));
    return;
  }

  // Ein Ziel mit Schema fuehrt aus der Doku heraus und gehoert in ein eigenes
  // Fenster.
  if (/^[a-z][a-z0-9+.-]*:/i.test(href)) {
    link.target = "_blank";
    link.rel = "noopener";
    return;
  }

  event.preventDefault();
  const [target, anchor] = splitAnchor(href);

  // Ein reiner Anker ohne Dateiname ist bereits oben abgefangen; bleibt ein
  // Verweis auf eine Datei. Alles ausser Markdown kann diese Ansicht nicht
  // zeigen, der Pfad steht aber im Text und laesst sich im Editor oeffnen.
  if (target.toLowerCase().endsWith(".md")) {
    openDoc(resolveDocPath(currentDocPath, target), "", anchor);
  }
}

// Loest einen Verweis gegen das Verzeichnis der offenen Datei auf; die
// URL-Klasse erledigt dabei "./" und "../".
function resolveDocPath(base, href) {
  const resolved = new URL(href, `https://docs.invalid/${base}`);
  return decodeURIComponent(resolved.pathname).replace(/^\//, "");
}

function splitAnchor(href) {
  const index = href.indexOf("#");
  return index === -1 ? [href, ""] : [href.slice(0, index), href.slice(index + 1)];
}

// Springt zu einer Ueberschrift der offenen Datei. Ohne Anker beginnt die
// Datei oben — sonst bliebe die Ansicht dort stehen, wo die vorige endete.
function scrollToAnchor(anchor) {
  const target = anchor ? elements.docViewer.querySelector(`#${CSS.escape(anchor)}`) : null;
  if (target) {
    target.scrollIntoView();
    return;
  }
  elements.docViewer.scrollTop = 0;
}

function openDocOverlay(title, path) {
  elements.docTitle.textContent = title;
  elements.docPath.textContent = path;
  elements.docOverlay.classList.remove("hidden");
  document.body.classList.add("doc-overlay-open");
  elements.closeDoc.focus({ preventScroll: true });
}

function closeDocOverlay() {
  elements.docOverlay.classList.add("hidden");
  document.body.classList.remove("doc-overlay-open");
}

// Mermaid ist zu gross, um es mitzuliefern, und wird deshalb nur bei Bedarf vom
// CDN geholt. Ohne Netz bleibt der Quelltext des Diagramms als Codeblock
// stehen — die Datei ist dann immer noch lesbar.
const MERMAID_MODULE_URL = "https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs";

let mermaidLoader = null;
let mermaidDiagramCount = 0;

async function renderMermaidDiagrams(container) {
  const blocks = Array.from(container.querySelectorAll("pre > code.language-mermaid"));
  if (blocks.length === 0) {
    return;
  }

  let mermaid;
  try {
    mermaid = await loadMermaid();
  } catch (error) {
    for (const block of blocks) {
      const note = document.createElement("p");
      note.className = "mermaid-message";
      note.textContent = `Mermaid konnte nicht geladen werden (${error.message}); das Diagramm bleibt als Quelltext stehen.`;
      block.closest("pre").before(note);
    }
    return;
  }

  for (const block of blocks) {
    const pre = block.closest("pre");
    // Das Laden dauert; inzwischen kann eine andere Datei im Fenster stehen.
    if (!pre.isConnected) {
      continue;
    }
    const source = block.textContent.trim();

    const diagram = document.createElement("div");
    diagram.className = "mermaid-diagram";
    diagram.setAttribute("aria-label", "Mermaid-Diagramm");
    pre.replaceWith(diagram);

    try {
      const { svg } = await mermaid.render(`doc-mermaid-${++mermaidDiagramCount}`, source);
      diagram.innerHTML = svg;
    } catch (error) {
      // Ein fehlerhaftes Diagramm ersetzt sich selbst durch die Meldung und
      // seinen Quelltext, damit die Stelle im Text nicht einfach verschwindet.
      diagram.classList.add("mermaid-error");
      diagram.textContent = `Diagramm konnte nicht gezeichnet werden: ${error.message}`;
      diagram.append(pre);
    }
  }
}

function loadMermaid() {
  if (!mermaidLoader) {
    mermaidLoader = import(MERMAID_MODULE_URL).then((module) => {
      const mermaid = module.default;
      mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: "neutral" });
      return mermaid;
    });
  }
  return mermaidLoader;
}

// Der Kontext-Block ist der einzige, der nicht beim Seitenaufbau laedt: seine
// Ausgabe ist lang und wird nur gebraucht, wenn jemand nachsieht.
let contextLoaded = false;

function onContextToggle() {
  if (!elements.contextCard.open || contextLoaded) {
    return;
  }
  contextLoaded = true;
  loadContext();
}

async function loadContext() {
  elements.contextPill.className = "pill muted";
  elements.contextPill.textContent = "Laden...";
  try {
    const response = await fetch("/api/context", { cache: "no-store" });
    renderContext(await response.json());
  } catch {
    // Beim naechsten Aufklappen darf es wieder versucht werden.
    contextLoaded = false;
    elements.contextPill.className = "pill warn";
    elements.contextPill.textContent = "Fehlgeschlagen";
    elements.contextMessage.textContent = "Kontext konnte nicht geladen werden.";
  }
}

// Herkunft eines Katalogeintrags in Worten.
const ORIGIN_LABELS = {
  dist: "mitgeliefert",
  local: "projekteigen",
  override: "projekteigen, ersetzt mitgeliefert",
};

// Reihenfolge und Beschriftung der Katalogsorten. Die Antwort ist ein Objekt
// und traegt die Sorten alphabetisch; hier steht die fachliche Reihenfolge.
const CATALOG_LABELS = {
  rules: "Regeln",
  reviews: "Reviews",
  checks: "Checks",
};

function renderContext(data) {
  elements.contextFacts.replaceChildren();
  elements.contextCatalogs.replaceChildren();
  elements.contextMessage.textContent = data.message || "";

  if (!data.available || !data.context) {
    elements.contextPill.className = data.message ? "pill warn" : "pill muted";
    elements.contextPill.textContent = data.message ? "Nicht lesbar" : "Nicht installiert";
    return;
  }

  const context = data.context;
  const display = data.display || {};

  addFact(elements.contextFacts, "Projekt", display.projectDir || context.project.dir);
  addFact(elements.contextFacts, "Repository", display.repoRoot || context.project.repoRoot);
  addFact(elements.contextFacts, "VCS", context.project.vcs || "keines");
  addFact(elements.contextFacts, "Konfiguration", display.config || context.project.config);
  addFact(elements.contextFacts, "Installation", display.playbook || context.playbook.dir);
  addFact(elements.contextFacts, "Projekteigen", display.local || context.local.dir);
  addFact(elements.contextFacts, "Umgang mit Befunden", context.remediation.mode || "unbekannt");

  // Pfade werden gegen das Projektverzeichnis gekuerzt: der gemeinsame Anfang
  // steht bereits oben und wuerde jede Zeile nur verlaengern.
  const root = context.project.dir;
  addPathGroup("Instruktionen", context.instructions || [], root);

  const catalogs = context.catalogs || {};
  const kinds = Object.keys(CATALOG_LABELS).filter((kind) => kind in catalogs);
  // Eine spaeter hinzukommende Sorte faellt nicht unter den Tisch.
  kinds.push(...Object.keys(catalogs).filter((kind) => !(kind in CATALOG_LABELS)));

  for (const kind of kinds) {
    addCatalogGroup(CATALOG_LABELS[kind] || kind, catalogs[kind] || [], root);
  }

  addPathGroup("Guidelines", context.guidelines || [], root);

  elements.contextPill.className = "pill ok";
  elements.contextPill.textContent = `Schema ${context.schemaVersion || "?"}`;
}

// Legt eine benannte Gruppe an und gibt deren Faktenliste zurueck.
function addContextGroup(title) {
  const group = document.createElement("div");
  group.className = "catalog-group";

  const heading = document.createElement("h3");
  heading.textContent = title;

  const list = document.createElement("dl");
  list.className = "facts";

  group.append(heading, list);
  elements.contextCatalogs.append(group);
  return list;
}

function addCatalogGroup(title, entries, root) {
  const list = addContextGroup(`${title} (${entries.length})`);
  if (entries.length === 0) {
    addFact(list, "keine", "");
    return;
  }

  for (const entry of entries) {
    const origin = ORIGIN_LABELS[entry.origin] || entry.origin;
    const row = addFact(list, entry.key, entry.disabled ? `${origin} · abgeschaltet` : origin);
    row.classList.toggle("disabled", Boolean(entry.disabled));
    // Welche Datei tatsaechlich gilt, ist die eigentliche Auskunft des Overlays.
    row.title = relativePath(entry.path, root);
  }
}

function addPathGroup(title, paths, root) {
  const list = addContextGroup(`${title} (${paths.length})`);
  if (paths.length === 0) {
    addFact(list, "keine", "");
    return;
  }

  for (const path of paths) {
    addFact(list, relativePath(path, root), "");
  }
}

function relativePath(path, root) {
  if (path && root && path.startsWith(root + "/")) {
    return path.slice(root.length + 1);
  }
  return path || "";
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
