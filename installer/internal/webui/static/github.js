"use strict";

// Seite "GitHub": Repo und Zugang, Pull Requests, CI-Läufe.
//
// Gelesen wird nur. Kein Element dieser Seite schreibt nach GitHub; ein PR
// nennt den Befehl für /k-pr-review zum Kopieren, mehr nicht.
//
// Die drei Karten laden unabhängig voneinander. Das ist der Grund für die drei
// Endpunkte: hinter jedem steht ein gh-Subprozess mit Netzzugriff, und eine
// langsame Abfrage soll die anderen Karten nicht aufhalten. Das Log eines
// roten Laufs kommt noch einmal später — erst beim Aufklappen.

// Muss vor den Ladefunktionen laufen: die blenden Blöcke ein, und das Menü
// zieht das nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();

startSession((message) => {
  document.getElementById("gh-overview-message").textContent = message;
});

const elements = {
  overviewPill: document.getElementById("gh-overview-pill"),
  overviewFacts: document.getElementById("gh-overview-facts"),
  aliasHint: document.getElementById("gh-alias-hint"),
  overviewMessage: document.getElementById("gh-overview-message"),

  pullsPill: document.getElementById("gh-pulls-pill"),
  pullsOpen: document.getElementById("gh-pulls-open"),
  pullsClosedWrap: document.getElementById("gh-pulls-closed-wrap"),
  pullsClosedLabel: document.getElementById("gh-pulls-closed-label"),
  pullsClosed: document.getElementById("gh-pulls-closed"),
  pullsMessage: document.getElementById("gh-pulls-message"),

  runsPill: document.getElementById("gh-runs-pill"),
  runsList: document.getElementById("gh-runs-list"),
  runsMessage: document.getElementById("gh-runs-message"),
};

// Ab dieser Zahl klappt die Liste der offenen PRs zusammen. Darunter ist sie
// kürzer als der Text darüber; darüber wird sie zur Strecke.
const FOLD_FROM = 8;

// Der Zustand ohne Daten ist erklärt, nicht leer. Welche Farbe die Marke
// dabei trägt, hängt daran, ob etwas kaputt ist oder nur nicht vorgesehen.
const STATE_TONE = {
  ok: "ok",
  disabled: "muted",
  undecided: "warn",
  "no-project": "muted",
  "not-installed": "warn",
  "not-logged-in": "warn",
  "bad-credentials": "error",
  "no-remote": "warn",
  "no-access": "error",
  "rate-limited": "warn",
  // Kein Fehler: GitHub hat das Log nach der Aufbewahrungsfrist verworfen.
  "log-gone": "muted",
  network: "error",
  timeout: "warn",
  // Kommt auf der Seite nicht an: eine abgebrochene Anfrage beantwortet der
  // Server nicht. Der Eintrag hält die Liste der Zustände vollständig.
  canceled: "muted",
  error: "error",
};

const STATE_LABEL = {
  disabled: "Nicht genutzt",
  undecided: "Offen",
  "no-project": "Kein Projekt",
  "not-installed": "gh fehlt",
  "not-logged-in": "Nicht angemeldet",
  "bad-credentials": "Token abgewiesen",
  "no-remote": "Kein Remote",
  "no-access": "Kein Zugriff",
  "rate-limited": "Rate-Limit",
  "log-gone": "Log verworfen",
  network: "Kein Netz",
  timeout: "Zeitüberschreitung",
  canceled: "Abgebrochen",
  error: "Fehler",
};

loadOverview();
loadPulls();
loadRuns();

async function fetchJSON(path) {
  const response = await fetch(path, { cache: "no-store" });
  return response.json();
}

function setPill(pill, tone, text) {
  pill.className = `pill ${tone}`;
  pill.textContent = text;
}

// Ein Zustand, der keine Daten liefert, setzt Marke und Satz und sagt der
// aufrufenden Karte, dass sie nichts weiter rendern muss.
function handledState(data, pill, message) {
  if (data.state === "ok") {
    return false;
  }
  setPill(pill, STATE_TONE[data.state] || "warn", STATE_LABEL[data.state] || "Unbekannt");
  message.textContent = data.message || "Unbekannter Zustand.";
  return true;
}

function emptyList(node, text) {
  node.classList.add("empty");
  node.replaceChildren(document.createTextNode(text));
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

function pill(tone, text) {
  const mark = document.createElement("span");
  mark.className = `pill ${tone}`;
  mark.textContent = text;
  return mark;
}

async function loadOverview() {
  let data;
  try {
    data = await fetchJSON("/api/github/overview");
  } catch {
    setPill(elements.overviewPill, "error", "Fehler");
    elements.overviewMessage.textContent = "Der Stand konnte nicht geladen werden.";
    return;
  }

  elements.overviewFacts.replaceChildren();
  elements.overviewMessage.textContent = "";
  if (handledState(data, elements.overviewPill, elements.overviewMessage)) {
    return;
  }

  setPill(elements.overviewPill, ciTone(data.ci), data.repo || "Repo");

  const link = document.createElement("a");
  link.href = data.url || "#";
  link.textContent = data.repo || "unbekannt";
  link.rel = "noreferrer";
  fact(elements.overviewFacts, "Repo", link);
  const notes = data.notes || {};
  fact(elements.overviewFacts, "Default-Branch", data.defaultBranch || "unbekannt");
  fact(elements.overviewFacts, "gh-Konto", data.account || noteValue(notes.account));
  // Leserecht ist nicht Schreibrecht: „angemeldet" sagt nichts darüber, ob
  // über gh gemergt oder approved werden kann.
  fact(elements.overviewFacts, "Recht am Repo", pill(permissionTone(data.permission), permissionLabel(data.permission)));
  fact(elements.overviewFacts, "CI auf dem Default-Branch", ciValue(data, notes.workflows));
  fact(elements.overviewFacts, "Letzter Tag", tagValue(data, notes.tag));
  fact(elements.overviewFacts, "Git-Remote", data.remoteUrl || noteValue(notes.remote));

  for (const workflow of data.workflows || []) {
    fact(elements.overviewFacts, workflow.name, pill(runTone(workflow), runResult(workflow)));
  }

  // Der Hinweis nennt den Alias und kein Konto: der Aliasname ist kein
  // Kontoname, und wer hinter seinem Schlüssel steht, stünde erst nach einem
  // weiteren Netzaufruf fest.
  elements.aliasHint.classList.toggle("hidden", !data.aliasHint);
  elements.aliasHint.textContent = data.aliasHint || "";
}

// Ein leeres Feld hat drei mögliche Gründe, und die Seite nennt sie
// unterschiedlich: „nichts da" ist ein gültiger Leerzustand und steht als
// schlichter Satz; „nicht lesbar" und „Frist abgelaufen" tragen eine Marke,
// damit ein Fehler nie wie ein leeres Feld aussieht.
const FIELD_MARK = {
  unreadable: ["error", "nicht lesbar"],
  timeout: ["warn", "Frist abgelaufen"],
};

function noteValue(note, fallback = "unbekannt") {
  if (!note || !note.state || note.state === "ok") {
    return fallback;
  }
  const mark = FIELD_MARK[note.state];
  if (!mark) {
    return note.message || fallback;
  }
  const value = document.createElement("span");
  const text = document.createElement("span");
  text.className = "hint";
  text.textContent = note.message || "";
  value.append(pill(mark[0], mark[1]), " ", text);
  return value;
}

function ciValue(data, note) {
  if ((data.workflows || []).length) {
    return pill(ciTone(data.ci), ciLabel(data));
  }
  if (note && FIELD_MARK[note.state]) {
    return noteValue(note);
  }
  return pill("muted", (note && note.message) || "keine Läufe");
}

function tagValue(data, note) {
  if (!data.lastTag) {
    return noteValue(note, "kein Tag gefunden");
  }
  // Tag gefunden, aber Zählung oder Datum gescheitert: der Tag steht da,
  // „nichts seitdem" wird nicht behauptet.
  if (note && FIELD_MARK[note.state]) {
    const value = document.createElement("span");
    value.append(`${data.lastTag} — `, noteValue(note));
    return value;
  }
  return tagText(data);
}

function permissionLabel(permission) {
  if (!permission) {
    return "unbekannt";
  }
  if (permission === "READ") {
    return "READ — nur lesen";
  }
  return permission;
}

function permissionTone(permission) {
  switch (permission) {
    case "ADMIN":
    case "MAINTAIN":
    case "WRITE":
      return "ok";
    case "READ":
    case "TRIAGE":
      return "warn";
    default:
      return "muted";
  }
}

function ciTone(ci) {
  return ci === "ok" || ci === "warn" || ci === "error" ? ci : "muted";
}

function ciLabel(data) {
  const count = (data.workflows || []).length;
  switch (data.ci) {
    case "ok":
      return `grün (${count} Workflows)`;
    case "warn":
      return `läuft noch (${count} Workflows)`;
    case "error":
      return `rot (${count} Workflows)`;
    default:
      return `${count} Workflows`;
  }
}

function tagText(data) {
  const since = data.commitsSinceTag || 0;
  const commits = since === 1 ? "1 Commit" : `${since} Commits`;
  return since ? `${data.lastTag} — ${commits} seitdem` : `${data.lastTag} — nichts seitdem`;
}

function runTone(run) {
  if (run.status && run.status !== "completed") {
    return "warn";
  }
  switch (run.conclusion) {
    case "success":
      return "ok";
    case "failure":
    case "timed_out":
    case "startup_failure":
      return "error";
    case "cancelled":
    case "action_required":
      return "warn";
    default:
      return "muted";
  }
}

function runResult(run) {
  if (run.status && run.status !== "completed") {
    return run.status === "in_progress" ? "läuft" : "wartet";
  }
  return run.conclusion || "ohne Ergebnis";
}

async function loadPulls() {
  let data;
  try {
    data = await fetchJSON("/api/github/pulls");
  } catch {
    setPill(elements.pullsPill, "error", "Fehler");
    elements.pullsMessage.textContent = "Die Pull Requests konnten nicht geladen werden.";
    return;
  }

  elements.pullsMessage.textContent = "";
  elements.pullsClosedWrap.classList.add("hidden");
  if (handledState(data, elements.pullsPill, elements.pullsMessage)) {
    emptyList(elements.pullsOpen, "Keine Angabe möglich.");
    return;
  }

  const open = data.open || [];
  const closed = data.closed || [];
  setPill(elements.pullsPill, open.length ? "ok" : "muted", open.length ? `${open.length} offen` : "keine offenen");

  // Dieses Repo hat keine PRs. Die leere Liste ist ein gültiger Zustand und
  // muss als solcher aussehen — nicht wie ein Fehler.
  if (!open.length && !closed.length) {
    emptyList(elements.pullsOpen, "Keine Pull Requests vorhanden — weder offene noch geschlossene.");
    return;
  }
  if (!open.length) {
    emptyList(elements.pullsOpen, "Kein offener Pull Request.");
  } else {
    renderPulls(elements.pullsOpen, open, data.defaultBranch);
  }

  if (closed.length) {
    elements.pullsClosedWrap.classList.remove("hidden");
    elements.pullsClosedLabel.textContent = `Zuletzt geschlossen und gemergt (${closed.length} von höchstens ${data.closedLimit})`;
    renderPulls(elements.pullsClosed, closed, data.defaultBranch);
  }
}

function renderPulls(node, pulls, defaultBranch) {
  node.classList.remove("empty");
  const rows = pulls.map((pull) => pullRow(pull, defaultBranch));

  // Über einer Handvoll wird die Liste zur Strecke: der Rest klappt zusammen.
  if (rows.length <= FOLD_FROM) {
    node.replaceChildren(...rows);
    return;
  }
  const fold = document.createElement("details");
  fold.className = "gh-fold";
  const head = document.createElement("summary");
  head.className = "disclosure gh-fold-head";
  const marker = document.createElement("span");
  marker.className = "disclosure-marker";
  marker.setAttribute("aria-hidden", "true");
  marker.textContent = "▸";
  const label = document.createElement("span");
  label.textContent = `${rows.length - FOLD_FROM} weitere`;
  head.append(marker, label);
  fold.append(head, ...rows.slice(FOLD_FROM));
  node.replaceChildren(...rows.slice(0, FOLD_FROM), fold);
}

function pullRow(pull, defaultBranch) {
  const item = document.createElement("div");
  item.className = "pr-item";

  const head = document.createElement("div");
  head.className = "pr-head";
  const link = document.createElement("a");
  link.className = "pr-number";
  link.href = pull.url || "#";
  link.rel = "noreferrer";
  link.textContent = `#${pull.number}`;
  const title = document.createElement("span");
  title.className = "pr-title";
  title.textContent = pull.title || "ohne Titel";
  head.append(link, title, pill(pullTone(pull), pullState(pull)));
  item.append(head);

  // Quelle → Ziel steht immer da: „wohin geht dieser PR" ist die Frage, die
  // die Liste beantwortet.
  const branches = document.createElement("div");
  branches.className = "pr-branches";
  branches.textContent = `${pull.head || "?"} → ${pull.base || "?"}`;
  item.append(branches);

  const marks = document.createElement("div");
  marks.className = "pill-row";
  if (pull.fork) {
    marks.append(pill("warn", pull.forkOwner ? `Fork: ${pull.forkOwner}` : "Fork"));
  }
  if (pull.nonDefaultBase) {
    marks.append(pill("warn", `Ziel ≠ ${defaultBranch || "Default"}`));
  }
  if (pull.dependabot) {
    marks.append(pill("muted", "Dependabot"));
  }
  if (pull.checks) {
    marks.append(pill(checkTone(pull.checks), `Checks: ${pull.checks}`));
  }
  if (pull.reviewDecision) {
    marks.append(pill(reviewTone(pull.reviewDecision), reviewLabel(pull.reviewDecision)));
  }
  if (pull.mergeable === "CONFLICTING") {
    marks.append(pill("error", "Konflikt"));
  }
  for (const label of pull.labels || []) {
    marks.append(pill("muted", label));
  }
  if (marks.childElementCount) {
    item.append(marks);
  }

  const meta = document.createElement("div");
  meta.className = "pr-meta";
  meta.textContent = pullMeta(pull);
  item.append(meta);

  // Kein Knopf, der etwas auslöst: der Einstieg ist der Befehl im Assistenten.
  const command = document.createElement("code");
  command.className = "pr-command";
  command.textContent = pull.command || `/k-pr-review ${pull.number}`;
  item.append(command);
  return item;
}

function pullMeta(pull) {
  const parts = [];
  parts.push(pull.author ? `von ${pull.author}` : "Autor unbekannt");
  parts.push(`+${pull.additions || 0}/−${pull.deletions || 0} in ${pull.changedFiles || 0} Dateien`);
  if (pull.mergedAt) {
    parts.push(`gemergt ${shortDate(pull.mergedAt)}`);
  } else if (pull.updatedAt) {
    parts.push(`zuletzt ${shortDate(pull.updatedAt)}`);
  }
  return parts.join(" · ");
}

function pullState(pull) {
  switch (pull.state) {
    case "draft":
      return "Entwurf";
    case "merged":
      return "gemergt";
    case "closed":
      return "geschlossen";
    default:
      return "offen";
  }
}

function pullTone(pull) {
  switch (pull.state) {
    case "merged":
      return "ok";
    case "draft":
    case "closed":
      return "muted";
    default:
      return "warn";
  }
}

function checkTone(state) {
  switch (state) {
    case "SUCCESS":
      return "ok";
    case "FAILURE":
    case "ERROR":
      return "error";
    default:
      return "warn";
  }
}

function reviewLabel(decision) {
  switch (decision) {
    case "APPROVED":
      return "Review: freigegeben";
    case "CHANGES_REQUESTED":
      return "Review: Änderungen erbeten";
    case "REVIEW_REQUIRED":
      return "Review: ausstehend";
    default:
      return `Review: ${decision}`;
  }
}

function reviewTone(decision) {
  switch (decision) {
    case "APPROVED":
      return "ok";
    case "CHANGES_REQUESTED":
      return "error";
    default:
      return "warn";
  }
}

async function loadRuns() {
  let data;
  try {
    data = await fetchJSON("/api/github/runs");
  } catch {
    setPill(elements.runsPill, "error", "Fehler");
    elements.runsMessage.textContent = "Die Läufe konnten nicht geladen werden.";
    return;
  }

  elements.runsMessage.textContent = "";
  if (handledState(data, elements.runsPill, elements.runsMessage)) {
    emptyList(elements.runsList, "Keine Angabe möglich.");
    return;
  }

  const runs = data.runs || [];
  if (!runs.length) {
    setPill(elements.runsPill, "muted", "keine");
    emptyList(elements.runsList, "Noch kein Lauf in diesem Repo.");
    return;
  }

  const failed = runs.filter((run) => run.failed).length;
  setPill(elements.runsPill, failed ? "error" : "ok", failed ? `${failed} rot` : `${runs.length} grün`);
  elements.runsList.classList.remove("empty");
  elements.runsList.replaceChildren(...runs.map(runRow));
}

function runRow(run) {
  // Nur ein roter Lauf hat etwas aufzuklappen; die übrigen bleiben eine Zeile.
  const item = document.createElement(run.failed ? "details" : "div");
  item.className = "run-item";

  // Kopf und Kennzahlen stehen zusammen im summary. Läge die Kennzahlenzeile
  // daneben, wäre sie am zugeklappten Lauf verborgen — ausgerechnet am roten,
  // der als einziger zugeklappt ist.
  const head = document.createElement(run.failed ? "summary" : "div");
  head.className = run.failed ? "disclosure run-summary" : "run-summary";

  const row = document.createElement("div");
  row.className = "run-head";
  if (run.failed) {
    const marker = document.createElement("span");
    marker.className = "disclosure-marker";
    marker.setAttribute("aria-hidden", "true");
    marker.textContent = "▸";
    row.append(marker);
  }

  const name = document.createElement("span");
  name.className = "run-workflow";
  name.textContent = run.workflow || "Workflow";
  const ref = document.createElement("span");
  ref.className = "run-ref";
  ref.textContent = run.isTag ? `Tag ${run.ref}` : run.ref || "?";
  row.append(name, ref, pill(runTone(run), runResult(run)));

  const meta = document.createElement("div");
  meta.className = "run-meta";
  meta.textContent = runMeta(run);
  head.append(row, meta);
  item.append(head);

  if (!run.failed) {
    return item;
  }

  const body = document.createElement("div");
  body.className = "run-failure";
  body.textContent = "Ursache wird beim Aufklappen geholt.";
  item.append(body);

  // Das Log eines Laufs ist die teuerste Abfrage dieser Seite. Sie läuft erst
  // hier und genau einmal je Lauf.
  let loaded = false;
  item.addEventListener("toggle", () => {
    if (!item.open || loaded) {
      return;
    }
    loaded = true;
    loadFailure(run, body);
  });
  return item;
}

function runMeta(run) {
  const parts = [];
  if (run.title) {
    parts.push(run.title);
  }
  parts.push(run.event || "Ereignis unbekannt");
  if (run.seconds) {
    parts.push(duration(run.seconds));
  }
  if (run.createdAt) {
    parts.push(shortDate(run.createdAt));
  }
  return parts.join(" · ");
}

async function loadFailure(run, body) {
  body.textContent = "Ursache wird geholt...";
  let data;
  try {
    data = await fetchJSON(`/api/github/runs/${run.id}/failure`);
  } catch {
    body.textContent = "Das Log konnte nicht geholt werden.";
    return;
  }

  // Die Marke sagt, welcher Fall es ist: ein verworfenes Log ist etwas anderes
  // als fehlender Zugriff oder eine abgelaufene Frist.
  if (data.state !== "ok") {
    const text = document.createElement("p");
    text.className = "hint gh-note";
    text.textContent = data.message || "Das Log ist nicht abrufbar.";
    // Ein Lauf-404 heißt falsche Kennung oder fehlender Zugriff — gh trennt
    // beides nicht. Die Marke „Kein Zugriff" behauptete eines davon.
    const label = data.state === "no-access" ? "Nicht abrufbar" : STATE_LABEL[data.state] || "Unbekannt";
    body.replaceChildren(pill(STATE_TONE[data.state] || "warn", label), text);
    return;
  }

  const parts = [];
  // Gruppiert nach gleicher Meldung: „32 Tests: stat …" statt 32 Zeilen.
  for (const group of data.groups || []) {
    parts.push(failureGroup(group));
  }
  // Unbekanntes Logformat: die letzten Fehlerzeilen statt einer Gruppierung.
  if (!parts.length && (data.lines || []).length) {
    const block = document.createElement("pre");
    block.className = "snippet";
    block.textContent = data.lines.join("\n");
    parts.push(block);
  }
  if (data.note) {
    const note = document.createElement("p");
    note.className = "hint gh-note";
    note.textContent = data.note;
    parts.push(note);
  }
  if (!parts.length) {
    body.textContent = "Im Log steht keine auswertbare Ursache.";
    return;
  }
  body.replaceChildren(...parts);
}

function failureGroup(group) {
  const box = document.createElement("div");
  box.className = "failure-group";

  const head = document.createElement("div");
  head.className = "failure-head";
  const count = document.createElement("span");
  count.className = "failure-count";
  count.textContent = group.count === 1 ? "1 Test" : `${group.count} Tests`;
  const message = document.createElement("code");
  message.className = "failure-message";
  message.textContent = group.message;
  head.append(count, message);
  box.append(head);

  const tests = document.createElement("details");
  tests.className = "gh-fold";
  const summary = document.createElement("summary");
  summary.className = "disclosure gh-fold-head";
  const marker = document.createElement("span");
  marker.className = "disclosure-marker";
  marker.setAttribute("aria-hidden", "true");
  marker.textContent = "▸";
  const label = document.createElement("span");
  label.textContent = "Welche Tests";
  summary.append(marker, label);
  const list = document.createElement("div");
  list.className = "failure-tests";
  list.textContent = (group.tests || []).join("\n");
  tests.append(summary, list);
  box.append(tests);
  return box;
}

function duration(seconds) {
  if (seconds < 60) {
    return `${seconds}s`;
  }
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${seconds % 60}s`;
}

// Kurzes Datum ohne Bibliothek. Ein unlesbarer Wert bleibt stehen, statt als
// „Invalid Date" zu erscheinen.
function shortDate(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("de-DE", { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit" });
}
