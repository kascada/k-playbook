"use strict";

// Seite "Status": die Startseite der Oberfläche. Sie zeigt, wie es um das
// Projekt steht: zuerst die Anbindung an die Kephalaion-Collection, dann die
// Umgebungsprüfung, darunter die Workflow-Karten und das Bild der
// Wissensablage.
//
// Nichts davon steht hier ein zweites Mal: Knöpfe und Sperrfläche bedient
// service.js, die Zahlen der Workflow-Karten holt workflows.js, und das Bild
// der Wissensablage holt und zeichnet docview.js. Klassisches Skript wie alle
// übrigen — jeder Name auf oberster Ebene teilt sich den Namensraum mit
// session.js, nav.js, service.js, docview.js und workflows.js.

// Die Datei, deren erstes Diagramm die Karte zeigt.
const STATUS_KNOWLEDGE_FILE = "knowledge-storage.md";

// Kurze Zustandswörter der Prüfung für die Marken; der volle Satz kommt vom
// Server (status.message).
const KEPHALAION_STATE_LABELS = {
  "ok": "Verbunden",
  "not-configured": "Nicht eingetragen",
  "invalid-entry": "Eintrag ungültig",
  "kephalaion-missing": "Kephalaion fehlt",
  "node-address": "Node-Adresse",
  "node-unreachable": "Node antwortet nicht",
  "token-missing": "Token fehlt",
  "hub-unknown": "Hub unbekannt",
  "hub-never-synced": "Nie abgeglichen",
  "login-invalid": "Anmeldung ungültig",
  "login-missing": "Anmeldung fehlt",
  "replica-unreadable": "Replica nicht lesbar",
  "collection-unreadable": "Collection nicht lesbar",
};

// Beschriftungen der Umgebungsprüfung: Stufe und Bereich kommen vom Server
// als Schlüssel (health.Level, health.Area); hier stehen die Wörter dazu. Ein
// unbekannter Bereich — eine neue Quelle — erscheint mit seinem Schlüssel.
const ENVIRONMENT_LEVEL_LABELS = {
  "error": "Fehler",
  "warning": "Warnung",
};

const ENVIRONMENT_AREA_LABELS = {
  "kephalaion": "Kephalaion",
  "gh": "GitHub CLI",
  "baseTools": "Basis-Werkzeuge",
  "versionSources": "Versionsinventar",
  "github": "GitHub-Zugänge",
};

const environmentElements = {
  pill: document.getElementById("environment-pill"),
  problems: document.getElementById("environment-problems"),
  message: document.getElementById("environment-message"),
};

const kephalaionElements = {
  statusPill: document.getElementById("status-kephalaion-pill"),
  statusFacts: document.getElementById("status-facts"),
  statusMessage: document.getElementById("status-kephalaion-message"),
  change: document.getElementById("kephalaion-change"),
  card: document.getElementById("kephalaion-setup-card"),
  setupPill: document.getElementById("kephalaion-setup-pill"),
  localFile: document.getElementById("kephalaion-local-file"),
  reason: document.getElementById("kephalaion-setup-reason"),
  form: document.getElementById("kephalaion-setup-form"),
  pairs: document.getElementById("kephalaion-pairs"),
  collections: document.getElementById("kephalaion-collections"),
  test: document.getElementById("kephalaion-test"),
  save: document.getElementById("kephalaion-save"),
  cancel: document.getElementById("kephalaion-cancel"),
  admin: document.getElementById("kephalaion-admin"),
  adminSteps: document.getElementById("kephalaion-admin-steps"),
  message: document.getElementById("kephalaion-setup-message"),
};

// Der Stand des Einrichtungsschritts: die Kandidaten, die Auswahl und die
// Auswahl, für die zuletzt ein Test bestanden wurde. Eintragen geht nur für
// genau diese.
const kephalaionSetup = {
  pairs: [],
  hub: "",
  account: "",
  collection: "",
  passed: "",
  current: null,
};

// Muss vor den Ladefunktionen laufen: die blenden Blöcke ein, und das Menü
// zieht das nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();

startSession(showClosed);

kephalaionElements.test.addEventListener("click", () => submitKephalaion("test"));
kephalaionElements.save.addEventListener("click", () => submitKephalaion("save"));
kephalaionElements.change.addEventListener("click", () => openKephalaionSetup(true));
kephalaionElements.cancel.addEventListener("click", closeKephalaionSetup);

loadKephalaion();
loadEnvironment();
loadWorkflowCounts(document.getElementById("status-workflows-message"));
loadKnowledgePicture();

// Die Umgebungsprüfung: der Report aus /api/environment. Ohne Problem eine
// Zeile, sonst die Liste; die Marke der Karte sagt, wie viele Fehler oder
// Warnungen es sind, und färbt darüber den Punkt im Blockmenü.
async function loadEnvironment() {
  const els = environmentElements;
  renderKephalaionLoading(els.message, "Umgebung wird geprüft...");
  let data;
  try {
    const response = await fetch("/api/environment", { cache: "no-store" });
    data = await response.json();
  } catch {
    els.pill.className = "pill error";
    els.pill.textContent = "Nicht geprüft";
    els.message.textContent = "Die Umgebungsprüfung konnte nicht geladen werden.";
    return;
  }
  renderEnvironment(data);
}

function renderEnvironment(data) {
  const els = environmentElements;
  els.problems.replaceChildren();
  els.message.textContent = "";
  if (!data.available || !data.health) {
    els.pill.className = "pill muted";
    els.pill.textContent = "Kein Projekt";
    els.message.textContent = data.message || "";
    return;
  }
  const problems = data.health.problems || [];
  if (problems.length === 0) {
    els.pill.className = "pill ok";
    els.pill.textContent = "Keine Probleme";
    els.message.textContent = "Die Umgebung dieses Projekts ist in Ordnung: keine Fehler, keine Warnungen.";
    return;
  }
  const errors = problems.filter((problem) => problem.level === "error").length;
  const warnings = problems.length - errors;
  els.pill.className = errors > 0 ? "pill error" : "pill warn";
  els.pill.textContent = environmentCountLabel(errors, warnings);
  for (const problem of problems) {
    els.problems.append(renderEnvironmentProblem(problem));
  }
}

// „2 Fehler, 1 Warnung“ — nur die Zahlen, die größer als null sind.
function environmentCountLabel(errors, warnings) {
  const parts = [];
  if (errors > 0) {
    parts.push(errors === 1 ? "1 Fehler" : `${errors} Fehler`);
  }
  if (warnings > 0) {
    parts.push(warnings === 1 ? "1 Warnung" : `${warnings} Warnungen`);
  }
  return parts.join(", ");
}

// Ein Eintrag: Stufe als Zustandsmarke und Bereich in der Kopfzeile, darunter
// der Satz und die Abhilfe. Alles Text aus der Antwort steht in textContent.
function renderEnvironmentProblem(problem) {
  const item = document.createElement("li");
  item.className = "environment-item";

  const head = document.createElement("div");
  head.className = "environment-head";
  const level = document.createElement("span");
  level.className = problem.level === "error" ? "pill error" : "pill warn";
  level.textContent = ENVIRONMENT_LEVEL_LABELS[problem.level] || problem.level || "Unbekannt";
  const area = document.createElement("span");
  area.className = "environment-area";
  area.textContent = ENVIRONMENT_AREA_LABELS[problem.area] || problem.area || "";
  head.append(level, area);

  const message = document.createElement("p");
  message.className = "environment-text";
  message.textContent = problem.message || "";

  item.append(head, message);
  if (problem.hint) {
    const hint = document.createElement("p");
    hint.className = "environment-hint";
    hint.textContent = `Abhilfe: ${problem.hint}`;
    item.append(hint);
  }
  return item;
}

async function loadKnowledgePicture() {
  const viewer = document.getElementById("knowledge-picture");
  const message = document.getElementById("knowledge-picture-message");
  try {
    const data = await loadDocInto(viewer, STATUS_KNOWLEDGE_FILE, { firstDiagramOnly: true });
    message.textContent = data.available ? data.message || "" : "Keine Projektkonfiguration gefunden.";
  } catch {
    viewer.textContent = "";
    message.textContent = "Das Bild der Wissensablage konnte nicht geladen werden.";
  }
}

function kephalaionLabel(state) {
  return KEPHALAION_STATE_LABELS[state] || state || "Unbekannt";
}

function kephalaionPillClass(state) {
  if (state === "ok") {
    return "pill ok";
  }
  if (state === "not-configured") {
    return "pill warn";
  }
  return "pill error";
}

// Ein drehender Ring mit Text im Meldungsfeld, wie renderLoading in app.js —
// das auf dieser Seite nicht geladen ist. Die nächste Ausgabe ersetzt ihn.
function renderKephalaionLoading(element, text) {
  const loading = document.createElement("span");
  loading.className = "loading-inline";
  const spinner = document.createElement("span");
  spinner.className = "loading-spinner";
  spinner.setAttribute("aria-hidden", "true");
  loading.append(spinner, text);
  element.replaceChildren(loading);
}

function addStatusFact(list, term, detail) {
  const row = document.createElement("div");
  const dt = document.createElement("dt");
  dt.textContent = term;
  const dd = document.createElement("dd");
  dd.textContent = detail;
  row.append(dt, dd);
  list.append(row);
  return row;
}

// Die Anbindung: Anzeige im Statusfeld und, wenn sie nicht trägt, der
// Einrichtungsschritt ganz oben.
async function loadKephalaion() {
  const els = kephalaionElements;
  renderKephalaionLoading(els.statusMessage, "Kephalaion wird geprüft...");
  let data;
  try {
    const response = await fetch("/api/kephalaion", { cache: "no-store" });
    data = await response.json();
  } catch {
    els.statusPill.className = "pill error";
    els.statusPill.textContent = "Nicht geprüft";
    els.statusMessage.textContent = "Die Anbindung an Kephalaion konnte nicht geladen werden.";
    return;
  }
  renderKephalaionStatus(data);
  if (data.available && data.status && data.status.state !== "ok") {
    openKephalaionSetup(false);
  }
}

function renderKephalaionStatus(data) {
  const els = kephalaionElements;
  els.statusFacts.replaceChildren();
  els.statusMessage.textContent = "";
  if (!data.available || !data.status) {
    els.statusPill.className = "pill muted";
    els.statusPill.textContent = "Kein Projekt";
    els.statusMessage.textContent = data.message || "";
    els.change.hidden = true;
    return;
  }
  const status = data.status;
  kephalaionSetup.current = status;
  if (data.localFile) {
    els.localFile.textContent = data.localFile;
  }
  els.statusPill.className = kephalaionPillClass(status.state);
  els.statusPill.textContent = kephalaionLabel(status.state);

  const collection = addStatusFact(els.statusFacts, "Collection", status.address || "nicht eingetragen");
  if (!status.ok) {
    collection.classList.add("missing");
  }
  let account = status.account || "–";
  if (status.user) {
    account += ` (User ${status.user})`;
  }
  addStatusFact(els.statusFacts, "Account", account);
  if (status.ok) {
    addStatusFact(els.statusFacts, "Rechte", (status.rights || []).join(", ") || "–");
    if (status.sync) {
      const parts = [];
      if (status.sync.lastSuccess) {
        parts.push(`zuletzt ${formatKephalaionTime(status.sync.lastSuccess)}`);
      }
      if (status.sync.revision !== undefined) {
        parts.push(`Revision ${status.sync.revision}`);
      }
      if (status.sync.lastError) {
        parts.push(`letzter Fehler: ${status.sync.lastError}`);
      }
      addStatusFact(els.statusFacts, "Abgleich", parts.join(", ") || "–");
    }
  }
  els.statusMessage.textContent = status.message || "";
  // Ohne tragende Anbindung steht der Einrichtungsschritt ohnehin offen.
  els.change.hidden = !status.ok;
}

function formatKephalaionTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("de-DE", { dateStyle: "short", timeStyle: "short" });
}

// Öffnet den Einrichtungsschritt. byChoice heißt: über „Collection ändern“
// geöffnet, bei bestehender Anbindung — dann gibt es „Abbrechen“.
async function openKephalaionSetup(byChoice) {
  const els = kephalaionElements;
  els.card.classList.remove("hidden");
  els.cancel.hidden = !byChoice;
  const current = kephalaionSetup.current;
  els.setupPill.className = current ? kephalaionPillClass(current.state) : "pill muted";
  els.setupPill.textContent = current ? kephalaionLabel(current.state) : "Prüfen...";
  els.reason.textContent = current && current.state !== "ok" ? current.message || "" : "";
  if (byChoice) {
    els.card.scrollIntoView({ behavior: "smooth", block: "start" });
  }
  await loadKephalaionCandidates();
}

function closeKephalaionSetup() {
  kephalaionElements.card.classList.add("hidden");
}

async function loadKephalaionCandidates() {
  const els = kephalaionElements;
  els.form.hidden = true;
  els.admin.hidden = true;
  renderKephalaionLoading(els.message, "Hub und Accounts dieses Rechners werden gesucht...");
  let data;
  try {
    const response = await fetch("/api/kephalaion/candidates", { cache: "no-store" });
    data = await response.json();
  } catch {
    els.message.textContent = "Die Kandidaten konnten nicht geladen werden.";
    return;
  }
  renderKephalaionCandidates(data);
}

function renderKephalaionCandidates(data) {
  const els = kephalaionElements;
  els.message.textContent = "";
  const notes = [];
  if (!data.available) {
    els.message.textContent = data.message || "";
    return;
  }
  if (data.configProblem) {
    notes.push(data.configProblem);
  } else if (!data.endpoint) {
    notes.push(`Keine zulässige Adresse des Nodes: ${data.addressProblem || "unbekannt"}`);
  }
  if (data.configPresent && !data.configProblem && data.configSource && data.configSource !== "user") {
    // Die config gilt nicht beim User, die Tokens liegen aber dort: Beide
    // Orte getrennt nennen, damit niemand die Tokens neben der globalen
    // config sucht.
    const source = data.configSource === "env" ? "aus KEPHALAION_CONFIG" : "global";
    notes.push(`Es gilt die config ${data.configPath} (${source}); die Token-Dateien liegen beim User unter ${data.tokensDir || "~/.config/kephalaion/tokens"}.`);
  }
  for (const pending of data.pending || []) {
    notes.push(`Für Account ${pending.account} am Hub ${pending.hub} liegt nur ein Token, das noch auf den Tausch wartet: `
      + `kephalaion node account check ${pending.hub} ${pending.account}`);
  }

  kephalaionSetup.pairs = data.pairs || [];
  const preselect = data.preselect || {};
  kephalaionSetup.hub = preselect.hub || "";
  kephalaionSetup.account = preselect.account || "";
  kephalaionSetup.collection = preselect.collection || "";
  kephalaionSetup.passed = "";
  kephalaionSetup.projectName = data.projectName || "";

  const usable = kephalaionSetup.pairs.some((pair) => pair.state === "ok");
  // Antwortet der Node nicht, fehlt nichts am Hub: dann ist der Dienst zu
  // starten, nicht ein Admin zu fragen.
  const nodeDown = kephalaionSetup.pairs.length > 0
    && kephalaionSetup.pairs.every((pair) => pair.state === "node-unreachable");
  if (nodeDown) {
    notes.push("Der Node dieses Rechners antwortet nicht. Kephalaion starten (kephalaion serve oder der eingerichtete "
      + "User-Dienst), dann die Seite neu laden.");
  }
  if (kephalaionSetup.pairs.length === 0 || (!usable && !nodeDown)) {
    els.admin.hidden = false;
    els.adminSteps.replaceChildren();
    for (const step of data.adminSteps || []) {
      const item = document.createElement("li");
      item.textContent = step;
      els.adminSteps.append(item);
    }
  }
  if (kephalaionSetup.pairs.length > 0) {
    els.form.hidden = false;
    renderKephalaionPairs();
  } else {
    notes.push(`Keine Token-Datei unter ${data.tokensDir || "~/.config/kephalaion/tokens"}/<hub>/<account>.token gefunden.`);
  }
  els.message.textContent = notes.join("\n");
}

function selectedPair() {
  return kephalaionSetup.pairs.find((pair) => pair.hub === kephalaionSetup.hub && pair.account === kephalaionSetup.account);
}

function selectionKey() {
  return `${kephalaionSetup.hub}/${kephalaionSetup.account}/${kephalaionSetup.collection}`;
}

function makeChoice(name, value, selected, title, description, onChange) {
  const label = document.createElement("label");
  label.className = selected ? "choice selected" : "choice";
  const input = document.createElement("input");
  input.type = "radio";
  input.name = name;
  input.value = value;
  input.checked = selected;
  input.addEventListener("change", onChange);
  const text = document.createElement("div");
  const heading = document.createElement("div");
  heading.className = "choice-label";
  heading.textContent = title;
  const detail = document.createElement("p");
  detail.className = "choice-description";
  detail.textContent = description;
  text.append(heading, detail);
  label.append(input, text);
  return label;
}

function renderKephalaionPairs() {
  const els = kephalaionElements;
  els.pairs.replaceChildren();
  for (const pair of kephalaionSetup.pairs) {
    const selected = pair.hub === kephalaionSetup.hub && pair.account === kephalaionSetup.account;
    const description = pair.message;
    els.pairs.append(makeChoice("kephalaion-pair", `${pair.hub}/${pair.account}`, selected,
      `Hub ${pair.hub} · Account ${pair.account}`, description, () => {
        kephalaionSetup.hub = pair.hub;
        kephalaionSetup.account = pair.account;
        const names = (pair.collections || []).map((c) => c.collection);
        if (!names.includes(kephalaionSetup.collection)) {
          kephalaionSetup.collection = names.includes(kephalaionSetup.projectName)
            ? kephalaionSetup.projectName
            : names.length === 1 ? names[0] : "";
        }
        renderKephalaionPairs();
      }));
  }
  renderKephalaionCollections();
}

function renderKephalaionCollections() {
  const els = kephalaionElements;
  els.collections.replaceChildren();
  const pair = selectedPair();
  const collections = pair ? pair.collections || [] : [];
  if (collections.length === 0) {
    const empty = document.createElement("p");
    empty.className = "hint";
    empty.textContent = pair && pair.state === "ok"
      ? "Dieser Account kann keine Collection lesen. Ein Admin gibt sie ihm am Hub."
      : "Für dieses Paar meldet der Node keine lesbaren Collections.";
    els.collections.append(empty);
  }
  for (const collection of collections) {
    const selected = collection.collection === kephalaionSetup.collection;
    const parts = [`Rechte: ${(collection.rights || []).join(", ")}`];
    if (collection.collection === kephalaionSetup.projectName) {
      parts.push("wie das Projekt benannt");
    }
    els.collections.append(makeChoice("kephalaion-collection", collection.collection, selected,
      collection.address, parts.join(" · "), () => {
        kephalaionSetup.collection = collection.collection;
        renderKephalaionCollections();
      }));
  }
  updateKephalaionButtons();
}

function updateKephalaionButtons() {
  const els = kephalaionElements;
  const complete = kephalaionSetup.hub && kephalaionSetup.account && kephalaionSetup.collection;
  els.test.disabled = !complete;
  els.save.disabled = !complete || kephalaionSetup.passed !== selectionKey();
  els.save.classList.toggle("attention-highlight", !els.save.disabled);
}

// Testen schreibt nichts; Eintragen testet am Server erneut und schreibt nur
// bei ok. Beide schicken nur die drei Namen.
async function submitKephalaion(action) {
  const els = kephalaionElements;
  const key = selectionKey();
  els.test.disabled = true;
  els.save.disabled = true;
  renderKephalaionLoading(els.message, action === "save" ? "Wird getestet und eingetragen..." : "Wird getestet...");
  let data;
  try {
    const response = await fetch(`/api/kephalaion/${action}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        hub: kephalaionSetup.hub,
        account: kephalaionSetup.account,
        collection: kephalaionSetup.collection,
      }),
    });
    data = await response.json();
  } catch {
    els.message.textContent = action === "save" ? "Eintragen fehlgeschlagen." : "Test fehlgeschlagen.";
    updateKephalaionButtons();
    return;
  }
  const status = data.status || {};
  els.setupPill.className = kephalaionPillClass(status.state);
  els.setupPill.textContent = kephalaionLabel(status.state);
  if (action === "test") {
    kephalaionSetup.passed = status.ok ? key : "";
    // Bestanden heißt noch nicht eingetragen: die Marke sagt das.
    if (status.ok) {
      els.setupPill.textContent = "Test bestanden";
    }
    els.message.textContent = status.ok
      ? `Test bestanden: ${status.message} Jetzt eintragen.`
      : `Test nicht bestanden: ${status.message || data.message || ""}`;
    updateKephalaionButtons();
    return;
  }
  if (!data.saved) {
    kephalaionSetup.passed = "";
    els.message.textContent = [data.message, status.message].filter(Boolean).join("\n");
    updateKephalaionButtons();
    return;
  }
  els.message.textContent = data.message || "Eingetragen.";
  kephalaionSetup.passed = "";
  updateKephalaionButtons();
  await loadKephalaion();
  if (kephalaionSetup.current && kephalaionSetup.current.ok) {
    closeKephalaionSetup();
  }
}
