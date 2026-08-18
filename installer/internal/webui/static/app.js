"use strict";

// Heartbeat, Abmeldung und serverAvailable stehen in session.js — sie gelten
// für jede Seite gleich und werden vor dieser Datei geladen.

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
  toolsLanguages: document.getElementById("tools-languages"),
  toolsFacts: document.getElementById("tools-facts"),
  toolsMessage: document.getElementById("tools-message"),
  toolsCommand: document.getElementById("tools-command"),
  toolsCommandText: document.getElementById("tools-command-text"),
  privateCard: document.getElementById("private-card"),
  privatePill: document.getElementById("private-pill"),
  privateEntries: document.getElementById("private-entries"),
  privateMessage: document.getElementById("private-message"),
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
  mcpCard: document.getElementById("mcp-card"),
  mcpPill: document.getElementById("mcp-pill"),
  mcpCommand: document.getElementById("mcp-command"),
  mcpFacts: document.getElementById("mcp-facts"),
  mcpWorkdir: document.getElementById("mcp-workdir"),
  mcpMessage: document.getElementById("mcp-message"),
  mcpApply: document.getElementById("mcp-apply"),
  docsCard: document.getElementById("docs-card"),
  workflowsCard: document.getElementById("workflows-card"),
  workflowsReviews: document.getElementById("workflows-reviews"),
  workflowsTasks: document.getElementById("workflows-tasks"),
  workflowsMessage: document.getElementById("workflows-message"),
  docsPill: document.getElementById("docs-pill"),
  docsList: document.getElementById("docs-list"),
  docsMessage: document.getElementById("docs-message"),
  docOverlay: document.getElementById("doc-overlay"),
  docTitle: document.getElementById("doc-title"),
  docPath: document.getElementById("doc-path"),
  docViewer: document.getElementById("doc-viewer"),
  closeDoc: document.getElementById("close-doc"),
  blockNav: document.getElementById("block-nav"),
};

const CONFIG_LABELS = { doneLabel: "Angelegt", todoLabel: "Anlegen" };
// Beide Beschriftungen gleich: der Knopf tut hier nur eines, und im
// blockierten Fall wechselte er sonst auf die Beschriftung des erledigten
// Zustands, ohne dass etwas erledigt wäre.
const RESET_LABELS = {
  doneLabel: "Zurücksetzen und neu anlegen",
  todoLabel: "Zurücksetzen und neu anlegen",
};

// Lesbare Beschreibung je Zustand aus dem Backend.
const STATE_LABELS = {
  ok: "eingerichtet",
  missing: "nicht vorhanden",
  stale: "zeigt woandershin",
  incomplete: "weicht vom Katalog ab",
  blocked: "blockiert",
  "no-source": "Quelle fehlt",
  conflict: "Konflikt, Handarbeit nötig",
};

// Die Abweichungen eines Katalog-Eintrags, in der Reihenfolge, in der sie
// jemanden interessieren: erst was fehlt, zuletzt was ohnehin dem Projekt
// gehört.
const REGISTRY_DEVIATIONS = [
  ["missing", "fehlt"],
  ["wrong", "zeigt woandershin"],
  ["stale", "verwaist"],
  ["blocked", "projekteigen, bleibt liegen"],
];

elements.shutdown.addEventListener("click", shutdown);
elements.update.addEventListener("click", onUpdateClick);
elements.configCreate.addEventListener("click", onConfigClick);
elements.localCreate.addEventListener("click", createLocal);
elements.assistantApply.addEventListener("click", applyAssistant);
elements.mcpApply.addEventListener("click", applyMCP);
elements.contextCard.addEventListener("toggle", onContextToggle);
elements.closeDoc.addEventListener("click", closeDocOverlay);
elements.docViewer.addEventListener("click", onDocViewerClick);
// Ein Klick neben das Fenster schließt es; einer darin darf nicht durchgreifen.
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
// Ein Listener für alle Kopier-Knöpfe: sie stehen in Blöcken, die erst
// später sichtbar werden, und einzeln gebundene Listener müssten nachgezogen
// werden.
document.addEventListener("click", onCopyClick);
// Muss vor den Ladefunktionen laufen: die blenden Blöcke ein, und das Menü
// zieht das nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();
// Die Startseite hat für den beendeten Server ein eigenes Fenster.
startSession(showClosed);
// Der Assistenten-Block folgt erst, wenn die Konfiguration steht; loadConfig
// blendet ihn dann ein und lädt ihn nach.
loadConfig();
// Der PATH betrifft den Host, nicht das Projekt — deshalb unabhängig davon,
// ob eine Konfiguration gefunden wurde.
loadPath();
// Die Update-Prüfung braucht das Netz. Sie läuft nebenher, damit die Seite
// nicht auf einen langsamen Remote wartet.
checkUpdate();

// updateAvailable steuert, was ein Klick auf den Button tut: prüfen oder
// tatsächlich aktualisieren. devSyncActive geht beidem vor: solange ein
// Arbeitsstand eingespielt ist, gibt es nichts zu ziehen.
let updateAvailable = false;
let devSyncActive = false;

async function checkUpdate() {
  elements.update.disabled = true;
  elements.update.textContent = "Prüfe...";
  try {
    const response = await fetch("/api/update", { cache: "no-store" });
    renderUpdate(await response.json());
  } catch {
    resetUpdateButton("Update prüfen");
  }
}

async function onUpdateClick() {
  // Ein eingespielter Arbeitsstand wird zuerst verworfen, und zwar allein: der
  // Zustand, den man gerade ansieht, verschwindet dabei. Ob danach ein Update
  // ansteht, zeigt die Antwort — das zieht dann ein zweiter Klick.
  if (devSyncActive) {
    await discardDevSync();
    return;
  }

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
        "Das Programm wurde aktualisiert. Dieses Fenster schließen und " +
          "bin/k-playbook neu starten, um die neue Version zu verwenden."
      );
    }
  } catch {
    resetUpdateButton("Update prüfen");
  }
}

function renderUpdate(data) {
  const cleanliness = data.cleanliness || {};
  const blockingCleanliness = Boolean(
    (cleanliness.modified && cleanliness.modified.length > 0) || cleanliness.ahead > 0,
  );
  updateAvailable = Boolean(data.available) && !blockingCleanliness;
  devSyncActive = Boolean(cleanliness.devSync);
  renderCleanliness(cleanliness);

  // Der eingespielte Arbeitsstand geht vor: er verdeckt, was upstream liegt,
  // und muss erst weg. Der Knopf sagt, was er tut — was man ansieht,
  // verschwindet dabei.
  if (devSyncActive) {
    elements.update.className = "secondary";
    elements.update.textContent = "Arbeitsstand verwerfen";
    elements.update.title =
      "Stellt den unberührten Clone wieder her. Danach lässt sich auf ein Update prüfen.";
    elements.update.disabled = false;
    return;
  }

  if (data.available && blockingCleanliness) {
    elements.update.className = "secondary";
    elements.update.textContent = "Update blockiert";
    elements.update.title = cleanliness.message || `${data.local} -> ${data.remote} (${data.branch})`;
    elements.update.disabled = false;
    return;
  }

  if (updateAvailable) {
    // Hervorgehoben, solange etwas anliegt.
    elements.update.className = "primary attention-highlight";
    elements.update.textContent = "Update verfügbar";
    elements.update.title = `${data.local} -> ${data.remote} (${data.branch})`;
    elements.update.disabled = false;
    return;
  }

  // Ohne Meldung ist der Stand geprüft und gleich; mit Meldung konnte nicht
  // geprüft werden, dann bleibt es bei der Aufforderung.
  resetUpdateButton(data.message ? "Update prüfen" : "Version ist aktuell");
  elements.update.title = data.message || `Stand ${data.local || "unbekannt"} (${data.branch || "?"})`;
}

async function discardDevSync() {
  elements.update.disabled = true;
  elements.update.textContent = "Verwerfe...";
  try {
    const response = await fetch("/api/update/discard", { method: "POST" });
    renderUpdate(await response.json());
  } catch {
    resetUpdateButton("Arbeitsstand verwerfen");
  }
}

function resetUpdateButton(label) {
  updateAvailable = false;
  elements.update.className = "secondary";
  elements.update.textContent = label;
  elements.update.disabled = false;
}

// Die Karte erscheint nur, wenn in der Installation lokal gearbeitet wurde.
// Sie kommt bei jeder Update-Prüfung mit, also auch ohne anstehendes Update —
// die Verschmutzung entsteht unabhängig davon.
//
// Bewusst kein Knopf, der zurücksetzt: das wäre `git checkout -- .` in einem
// fremden Verzeichnis, und die Oberfläche kann nicht wissen, ob dort jemand
// absichtlich entwickelt. Der Befehl steht zum Kopieren da, ausgeführt wird er
// vom Nutzer.
function renderCleanliness(state) {
  if (!state || state.clean) {
    elements.cleanCard.classList.add("hidden");
    return;
  }

  elements.cleanFacts.replaceChildren();
  elements.cleanMessage.textContent = state.message || "";

  // Ein eingespielter Arbeitsstand ist gewollt, kein Versehen. Deshalb keine
  // Warnfarbe und kein Befehl zum Verwerfen — der stünde hier für „mach die
  // Arbeit rückgängig, die du gerade ansehen willst".
  if (state.devSync) {
    elements.cleanPill.className = "pill muted";
    elements.cleanPill.textContent = "Entwicklungsstand";
    // Kein Befehl zum Kopieren: dafür gibt es oben den Knopf. Der Befehl
    // stünde nur als zweiter Weg daneben, der dasselbe tut.
    elements.cleanCommand.classList.add("hidden");
    elements.cleanCard.classList.remove("hidden");
    return;
  }

  const blocking = (state.modified && state.modified.length > 0) || state.ahead > 0;
  elements.cleanPill.className = "pill warn";
  elements.cleanPill.textContent = blocking ? "Verändert" : "Zusätzliche Dateien";

  if (state.ahead > 0) {
    addFact(elements.cleanFacts, "Lokale Commits", String(state.ahead));
  }
  for (const path of state.modified || []) {
    addFact(elements.cleanFacts, "Verändert", path);
  }
  for (const path of state.untracked || []) {
    addFact(elements.cleanFacts, "Zusätzlich", path);
  }

  // Lokale Commits sind mit Verwerfen nicht aufzulösen; dafür gibt es keinen
  // Befehl, den man blind vorschlagen könnte.
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

// Legt eine Zeile in einer Faktenliste an und gibt sie zurück, damit der
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

// Setzt einen drehenden Ring mit Text in ein Meldungsfeld. Aufgeräumt wird
// nichts von Hand: jede render-Funktion beschreibt ihr Meldungsfeld als Erstes
// und wirft den Ring damit hinaus.
function renderLoading(element, text) {
  const loading = document.createElement("span");
  loading.className = "loading-inline";
  const spinner = document.createElement("span");
  spinner.className = "loading-spinner";
  spinner.setAttribute("aria-hidden", "true");
  loading.append(spinner, text);
  element.replaceChildren(loading);
}

// Die Karte erscheint nur, solange etwas fehlt. Steht der Aufruf, ist sie
// nichts, was man wissen müsste — und würde die Schritte nur verdecken.
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

// Liegt eine Konfiguration aus einem abgelösten Modell vor, schreibt derselbe
// Knopf über einen anderen Endpunkt: `POST /api/config` geht nie über eine
// vorhandene Datei, und diese Grenze soll auch hier sichtbar bleiben.
let configOutdated = false;

async function onConfigClick() {
  await createConfig(configOutdated ? "/api/config/reset" : "/api/config");
}

async function createConfig(endpoint) {
  setBlockState(elements.configPill, elements.configCreate, "busy");
  const body = JSON.stringify({
    projectDir: elements.configProjectDir.value.trim(),
    repoRoot: elements.configRepoRoot.value.trim(),
  });

  try {
    const response = await fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body,
    });
    const data = await response.json();
    if (data.installed) {
      // Die Kopfzeile wird serverseitig gefüllt und muss den neuen Ort zeigen.
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
  configOutdated = false;

  // Steht die Konfiguration, ist dieser Schritt erledigt und verschwindet. Wo
  // alles liegt, zeigt die Kopfzeile.
  if (data.installed) {
    // Ausnahme: eine Konfiguration, die neuer ist als das Werkzeug. Sie ist
    // nichts, was sich hier einrichten ließe — die Installation ist hinterher.
    // Die Karte bleibt deshalb als Warnung stehen, statt die Meldung in einem
    // ausgeblendeten Block zu verstecken.
    if (data.schema === "newer") {
      renderNewerConfig(data);
    } else {
      elements.configCard.classList.add("hidden");
    }
    elements.localCard.classList.remove("hidden");
    elements.privateCard.classList.remove("hidden");
    elements.mcpCard.classList.remove("hidden");
    elements.workflowsCard.classList.remove("hidden");
    elements.assistantCard.classList.remove("hidden");
    elements.remediationCard.classList.remove("hidden");
    elements.ghCard.classList.remove("hidden");
    elements.toolsCard.classList.remove("hidden");
    elements.docsCard.classList.remove("hidden");
    // Der Kontext-Block wird nur sichtbar, nicht geladen: das passiert erst
    // beim Aufklappen.
    elements.contextCard.classList.remove("hidden");
    loadLocal();
    loadPrivate();
    loadMCP();
    loadAssistant();
    loadRemediation();
    loadGH();
    loadTools();
    loadWorkflows();
    loadDocs();
    return;
  }

  // Solange sie fehlt, ist dies der einzige Schritt, der zur Wahl steht.
  elements.configCard.classList.remove("hidden");
  elements.localCard.classList.add("hidden");
  elements.privateCard.classList.add("hidden");
  elements.mcpCard.classList.add("hidden");
  elements.workflowsCard.classList.add("hidden");
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

  configOutdated = data.schema === "outdated" || data.schema === "missing";
  // Beim Zurücksetzen steht das Hauptverzeichnis fest — es ist der Ort der
  // Datei, die ersetzt wird. Nur das Repository bleibt zur Wahl.
  elements.configProjectDir.readOnly = configOutdated;
  if (configOutdated) {
    renderOutdatedConfig(data);
    return;
  }

  // Kommt mehr als ein Ort in Frage, muss der Nutzer sehen, welche das sind —
  // der Vorschlag steht bereits im Feld.
  const projectCandidates = suggestion.projectCandidates || [];
  if (projectCandidates.length > 1) {
    addFact(elements.configFacts, "Weitere mögliche Orte", projectCandidates.slice(1).join(", "));
  }

  const repoCandidates = suggestion.repoCandidates || [];
  if (repoCandidates.length > 1) {
    addFact(elements.configFacts, "Gefundene Repositories", repoCandidates.join(", "));
  }

  setBlockState(elements.configPill, elements.configCreate, "todo", CONFIG_LABELS);
}

// Die vorhandene Datei beschreibt ein abgelöstes Modell. Der Block erklärt,
// was sie beschreibt und was mit ihr geschieht — beides braucht es, weil hier
// als Einziges in der Oberfläche etwas Vorhandenes ersetzt wird.
function renderOutdatedConfig(data) {
  addFact(elements.configFacts, "Gefundene Datei", data.configPath || "");
  addFact(
    elements.configFacts,
    "schema_version",
    data.schemaVersion ? `${data.schemaVersion} — ${data.legacyModel || "abgelöst"}` : "fehlt",
  );

  const legacy = data.legacyContent || [];
  if (legacy.length > 0) {
    // Der Umzug geht vor: das Installationsverzeichnis wird beim nächsten
    // Update ersetzt, und was dort liegt, wäre danach weg.
    addFact(elements.configFacts, "Zuerst umziehen nach k-playbook-local/", legacy.join(", "));
    setBlockState(elements.configPill, elements.configCreate, "blocked", RESET_LABELS);
    elements.configPill.textContent = "Veraltet";
    return;
  }

  addFact(elements.configFacts, "Die alte Datei", "wird daneben gesichert, nicht gelöscht");
  setBlockState(elements.configPill, elements.configCreate, "todo", RESET_LABELS);
  elements.configPill.textContent = "Veraltet";
}

// Der umgekehrte Fall: die Datei ist neuer als das Werkzeug. Zurücksetzen wäre
// hier genau falsch — es würde die neuere Konfiguration wegwerfen. Was hilft,
// ist ein Update der Installation, und das steht im Block darüber.
function renderNewerConfig(data) {
  elements.configCard.classList.remove("hidden");
  elements.configForm.classList.add("hidden");
  addFact(elements.configFacts, "Gefundene Datei", data.configPath || "");
  addFact(
    elements.configFacts,
    "schema_version",
    `${data.schemaVersion || "?"} — neuer als diese Installation`,
  );
  addFact(elements.configFacts, "Was hilft", `git pull in ${data.playbookDir || "k-playbook/"}`);
  setBlockState(elements.configPill, elements.configCreate, "blocked", CONFIG_LABELS);
  elements.configPill.textContent = "Installation zu alt";
}

// Setzt Pill und Button eines Blocks aus einem Zustand. Einheitliche Regel für
// alle Blöcke: der Button kann immer ausgelöst werden und tut immer dasselbe,
// nur Beschriftung und Hervorhebung wechseln.
function setBlockState(pill, button, state, labels = {}) {
  const { doneLabel = "Aktualisieren", todoLabel = "Einrichten" } = labels;

  if (state === "busy") {
    pill.className = "pill muted";
    pill.textContent = "Prüfen...";
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

const LOCAL_LABELS = { doneLabel: "Ergänzen", todoLabel: "Anlegen" };

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
    // "Fehlende Einträge" statt "Fehlt": in dieser Liste steht links immer die
    // Bezeichnung und rechts der Wert. Ein Statuswort links liest sich, als
    // wäre die Liste rechts der Zustand.
    addFact(elements.localFacts, "Fehlende Einträge", missing.map((entry) => entry.path).join(", "));
  }

  setBlockState(elements.localPill, elements.localCreate, data.ok ? "ok" : "todo", LOCAL_LABELS);
}

// Wie die gemessenen Zustände heißen. Die Namen sagen, was jemand ohne
// git-Kenntnisse wissen muss — nicht, was git dazu getan hat.
const PRIVACY_LABELS = {
  private: "privat",
  public: "wird versioniert",
  partial: "teilweise privat",
  "pending-commit": "privat erst nach dem nächsten Commit",
  "no-vcs": "ohne Versionskontrolle",
  missing: "Verzeichnis nicht vorhanden",
  unknown: "nicht ermittelbar",
};

// Die beiden Zustände, die privat aussehen und keiner sind. Sie sind der Grund
// für diesen Block und werden deshalb als Warnung dargestellt.
const PRIVACY_WARNINGS = ["partial", "pending-commit"];

async function loadPrivate() {
  elements.privatePill.className = "pill muted";
  elements.privatePill.textContent = "Prüfen...";
  renderLoading(elements.privateMessage, "Zustand wird gemessen...");
  try {
    const response = await fetch("/api/local/private", { cache: "no-store" });
    renderPrivate(await response.json());
  } catch {
    elements.privateMessage.textContent = "Status konnte nicht geladen werden.";
  }
}

async function setPrivate(path, wantPrivate) {
  elements.privatePill.className = "pill muted";
  elements.privatePill.textContent = "Umschalten...";
  try {
    const response = await fetch("/api/local/private", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path, private: wantPrivate }),
    });
    renderPrivate(await response.json());
  } catch {
    elements.privateMessage.textContent = "Umschalten fehlgeschlagen.";
  }
}

function renderPrivate(data) {
  elements.privateEntries.replaceChildren();
  elements.privateMessage.textContent = data.message || "";

  const entries = data.entries || [];
  for (const entry of entries) {
    elements.privateEntries.append(privateEntryBox(entry));
  }

  if (entries.some((entry) => PRIVACY_WARNINGS.includes(entry.state))) {
    elements.privatePill.className = "pill warn";
    elements.privatePill.textContent = "Nicht wirklich privat";
    return;
  }
  elements.privatePill.className = "pill ok";
  elements.privatePill.textContent = "Gemessen";
}

// Ein Verzeichnis: Zustand, das Repository, auf das er sich bezieht, die
// auslösende Regel samt Quelle — und der Schalter, sofern k-playbook ihn
// einlösen kann.
function privateEntryBox(entry) {
  const box = document.createElement("div");
  box.className = "setting";

  const head = document.createElement("div");
  head.className = "setting-head";

  const titles = document.createElement("div");
  const title = document.createElement("div");
  title.className = "setting-title";
  title.textContent = `${entry.path}/`;
  const state = document.createElement("div");
  state.className = PRIVACY_WARNINGS.includes(entry.state) ? "setting-state warn" : "setting-state";
  state.textContent = privacyStateText(entry);
  titles.append(title, state);

  head.append(titles, privateAction(entry));
  box.append(head);

  // Ohne das Repository wäre die Aussage mehrdeutig: git nimmt das
  // nächstgelegene, und k-playbook-local kann ein eigenes sein.
  const facts = document.createElement("dl");
  facts.className = "facts";
  if (entry.repoRoot) {
    addFact(facts, "Repository", entry.repoRoot);
  }
  if (entry.rule) {
    const place = entry.rule.line ? `${entry.rule.source}:${entry.rule.line}` : entry.rule.source;
    addFact(facts, "Regel", `${entry.rule.pattern} — ${place}`);
  }
  if (facts.childElementCount > 0) {
    box.append(facts);
  }

  const note = privacyNote(entry);
  if (note) {
    const paragraph = document.createElement("p");
    paragraph.className = PRIVACY_WARNINGS.includes(entry.state) ? "setting-note warn" : "setting-note";
    paragraph.textContent = note;
    box.append(paragraph);
  }
  return box;
}

function privacyStateText(entry) {
  const label = PRIVACY_LABELS[entry.state] || entry.state;
  if (entry.state !== "partial") {
    return label;
  }

  const count = (entry.tracked || []).length;
  const files = count === 1 ? "1 Datei steht" : `${count} Dateien stehen`;
  return `${label}: ${files} weiterhin im Repository`;
}

// Was der Zustand für den Inhalt bedeutet. Bei den beiden unfertigen Zuständen
// stehen die betroffenen Dateien dabei — sie sind der Grund für die Warnung.
function privacyNote(entry) {
  switch (entry.state) {
    case "partial":
      return `Die Regel wirkt nur für neue Dateien. Im Repository stehen weiterhin: ${(entry.tracked || []).join(", ")}.`;
    case "pending-commit":
      return `Aus dem Index genommen, aber noch nicht committet — bis dahin bekommt jeder Clone: ${(entry.inHead || []).join(", ")}.`;
    default:
      return entry.reason || "";
  }
}

// Der Schalter steht nur da, wo k-playbook ihn einlösen kann. Stammt die Regel
// von woanders, steht an seiner Stelle die Quelle: geschrieben wird nur die
// eine verwaltete Datei.
function privateAction(entry) {
  if (!entry.canToggle) {
    const hint = document.createElement("p");
    hint.className = "hint setting-blocked";
    hint.textContent = entry.blocked || "";
    return hint;
  }

  const wantPrivate = entry.state === "public" || entry.state === "partial";
  const button = document.createElement("button");
  button.type = "button";
  button.className = entry.state === "partial" ? "primary attention-highlight" : "secondary";
  button.textContent = wantPrivate ? "Privat machen" : "Wieder versionieren";
  button.addEventListener("click", () => {
    if (window.confirm(privateConfirmText(entry, wantPrivate))) {
      setPrivate(entry.path, wantPrivate);
    }
  });
  return button;
}

// Vor dem Umschalten steht, was mit den Dateien passiert. Der Hinweis auf die
// Historie gehört dazu: kein Schalter dieser Oberfläche macht rückgängig, was
// bereits gepusht ist.
function privateConfirmText(entry, wantPrivate) {
  if (!wantPrivate) {
    return (
      `${entry.path}/ wird wieder versioniert: die von k-playbook verwaltete .gitignore wird entfernt.\n\n` +
      "Dateien, die früher aus dem Index genommen wurden, kommen dadurch nicht von selbst zurück."
    );
  }

  const parts = [
    `${entry.path}/ wird privat: k-playbook legt dort eine .gitignore an, die alles außer sich selbst und README.md heraushält.`,
  ];
  const tracked = entry.tracked || [];
  if (tracked.length > 0) {
    parts.push(`Aus dem Index genommen werden dabei: ${tracked.join(", ")}.`);
  } else {
    parts.push("Was dort bereits versioniert ist, wird aus dem Index genommen.");
  }
  parts.push("Die Dateien bleiben auf der Platte. Wirksam wird das erst mit dem nächsten Commit.");
  parts.push("Was bereits gepusht wurde, bleibt in der Historie — das macht kein Schalter rückgängig.");
  return parts.join("\n\n");
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

  // Die Wurzeldatei zuerst: sie ist der Einstiegspunkt, alles andere hängt
  // daran.
  const root = data.root || {};
  if (root.path) {
    const detail = !root.present
      ? "nicht vorhanden"
      : root.hasMarker
        ? "enthält den Anstoß"
        : "vorhanden, Anstoß fehlt";
    addFact(elements.assistantFacts, "AGENTS.md — Einstieg", detail);
  }

  for (const entry of data.entries || []) {
    const label = STATE_LABELS[entry.state] || entry.state;
    const term = entry.assistant ? `${entry.path} — ${entry.assistant}` : entry.path;
    addFact(elements.assistantFacts, term, entry.detail ? `${label} (${entry.detail})` : label);

    // Bei einem Katalog-Eintrag zählt nicht nur, dass etwas nicht stimmt,
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

// Wie die Zustände einer Registrierung heißen. Die ausführliche Fassung steht
// auf der Seite /mcp; hier genügt der Halbsatz neben dem Dateinamen.
const MCP_STATE_LABELS = {
  ok: "eingetragen",
  "no-wrapper": "Wrapper fehlt",
  "missing-file": "Datei nicht vorhanden",
  "missing-entry": "Eintrag fehlt",
  stale: "zeigt woandershin",
  unreadable: "kein lesbares JSON, bleibt unangetastet",
};

const MCP_LABELS = { doneLabel: "Erneut eintragen", todoLabel: "Einrichten" };

async function loadMCP() {
  setBlockState(elements.mcpPill, elements.mcpApply, "busy", MCP_LABELS);
  try {
    const response = await fetch("/api/mcp", { cache: "no-store" });
    renderMCP(await response.json());
  } catch {
    elements.mcpMessage.textContent = "Status konnte nicht geladen werden.";
  }
}

async function applyMCP() {
  setBlockState(elements.mcpPill, elements.mcpApply, "busy", MCP_LABELS);
  try {
    const response = await fetch("/api/mcp", { method: "POST" });
    renderMCP(await response.json());
  } catch {
    elements.mcpMessage.textContent = "Einrichten fehlgeschlagen.";
    setBlockState(elements.mcpPill, elements.mcpApply, "todo", MCP_LABELS);
  }
}

function renderMCP(data) {
  elements.mcpFacts.replaceChildren();
  elements.mcpMessage.textContent = data.message || "";

  if (data.command) {
    elements.mcpCommand.textContent = data.command;
  }

  for (const entry of data.entries || []) {
    const label = MCP_STATE_LABELS[entry.state] || entry.state;
    const term = entry.assistant ? `${entry.path} — ${entry.assistant}` : entry.path;
    addFact(elements.mcpFacts, term, entry.detail ? `${label} (${entry.detail})` : label);
  }

  // Die Bedingung steht immer im Text; deutlich wird sie erst, wenn schon die
  // Oberfläche nicht im Hauptverzeichnis gestartet wurde. Dann ist der Verdacht
  // messbar und keine Vorsichtsformel mehr.
  elements.mcpWorkdir.classList.toggle("hidden", !data.workdirMismatch);
  elements.mcpWorkdir.classList.toggle("warn", Boolean(data.workdirMismatch));

  // Ohne Installation gibt es nichts einzurichten, der Button bleibt grau —
  // ohne k-playbook/ ebenso: der eingetragene Wrapper existierte dann nicht.
  const environment = data.environment || {};
  const usable = environment.installed && environment.playbookPresent;
  const state = !usable ? "blocked" : data.ok ? "ok" : "todo";
  setBlockState(elements.mcpPill, elements.mcpApply, state, MCP_LABELS);
}

// Der Remediation-Block ist eine Einstellung, kein Einrichtungsschritt: die
// Auswahl wird sofort gespeichert, ein eigener Button wäre ein Zwischenschritt
// ohne Nutzen.
async function loadRemediation() {
  elements.remediationPill.className = "pill muted";
  elements.remediationPill.textContent = "Prüfen...";
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
    // current.mode trägt auch ohne Eintrag in der Datei den Standard.
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
  elements.remediationPill.textContent = current.prRequired ? "Nur über PR" : "Direkte Fixes möglich";

  // Der Standard gilt auch ohne Eintrag; das sollte sichtbar sein, damit
  // niemand einen ausdrücklich gewählten Wert vermutet.
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
  elements.ghPill.textContent = "Prüfen...";
  renderLoading(elements.ghMessage, "GitHub-CLI wird geprüft...");
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
    showGHCommand("gh ist auf diesem Rechner nicht installiert. Anleitung für den passenden Paketmanager:", commands.install);
    return;
  }

  if (!current.loggedIn) {
    elements.ghPill.className = "pill warn";
    elements.ghPill.textContent = "Nicht angemeldet";
    showGHCommand("Die Anmeldung läuft über den Browser und gehört ins Terminal:", commands.login);
    return;
  }

  elements.ghPill.className = "pill ok";
  elements.ghPill.textContent = current.account ? `Angemeldet als ${current.account}` : "Angemeldet";

  // Der Wechsel gilt für jedes Terminal und jedes Projekt auf diesem Rechner.
  // Deshalb steht er hier als Befehl und nicht als Knopf: wer ihn ausführt, tut
  // es bewusst und sieht danach das Ergebnis in seiner Shell.
  if ((current.accounts || []).length > 1) {
    showGHCommand(
      "Umschalten gilt für alle Terminals und Projekte auf diesem Rechner, nicht nur für dieses:",
      commands.switch,
    );
  }
}

function ghAccountLabel(current) {
  if (current.tokenFromEnv) {
    return current.account
      ? `${current.account}; zusätzlich ein Token in GH_TOKEN/GITHUB_TOKEN, das sticht`
      : "über GH_TOKEN/GITHUB_TOKEN, ohne Accountnamen";
  }
  if (!current.loggedIn) {
    return "kein Account hinterlegt";
  }
  // Gelesen aus der gh-Konfiguration, nicht beim Server geprüft.
  return `${current.account || "unbekannt"} (aus der gh-Konfiguration, Token nicht geprüft)`;
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
// Host verändert. Die Pill zeigt nur den Zustand.
async function loadTools() {
  elements.toolsPill.className = "pill muted";
  elements.toolsPill.textContent = "Prüfen...";
  // Jedes Tool wird einzeln aufgerufen; das dauert lange genug, dass der leere
  // Block sonst nach einem Ergebnis aussieht.
  renderLoading(elements.toolsMessage, "Security-Tools werden geprüft...");
  try {
    const response = await fetch("/api/tools", { cache: "no-store" });
    renderTools(await response.json());
  } catch {
    elements.toolsMessage.textContent = "Status konnte nicht geladen werden.";
  }
}

// Die Sprachauswahl ist das einzige Schreibbare in diesem Block. Sie gehört dem
// Projekt und landet in der K-PLAYBOOK.yaml; die Installation bleibt im Terminal.
async function setLanguages(languages) {
  elements.toolsPill.className = "pill muted";
  elements.toolsPill.textContent = "Speichern...";
  try {
    const response = await fetch("/api/languages", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ languages }),
    });
    renderTools(await response.json());
  } catch {
    elements.toolsMessage.textContent = "Speichern fehlgeschlagen.";
  }
}

function renderTools(data) {
  elements.toolsFacts.replaceChildren();
  elements.toolsLanguages.replaceChildren();
  elements.toolsMessage.textContent = data.message || "";
  elements.toolsCommand.classList.add("hidden");

  renderLanguageChoices(data);

  if (!data.available || data.message) {
    elements.toolsPill.className = "pill muted";
    elements.toolsPill.textContent = "Unbekannt";
    return;
  }

  // Ein sprachgebundenes Tool, das nicht zur Auswahl passt, ist nicht optional
  // im Sinne von "wäre schön", sondern schlicht nicht zuständig. Das muss
  // unterscheidbar bleiben, sonst sieht ein Python-Projekt lauter Lücken.
  const selected = new Set(data.languages || []);
  for (const tool of data.tools || []) {
    const languages = (tool.languages || "*").split(",").map((entry) => entry.trim());
    const universal = languages.includes("*");
    const relevant = universal || languages.some((entry) => selected.has(entry));

    let label = tool.name;
    if (!relevant) {
      label = `${tool.name} (${languages.join(", ")})`;
    } else if (!tool.required) {
      label = `${tool.name} (optional)`;
    }

    let detail;
    let missing = false;
    if (tool.status === "ok") {
      detail = tool.version || "vorhanden";
    } else if (relevant) {
      detail = `fehlt — ${tool.role}`;
      missing = true;
    } else {
      detail = `nicht gebraucht — ${tool.role}`;
    }
    const row = addFact(elements.toolsFacts, label, detail);
    if (missing) {
      row.classList.add("missing");
    }
  }
  if (data.binDir) {
    addFact(elements.toolsFacts, "Installationsort", data.binDir);
  }

  // Optionale Tools blockieren nichts, dürfen aber nicht unerwähnt bleiben:
  // sonst steht "fehlt" in der Liste und "Vollständig" darüber.
  const optional = data.missingOptional || 0;

  if (data.ok) {
    elements.toolsPill.className = optional > 0 ? "pill warn" : "pill ok";
    elements.toolsPill.textContent = optional > 0 ? "Pflicht vollständig" : "Vollständig";
    if (optional > 0) {
      elements.toolsMessage.textContent =
        `Alle Pflicht-Tools sind da. ${optional} optionale${optional === 1 ? "s fehlt" : " fehlen"}.`;
      // Sonst stünde hier kein Befehl: die Pflicht ist vollständig. Also der
      // Weg, der die optionalen mitnimmt.
      showToolsCommand(data.commandOptional);
    }
    return;
  }

  elements.toolsPill.className = "pill warn";
  elements.toolsPill.textContent = `${data.missing} fehlt`;
  // Fehlt Pflicht, gilt der Weg dafür. Was optional fehlt, kommt in der Meldung
  // dazu — zwei Befehle nebeneinander wären eine Wahl, die hier niemand treffen
  // muss.
  if (optional > 0) {
    elements.toolsMessage.textContent =
      `Dazu ${optional} optionale${optional === 1 ? "s Tool" : " Tools"}; die holt --include-optional mit.`;
  }
  showToolsCommand(data.command);
}

// showToolsCommand zeigt den Befehl, wenn es einen gibt. Beide Fassungen kommen
// fertig aus dem Preflight-Skript, samt Sprachauswahl — hier wird nichts
// zusammengesetzt.
function showToolsCommand(command) {
  if (!command) {
    return;
  }
  elements.toolsCommandText.textContent = command;
  elements.toolsCommand.classList.remove("hidden");
}

function renderLanguageChoices(data) {
  const available = data.availableLanguages || [];
  if (available.length === 0) {
    return;
  }
  const selected = new Set(data.languages || []);

  for (const language of available) {
    const isOn = selected.has(language);

    const label = document.createElement("label");
    label.className = isOn ? "choice selected" : "choice";

    const input = document.createElement("input");
    input.type = "checkbox";
    input.value = language;
    input.checked = isOn;
    input.addEventListener("change", () => {
      const next = new Set(selected);
      if (input.checked) {
        next.add(language);
      } else {
        next.delete(language);
      }
      setLanguages(available.filter((entry) => next.has(entry)));
    });

    const title = document.createElement("div");
    title.className = "choice-label";
    title.textContent = language;

    label.append(input, title);
    elements.toolsLanguages.append(label);
  }

  // Die Vorauswahl gilt auch ohne Eintrag in der Datei; das sollte sichtbar
  // sein, damit niemand eine ausdrückliche Entscheidung vermutet.
  if (!data.configured && !data.message) {
    elements.toolsMessage.textContent =
      "Vorauswahl, noch nicht in der Konfiguration festgehalten. Eine Auswahl schreibt sie fest.";
  }
}

// Der Workflow-Block führt nur weiter. Aufgelistet wird auf den Zielseiten;
// hier steht je Ziel bloß, wie viel dort liegt.
async function loadWorkflows() {
  try {
    const response = await fetch("/api/workflows", { cache: "no-store" });
    renderWorkflows(await response.json());
  } catch {
    elements.workflowsMessage.textContent = "Stand konnte nicht geladen werden.";
  }
}

function renderWorkflows(data) {
  elements.workflowsMessage.textContent = data.message || "";
  // Ohne belastbare Zahl bleibt der Knopf ein Knopf: er führt weiter, auch
  // wenn nicht feststeht, was dort liegt.
  const count = (value) => (data.available && !data.message ? `${value}` : "");
  elements.workflowsReviews.textContent = count(data.reviews);
  elements.workflowsTasks.textContent = count(data.tasks);
}

// Der Doku-Block listet die mitgelieferten Markdown-Dateien; gelesen wird eine
// davon erst auf Klick, in einem Fenster über der Seite.
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
  // wäre dafür die falsche Auskunft.
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
    // Der Titel steht auf dem Knopf, der Dateiname gehört trotzdem dazu.
    button.title = doc.path;
    button.dataset.path = doc.path;
    button.addEventListener("click", () => openDoc(doc.path, doc.title || doc.path));
    elements.docsList.append(button);
  }

  elements.docsPill.className = "pill ok";
  elements.docsPill.textContent = docs.length === 1 ? "1 Datei" : `${docs.length} Dateien`;
}

// Die offene Datei; Verweise darin werden relativ zu ihr aufgelöst.
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
    // Wurde inzwischen etwas anderes geöffnet, gehört diese Antwort nicht
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

// Verweise in der Doku zeigen überwiegend auf andere Dateien der Doku. Ohne
// eigene Behandlung würde ein Klick die Oberfläche verlassen — und mit ihr
// den Server, der an ihr hängt.
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

  // Ein Ziel mit Schema führt aus der Doku heraus und gehört in ein eigenes
  // Fenster.
  if (/^[a-z][a-z0-9+.-]*:/i.test(href)) {
    link.target = "_blank";
    link.rel = "noopener";
    return;
  }

  event.preventDefault();
  const [target, anchor] = splitAnchor(href);

  // Ein reiner Anker ohne Dateiname ist bereits oben abgefangen; bleibt ein
  // Verweis auf eine Datei. Alles außer Markdown kann diese Ansicht nicht
  // zeigen, der Pfad steht aber im Text und lässt sich im Editor öffnen.
  if (target.toLowerCase().endsWith(".md")) {
    openDoc(resolveDocPath(currentDocPath, target), "", anchor);
  }
}

// Löst einen Verweis gegen das Verzeichnis der offenen Datei auf; die
// URL-Klasse erledigt dabei "./" und "../".
function resolveDocPath(base, href) {
  const resolved = new URL(href, `https://docs.invalid/${base}`);
  return decodeURIComponent(resolved.pathname).replace(/^\//, "");
}

function splitAnchor(href) {
  const index = href.indexOf("#");
  return index === -1 ? [href, ""] : [href.slice(0, index), href.slice(index + 1)];
}

// Springt zu einer Überschrift der offenen Datei. Ohne Anker beginnt die
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

// Mermaid ist zu groß, um es mitzuliefern, und wird deshalb nur bei Bedarf vom
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

// Der Kontext-Block ist der einzige, der nicht beim Seitenaufbau lädt: seine
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
    // Beim nächsten Aufklappen darf es wieder versucht werden.
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
// und trägt die Sorten alphabetisch; hier steht die fachliche Reihenfolge.
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

  // Pfade werden gegen das Projektverzeichnis gekürzt: der gemeinsame Anfang
  // steht bereits oben und würde jede Zeile nur verlängern.
  const root = context.project.dir;
  addPathGroup("Instruktionen", context.instructions || [], root);

  const catalogs = context.catalogs || {};
  const kinds = Object.keys(CATALOG_LABELS).filter((kind) => kind in catalogs);
  // Eine später hinzukommende Sorte fällt nicht unter den Tisch.
  kinds.push(...Object.keys(catalogs).filter((kind) => !(kind in CATALOG_LABELS)));

  for (const kind of kinds) {
    addCatalogGroup(CATALOG_LABELS[kind] || kind, catalogs[kind] || [], root);
  }

  addPathGroup("Guidelines", context.guidelines || [], root);

  elements.contextPill.className = "pill ok";
  elements.contextPill.textContent = `Schema ${context.schemaVersion || "?"}`;
}

// Legt eine benannte Gruppe an und gibt deren Faktenliste zurück.
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
    // Welche Datei tatsächlich gilt, ist die eigentliche Auskunft des Overlays.
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

// Wie lange ein Kopier-Knopf die Rückmeldung stehen lässt.
const COPY_FEEDBACK_MS = 1500;

async function onCopyClick(event) {
  const button = event.target.closest("[data-copy]");
  if (!button) {
    return;
  }

  const source = document.getElementById(button.dataset.copy);
  if (!source) {
    return;
  }

  const done = await copyText(source.textContent);
  const label = button.textContent;
  button.textContent = done ? "Kopiert" : "Fehlgeschlagen";
  button.disabled = true;
  window.setTimeout(() => {
    button.textContent = label;
    button.disabled = false;
  }, COPY_FEEDBACK_MS);
}

// Die Zwischenablage-API gibt es nur im sicheren Kontext. 127.0.0.1 zählt
// dazu, aber nicht jeder Browser hält sich daran — deshalb der alte Weg als
// Rückfallebene.
async function copyText(text) {
  if (navigator.clipboard) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      // Weiter mit der Rückfallebene.
    }
  }

  const field = document.createElement("textarea");
  field.value = text;
  field.setAttribute("readonly", "");
  field.style.position = "fixed";
  field.style.opacity = "0";
  document.body.append(field);
  field.select();

  let done = false;
  try {
    done = document.execCommand("copy");
  } catch {
    done = false;
  }
  field.remove();
  return done;
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

// Baut das Menü aus den Blöcken selbst: Reihenfolge, Beschriftung und Status
// stehen bereits in den Karten, ein neuer Block braucht also nichts weiter als
// eine ID und eine Überschrift.
function buildBlockNav() {
  document.querySelectorAll(".blocks > .card").forEach((card) => {
    const heading = card.querySelector("h2");
    if (!card.id || !heading) {
      return;
    }

    const dot = document.createElement("span");
    dot.className = "block-nav-dot";
    const label = document.createElement("span");
    label.textContent = heading.textContent;

    const item = document.createElement("button");
    item.type = "button";
    item.className = "block-nav-item";
    item.append(dot, label);
    item.addEventListener("click", () => goToBlock(card, item));
    elements.blockNav.append(item);

    // Ob ein Block sichtbar ist und wie es um ihn steht, entscheiden die
    // Render-Funktionen an der Karte. Beobachten ist billiger, als in jeder
    // einzelnen zusätzlich das Menü nachzuziehen.
    const pill = card.querySelector(".section-head .pill");
    syncNavItem(card, item, dot, pill);
    const sync = () => syncNavItem(card, item, dot, pill);
    new MutationObserver(sync).observe(card, { attributes: true, attributeFilter: ["class"] });
    if (pill) {
      new MutationObserver(sync).observe(pill, { attributes: true, attributeFilter: ["class"] });
    }
  });
}

// Ein verborgener Block hat auch keinen Eintrag; der Punkt übernimmt die
// Statusfarbe seiner Pill, alles außerhalb von ok/warn/error bleibt neutral.
function syncNavItem(card, item, dot, pill) {
  item.classList.toggle("hidden", card.classList.contains("hidden"));
  const state = pill && ["ok", "warn", "error"].find((name) => pill.classList.contains(name));
  dot.className = state ? `block-nav-dot ${state}` : "block-nav-dot";
}

// Markiert wird der angeklickte Eintrag — er zeigt, wohin gesprungen wurde,
// und wandert beim Scrollen von Hand nicht mit.
function goToBlock(card, item) {
  // Ein zugeklappter Block wäre nach dem Sprung nur eine Kopfzeile. Das
  // Aufklappen löst über das toggle-Ereignis zugleich das Nachladen aus.
  if (card.tagName === "DETAILS") {
    card.open = true;
  }

  card.scrollIntoView({ behavior: "smooth", block: "start" });

  elements.blockNav.querySelectorAll(".block-nav-item.active").forEach((other) => {
    other.classList.remove("active");
    other.removeAttribute("aria-current");
  });
  item.classList.add("active");
  item.setAttribute("aria-current", "true");
}

function showClosed(message = "") {
  serverAvailable = false;
  elements.closedTitle.textContent = "Dieses Browserfenster kann jetzt geschlossen werden.";
  elements.closedMessage.textContent = message;
  elements.closedMessage.classList.toggle("hidden", !message);
  elements.closed.classList.remove("hidden");
}
