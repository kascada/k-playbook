"use strict";

// Seite "Status": die Startseite der Oberfläche. Sie zeigt, wie es um das
// Projekt steht: zuerst die Anbindung an die Kephalaion-Collection, dann die
// Umgebungsprüfung, darunter die Workflow-Karten.
//
// Nichts davon steht hier ein zweites Mal: Knöpfe und Sperrfläche bedient
// service.js, und die Zahlen der Workflow-Karten holt workflows.js.
// Klassisches Skript wie alle übrigen — jeder Name auf oberster Ebene teilt
// sich den Namensraum mit session.js, nav.js, service.js und workflows.js.

// Kurze Zustandswörter der Prüfung für die Marken; der volle Satz kommt vom
// Server (status.message).
const KEPHALAION_STATE_LABELS = {
  "ok": "Verbunden",
  "not-configured": "Nicht eingetragen",
  "invalid-entry": "Eintrag ungültig",
  "kephalaion-missing": "Kephalaion fehlt",
  "client-failed": "Auskunft fehlt",
  "node-address": "Node-Adresse",
  "account-choice-missing": "Account nicht gewählt",
  "node-unreachable": "Node antwortet nicht",
  "token-missing": "Token fehlt",
  "hub-unknown": "Hub unbekannt",
  "hub-never-synced": "Nie abgeglichen",
  "login-invalid": "Anmeldung ungültig",
  "login-missing": "Anmeldung fehlt",
  "replica-unreadable": "Replica nicht lesbar",
  "collection-unreadable": "Collection nicht lesbar",
};

// Woher der Account kommt (choice der Auskunft kephalaion node client).
const KEPHALAION_CHOICE_LABELS = {
  "override": "Überschreibung",
  "default": "Standard",
  "only": "einzige Token-Datei",
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
  "baseTools": "Basis-Werkzeuge",
  "versionSources": "Versionsinventar",
  "github": "GitHub-Zugänge",
  "share": "Mitgelieferte Dateien",
  "anchor": "Anker",
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
  configFile: document.getElementById("kephalaion-config-file"),
  reason: document.getElementById("kephalaion-setup-reason"),
  form: document.getElementById("kephalaion-setup-form"),
  hubs: document.getElementById("kephalaion-hubs"),
  collections: document.getElementById("kephalaion-collections"),
  test: document.getElementById("kephalaion-test"),
  save: document.getElementById("kephalaion-save"),
  cancel: document.getElementById("kephalaion-cancel"),
  admin: document.getElementById("kephalaion-admin"),
  adminSteps: document.getElementById("kephalaion-admin-steps"),
  message: document.getElementById("kephalaion-setup-message"),
};

// Der Stand des Einrichtungsschritts: die Hubs aus der Auskunft, die Auswahl
// und die Auswahl, für die zuletzt ein Test bestanden wurde. Eintragen geht
// nur für genau diese. Einen Account wählt hier niemand: Er kommt von
// Kephalaion.
const kephalaionSetup = {
  hubs: [],
  hub: "",
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
  if (data.configFile) {
    els.configFile.textContent = data.configFile;
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
  if (status.choice) {
    account += ` · ${KEPHALAION_CHOICE_LABELS[status.choice] || status.choice}`;
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
  renderKephalaionLoading(els.message, "Hubs und Accounts dieses Rechners werden bei Kephalaion erfragt...");
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
  if (data.configFile) {
    els.configFile.textContent = data.configFile;
  }
  if (data.clientProblem) {
    // Ohne Auskunft gibt es weder Hubs noch Accounts; der Satz nennt die
    // Abhilfe (Installation, kephalaion upgrade oder den Grund).
    els.message.textContent = data.clientProblem;
    return;
  }
  const config = data.configPath ? `${data.configPath} (Quelle ${data.configSource})` : "keine";
  notes.push(`Kephalaion auf diesem Rechner: config ${config} · Node ${data.nodeUrl || "–"} · Token-Verzeichnis ${data.tokensDir || "–"}.`);
  if (data.configProblem) {
    notes.push(`Die config ist nicht lesbar (${data.configProblem}); für den Node gilt die Standardadresse.`);
  }

  kephalaionSetup.hubs = data.hubs || [];
  const preselect = data.preselect || {};
  kephalaionSetup.hub = preselect.hub || "";
  kephalaionSetup.collection = preselect.collection || "";
  kephalaionSetup.passed = "";
  kephalaionSetup.projectName = data.projectName || "";

  const usable = kephalaionSetup.hubs.some((hub) => hub.state === "ok");
  // Antwortet der Node nicht, fehlt nichts am Hub: dann ist der Dienst zu
  // starten, nicht ein Admin zu fragen.
  const nodeDown = kephalaionSetup.hubs.length > 0
    && kephalaionSetup.hubs.every((hub) => hub.state === "node-unreachable");
  if (nodeDown) {
    notes.push("Der Node dieses Rechners antwortet nicht. Kephalaion starten (kephalaion serve oder der eingerichtete "
      + "User-Dienst), dann die Seite neu laden.");
  }
  if (kephalaionSetup.hubs.length === 0 || (!usable && !nodeDown)) {
    els.admin.hidden = false;
    els.adminSteps.replaceChildren();
    for (const step of data.adminSteps || []) {
      const item = document.createElement("li");
      item.textContent = step;
      els.adminSteps.append(item);
    }
  }
  if (kephalaionSetup.hubs.length > 0) {
    els.form.hidden = false;
    renderKephalaionHubs();
  } else {
    notes.push(`Keine Token-Datei unter ${data.tokensDir || "~/.config/kephalaion/tokens"}/<hub>/<account>.token gefunden.`);
  }
  els.message.textContent = notes.join("\n");
}

function selectedHub() {
  return kephalaionSetup.hubs.find((hub) => hub.hub === kephalaionSetup.hub);
}

function selectionKey() {
  return `${kephalaionSetup.hub}/${kephalaionSetup.collection}`;
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

function renderKephalaionHubs() {
  const els = kephalaionElements;
  els.hubs.replaceChildren();
  for (const hub of kephalaionSetup.hubs) {
    const selected = hub.hub === kephalaionSetup.hub;
    const who = hub.account
      ? `Account ${hub.account} (${KEPHALAION_CHOICE_LABELS[hub.choice] || hub.choice || "gewählt"})`
      : "kein Account gewählt";
    els.hubs.append(makeChoice("kephalaion-hub", hub.hub, selected, `Hub ${hub.hub} · ${who}`, hub.message, () => {
      kephalaionSetup.hub = hub.hub;
      const names = (hub.collections || []).map((c) => c.collection);
      if (!names.includes(kephalaionSetup.collection)) {
        kephalaionSetup.collection = names.includes(kephalaionSetup.projectName)
          ? kephalaionSetup.projectName
          : names.length === 1 ? names[0] : "";
      }
      renderKephalaionHubs();
    }));
  }
  renderKephalaionCollections();
}

function renderKephalaionCollections() {
  const els = kephalaionElements;
  els.collections.replaceChildren();
  const hub = selectedHub();
  const collections = hub ? hub.collections || [] : [];
  if (collections.length === 0) {
    const empty = document.createElement("p");
    empty.className = "hint";
    empty.textContent = hub && hub.state === "ok"
      ? "Dieser Account kann keine Collection lesen. Ein Admin gibt sie ihm am Hub."
      : "Für diesen Hub meldet der Node keine lesbaren Collections.";
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
  const complete = kephalaionSetup.hub && kephalaionSetup.collection;
  els.test.disabled = !complete;
  els.save.disabled = !complete || kephalaionSetup.passed !== selectionKey();
  els.save.classList.toggle("attention-highlight", !els.save.disabled);
}

// Testen schreibt nichts; Eintragen testet am Server erneut und schreibt nur
// bei ok. Beide schicken nur Hub und Collection; den Account nennt Kephalaion.
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
