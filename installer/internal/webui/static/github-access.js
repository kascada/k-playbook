"use strict";

// Seite "GitHub-Zugänge": die persönliche Zugangsdatei, ihre Dienste mit der
// Verfügbarkeit jedes Zugangs, und der Test gegen GitHub auf Knopfdruck.
//
// Beim Laden fragt die Seite nur GET /api/github-access — Lesen und
// Verfügbarkeit ohne Netz. GitHub wird erst gefragt, wenn jemand „Prüfen“
// oder „Alle prüfen“ drückt: je Dienst ein eigener POST /api/github-access/test
// mit service, damit sich die Blöcke einzeln füllen und ein langsamer Dienst
// die anderen nicht aufhält. Geschrieben wird nichts: die Datei gehört in den
// Editor, die Seite sagt, wo sie liegt. Alles aus den Antworten steht in
// textContent — kein Wert wird als HTML gedeutet.

buildBlockNav();

startSession((message) => {
  document.getElementById("access-file-message").textContent = message;
});

const elements = {
  filePill: document.getElementById("access-file-pill"),
  fileFacts: document.getElementById("access-file-facts"),
  problems: document.getElementById("access-problems"),
  fileMessage: document.getElementById("access-file-message"),
  servicesPill: document.getElementById("access-services-pill"),
  testAll: document.getElementById("access-test-all"),
  services: document.getElementById("access-services"),
  servicesMessage: document.getElementById("access-services-message"),
};

const STATE_LABEL = {
  available: "verfügbar",
  unavailable: "nicht verfügbar",
  disabled: "deaktiviert",
};

const STATE_TONE = {
  available: "ok",
  unavailable: "warn",
  disabled: "muted",
};

const RESULT_LABEL = {
  ok: "ok",
  failed: "fehlgeschlagen",
  skipped: "übersprungen",
};

const RESULT_TONE = {
  ok: "ok",
  failed: "error",
  skipped: "muted",
};

const ANSWER_LABEL = { yes: "ja", no: "nein", unknown: "unbekannt" };

// serviceBlocks hält je Dienst die Elemente, die ein Testergebnis füllt.
const serviceBlocks = new Map();

elements.testAll.addEventListener("click", testAll);

loadAccess();

function setPill(pill, tone, text) {
  pill.className = `pill ${tone}`;
  pill.textContent = text;
}

function pill(tone, text) {
  const mark = document.createElement("span");
  mark.className = `pill ${tone}`;
  mark.textContent = text;
  return mark;
}

function fact(list, term, detail) {
  const row = document.createElement("div");
  const dt = document.createElement("dt");
  dt.textContent = term;
  const dd = document.createElement("dd");
  if (typeof detail === "string") {
    dd.textContent = detail;
  } else {
    dd.append(detail);
  }
  row.append(dt, dd);
  list.append(row);
}

async function loadAccess() {
  let data;
  try {
    const response = await fetch("/api/github-access", { cache: "no-store" });
    data = await response.json();
  } catch {
    setPill(elements.filePill, "error", "Fehler");
    elements.fileMessage.textContent = "Die Zugangsdatei konnte nicht geladen werden.";
    setPill(elements.servicesPill, "error", "Fehler");
    return;
  }
  renderFile(data);
  renderServices(data);
}

// Die Karte „Datei“: Pfad, Zahl der Dienste, Dienste ohne verfügbaren
// Zugang; die Probleme des Lesers als Liste wie in der Umgebungsprüfung.
function renderFile(data) {
  elements.fileFacts.replaceChildren();
  elements.problems.replaceChildren();
  elements.fileMessage.textContent = "";

  if (data.file) {
    const code = document.createElement("code");
    code.textContent = data.file;
    fact(elements.fileFacts, "Datei", code);
  }
  if (!data.available) {
    setPill(elements.filePill, "error", "Nicht lesbar");
    elements.fileMessage.textContent = data.message || "Die Datei ist nicht lesbar.";
    for (const problem of data.problems || []) {
      elements.problems.append(renderProblem(problem));
    }
    return;
  }

  const services = Object.entries(data.services || {});
  const unusable = services.filter(([, service]) => !usable(service)).length;
  fact(elements.fileFacts, "Dienste", countLabel(services.length, "Dienst", "Dienste", "keine Dienste"));
  if (services.length) {
    fact(elements.fileFacts, "Ohne verfügbaren Zugang", unusable ? pill("warn", String(unusable)) : "keiner");
  }

  const problems = data.problems || [];
  const errors = problems.filter((problem) => problem.level === "error").length;
  if (!problems.length) {
    setPill(elements.filePill, "ok", "Keine Probleme");
    elements.fileMessage.textContent = data.created
      ? "Die Datei wurde gerade als Vorlage angelegt — trage deine Dienste im Editor ein."
      : services.length
        ? "Die Datei liest sich ohne Probleme."
        : "Die Datei liest sich ohne Probleme; noch ist kein Dienst eingetragen.";
    return;
  }
  setPill(elements.filePill, errors ? "error" : "warn", problemCountLabel(errors, problems.length - errors));
  for (const problem of problems) {
    elements.problems.append(renderProblem(problem));
  }
}

function usable(service) {
  return (service.access || []).some((access) => access.state === "available");
}

function countLabel(count, one, many, none) {
  if (count === 0) {
    return none;
  }
  return count === 1 ? `1 ${one}` : `${count} ${many}`;
}

function problemCountLabel(errors, warnings) {
  const parts = [];
  if (errors) {
    parts.push(errors === 1 ? "1 Fehler" : `${errors} Fehler`);
  }
  if (warnings) {
    parts.push(warnings === 1 ? "1 Warnung" : `${warnings} Warnungen`);
  }
  return parts.join(", ");
}

// Ein Problem des Lesers: Stufe und Pfad in der Datei, darunter Satz und
// Abhilfe — dieselbe Form wie ein Eintrag der Umgebungsprüfung.
function renderProblem(problem) {
  const item = document.createElement("li");
  item.className = "environment-item";
  const head = document.createElement("div");
  head.className = "environment-head";
  head.append(pill(problem.level === "error" ? "error" : "warn", problem.level === "error" ? "Fehler" : "Warnung"));
  const path = document.createElement("span");
  path.className = "environment-area";
  path.textContent = problem.path || "";
  head.append(path);
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

// Die Karte „Dienste“: je Dienst ein Block mit Name, Repository, Knopf und
// einer Zeile je Zugang — was die Datei sagt, die Verfügbarkeit, und nach
// dem Test das Ergebnis.
function renderServices(data) {
  elements.services.replaceChildren();
  elements.servicesMessage.textContent = "";
  serviceBlocks.clear();
  elements.testAll.disabled = true;

  if (!data.available) {
    setPill(elements.servicesPill, "muted", "Keine Angabe");
    elements.servicesMessage.textContent = "Solange die Datei nicht lesbar ist, gibt es keine Dienste.";
    return;
  }
  const services = Object.entries(data.services || {});
  if ((data.problems || []).some((problem) => problem.level === "error")) {
    setPill(elements.servicesPill, "error", "Datei fehlerhaft");
    elements.servicesMessage.textContent = "Die Datei hat Fehler; bis sie behoben sind, gibt es keine Konfiguration zu zeigen.";
    return;
  }
  if (!services.length) {
    setPill(elements.servicesPill, "muted", "keine");
    elements.servicesMessage.textContent = "Noch ist kein Dienst eingetragen.";
    return;
  }
  const unusable = services.filter(([, service]) => !usable(service)).length;
  setPill(elements.servicesPill, unusable ? "warn" : "ok", unusable ? `${unusable} ohne verfügbaren Zugang` : `${services.length} nutzbar`);
  elements.testAll.disabled = false;
  for (const [name, service] of services) {
    elements.services.append(serviceBlock(name, service));
  }
}

function serviceBlock(name, service) {
  const block = document.createElement("section");
  block.className = "access-service";

  const head = document.createElement("div");
  head.className = "access-service-head";
  const title = document.createElement("h3");
  title.textContent = name;
  const repo = document.createElement("span");
  repo.className = "access-repo";
  repo.textContent = service.repo || "";
  const statePill = pill(usable(service) ? "ok" : "warn", usable(service) ? "nutzbar" : "kein verfügbarer Zugang");
  const button = document.createElement("button");
  button.type = "button";
  button.className = "secondary";
  button.textContent = "Prüfen";
  button.title = `Fragt GitHub, ob die verfügbaren Zugänge von ${name} funktionieren.`;
  button.addEventListener("click", () => testService(name));
  head.append(title, repo, statePill, button);
  block.append(head);

  const list = document.createElement("div");
  list.className = "access-list";
  const rows = (service.access || []).map((access) => accessRow(access));
  list.append(...rows.map((row) => row.element));
  block.append(list);

  const message = document.createElement("p");
  message.className = "message access-service-message";
  block.append(message);

  serviceBlocks.set(name, { button, rows, message, statePill });
  return block;
}

// Eine Zeile je Zugang: Typ und Schlüssel aus der Datei, die
// Verfügbarkeit, und ein Feld für das Ergebnis des Tests.
function accessRow(access) {
  const row = document.createElement("div");
  row.className = "access-row";

  const head = document.createElement("div");
  head.className = "access-row-head";
  const what = document.createElement("span");
  what.className = "access-what";
  what.textContent = describeAccess(access);
  head.append(what, pill(STATE_TONE[access.state] || "muted", STATE_LABEL[access.state] || access.state || "nicht beurteilt"));
  row.append(head);

  if (access.reason) {
    const reason = document.createElement("p");
    reason.className = "access-message";
    reason.textContent = access.reason;
    row.append(reason);
  }

  const result = document.createElement("div");
  result.className = "access-result";
  row.append(result);
  return { element: row, result };
}

function describeAccess(access) {
  const parts = [access.type];
  for (const key of ["login", "file", "key", "host"]) {
    if (access[key]) {
      parts.push(`${key}: ${access[key]}`);
    }
  }
  return parts.join("  ");
}

// „Alle prüfen“: ein POST je Dienst, nebeneinander; jeder Block füllt sich,
// sobald seine Antwort da ist.
async function testAll() {
  elements.testAll.disabled = true;
  elements.servicesMessage.textContent = "GitHub wird gefragt...";
  await Promise.all([...serviceBlocks.keys()].map((name) => testService(name)));
  elements.servicesMessage.textContent = "";
  elements.testAll.disabled = false;
}

async function testService(name) {
  const block = serviceBlocks.get(name);
  if (!block) {
    return;
  }
  block.button.disabled = true;
  block.message.textContent = "GitHub wird gefragt...";
  for (const row of block.rows) {
    row.result.replaceChildren();
  }
  let data;
  let status;
  try {
    const response = await fetch("/api/github-access/test", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ service: name }),
    });
    status = response.status;
    data = await response.json();
  } catch {
    block.message.textContent = "Der Test konnte nicht ausgeführt werden.";
    block.button.disabled = false;
    return;
  }
  block.button.disabled = false;
  if (status !== 200) {
    block.message.textContent = data.message || "Der Test ist gescheitert.";
    return;
  }
  block.message.textContent = "";
  const service = (data.services || {})[name];
  if (!service) {
    block.message.textContent = "Die Antwort trägt diesen Dienst nicht.";
    return;
  }
  (service.access || []).forEach((result, index) => {
    const row = block.rows[index];
    if (row) {
      renderResult(row.result, result);
    }
  });
}

// Das Ergebnis eines Zugangs: Marke mit ok, fehlgeschlagen oder
// übersprungen; dahinter Identität, lesen, schreiben, Ablauf, die Warnung
// und bei failed oder skipped die Meldung — GitHubs Antwort oder der Grund.
function renderResult(node, result) {
  node.replaceChildren();
  const head = document.createElement("div");
  head.className = "access-row-head";
  head.append(pill(RESULT_TONE[result.result] || "muted", RESULT_LABEL[result.result] || result.result || "unbekannt"));
  node.append(head);

  const facts = document.createElement("dl");
  facts.className = "facts access-facts";
  if (result.identity) {
    fact(facts, "Identität", result.identity);
  }
  if (result.read) {
    fact(facts, "Lesen", ANSWER_LABEL[result.read] || result.read);
  }
  if (result.write) {
    fact(facts, "Schreiben", ANSWER_LABEL[result.write] || result.write);
  }
  if (result.expires) {
    fact(facts, "Ablauf", result.expires === "none" ? "keiner" : result.expires);
  }
  if (result.warning) {
    fact(facts, "Warnung", pill("warn", result.warning));
  }
  if (facts.childElementCount) {
    node.append(facts);
  }
  if (result.message) {
    const message = document.createElement("p");
    message.className = "access-message";
    message.textContent = result.message;
    node.append(message);
  }
}
