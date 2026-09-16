"use strict";

// Seite einer Chat-Sitzung (/chat/<id>): Verlauf, Eingabe, Freigaben und
// Rückfragen.
//
// Die Seite hält nichts dauerhaft: sie lädt den Verlauf der Sitzung und spielt
// danach die Ereignisse des Stroms ein. Die Regeln dafür folgen der Web-App von
// OpenCode und stehen in installer/docs/architecture.md unter „Chat in der
// Oberfläche". Gemeinsames mit der Liste steht in chat-common.js.

const elements = {
  title: document.getElementById("chat-title"),
  parent: document.getElementById("chat-parent"),
  runPill: document.getElementById("chat-run-pill"),
  log: document.getElementById("chat-log"),
  permissions: document.getElementById("chat-permissions"),
  questions: document.getElementById("chat-questions"),
  form: document.getElementById("chat-form"),
  input: document.getElementById("chat-input"),
  suggestions: document.getElementById("chat-suggestions"),
  send: document.getElementById("chat-send"),
  abort: document.getElementById("chat-abort"),
  agentRow: document.getElementById("chat-agent-row"),
  agent: document.getElementById("chat-agent"),
  message: document.getElementById("chat-message"),
};

// Muss vor den Ladefunktionen laufen: die setzen Pills, und das Menü zieht das
// nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();

startSession((message) => {
  elements.message.textContent = message;
  setFormEnabled(false);
});

// Teile, die nur den Ablauf markieren und nichts zu lesen haben. Die Web-App
// von OpenCode blendet dieselben aus.
const HIDDEN_PARTS = new Set(["step-start", "step-finish", "patch", "snapshot"]);

// Fertige Antworttexte zeigt die Seite gerendert. Das HTML kommt vom Server
// (Goldmark ohne rohes HTML und ohne gefährliche Verweise): Text → HTML, false
// nach einem Fehlschlag.
const markdownCache = new Map();
// Texte, die aufs Rendern warten, samt den Nachrichten, die sie zeigen.
const markdownWaiting = new Map();
let markdownTimer = 0;

// Die Kennung steht im Pfad; ob sie gültig ist, hat der Server beim
// Ausliefern der Seite geprüft.
const sessionID = decodeURIComponent(window.location.pathname.replace(/^\/chat\//, ""));

const chat = {
  available: false,
  // Die Sitzung selbst, wie der Server sie geliefert hat. Sie nennt den
  // Vorgabe-Agenten und, bei einer Kind-Sitzung, ihre Elternsitzung.
  session: null,
  // messageID → { info, parts: Map(partID → part) }. IDs von OpenCode sind
  // aufsteigend sortierbar, die Reihenfolge ergibt sich daraus.
  messages: new Map(),
  // requestID → PermissionRequest, über alle Sitzungen.
  permissions: new Map(),
  // requestID → QuestionRequest. Anders als bei den Freigaben stehen hier nur
  // Anfragen, die zu dieser Seite gehören — die eigene Sitzung und ihre
  // Nachkommen. Was nicht dazugehört, kommt gar nicht erst hinein.
  questions: new Map(),
  // sessionID → true/false: gehört die Sitzung zu dieser Seite? Gefüllt aus
  // den Kind-Sitzungen, aus session.created, aus den task-Tool-Teilen und aus
  // der Elternkette.
  related: new Map(),
  // sessionID → laufende Herkunftsprüfung. Sie hält die Zusage, höchstens
  // eine Anfrage je fremder Sitzung zu stellen, auch bei zwei Rückfragen
  // derselben Sitzung kurz hintereinander.
  relatedChecks: new Map(),
  // name → Command aus der gefilterten Liste. Sie entscheidet beim Senden, ob
  // ein Text ein Command ist; einmal je Seite geladen.
  commands: new Map(),
  connectedOnce: false,
};

// Der Laufzustand entsteht aus mehreren Quellen zugleich. Welche die Marke
// bekommt, entscheidet die Vorrangliste in renderRunState(); hier steht, was
// die Quellen zuletzt gemeldet haben.
const run = {
  // Was die Ereignisse zuletzt sagten: "idle", "busy" oder "retry".
  type: "idle",
  // "" solange die Verbindung steht, sonst "closed" oder "reconnecting".
  connection: "",
  // Ein Fehler des Laufs. Er bleibt nicht die ganze Sitzung stehen, sondern
  // wird beim nächsten Senden und beim nächsten busy geräumt.
  failed: false,
  // Eine Marke, die die Sitzung abschließt — nicht erreichbar, nicht
  // gefunden, gelöscht. Sie steht über allem und wird nicht überschrieben.
  fixed: null,
};

elements.form.addEventListener("submit", (event) => {
  event.preventDefault();
  sendPrompt();
});
elements.input.addEventListener("keydown", (event) => {
  // Zuerst die Vorschlagsliste: solange sie geschlossen ist, sagt sie nein und
  // die Eingabe verhält sich wie zuvor.
  if (handleSuggestionKey(event)) {
    return;
  }
  if (event.key === "Enter" && !event.shiftKey && !event.isComposing) {
    event.preventDefault();
    sendPrompt();
  }
});
elements.input.addEventListener("input", updateSuggestions);
elements.input.addEventListener("blur", closeSuggestions);
// Ein Klick in die Liste soll die Schreibmarke im Feld lassen: verlöre das Feld
// den Fokus, schlösse die Liste, bevor der Klick ankommt.
elements.suggestions.addEventListener("pointerdown", (event) => {
  event.preventDefault();
});
elements.abort.addEventListener("click", abortRun);

init();

async function init() {
  renderLog();

  let status;
  try {
    status = await chatApi("/api/chat/status");
  } catch (error) {
    showUnavailable(error.message);
    return;
  }
  if (!status.available) {
    showUnavailable(status.message || "Der OpenCode-Dienst ist nicht erreichbar.");
    return;
  }
  chat.available = true;

  try {
    showSession(await chatApi(`/api/chat/sessions/${encodeURIComponent(sessionID)}`));
  } catch (error) {
    setFixedRun("error", "Nicht gefunden");
    elements.message.textContent = error.message;
    return;
  }

  setRunState("idle");
  setFormEnabled(true);
  openChatEvents(applyEvent, setConnectionState);
  await Promise.all([loadMessages(), loadPermissions(), loadRelated(), loadCommands(), loadAgents()]);
}

function showUnavailable(text) {
  setFixedRun("error", "Nicht erreichbar");
  elements.message.textContent = `${text} Was zu tun ist, steht unter „Alle Sitzungen".`;
  setFormEnabled(false);
}

// Übernimmt die Sitzung: Titel, Rückweg zur Elternsitzung und der
// Vorgabe-Agent stammen alle daraus.
function showSession(session) {
  chat.session = session;
  showTitle(session);
  showParentLink(session);
}

function showTitle(session) {
  elements.title.textContent = sessionTitle(session);
  document.title = `${sessionTitle(session)} - k-playbook`;
}

// Eine Kind-Sitzung bekommt hinter dem Rückweg einen Link auf ihre
// Elternsitzung — von dort ist sie über den task-Tool-Teil erreichbar, aber
// nicht umgekehrt: die Liste /chat zeigt nur oberste Sitzungen.
function showParentLink(session) {
  elements.parent.replaceChildren();
  const parent = session && session.parentID;
  if (!parent) {
    return;
  }
  const link = document.createElement("a");
  link.className = "chat-back";
  link.href = `/chat/${encodeURIComponent(parent)}`;
  link.textContent = "Elternsitzung öffnen";
  elements.parent.append(" · ", link);
}

// --- Ereignisse -------------------------------------------------------------

// Spielt ein Ereignis in den Stand der Seite ein. Was eine andere Sitzung
// betrifft, bleibt außen vor.
function applyEvent(event) {
  const props = event.properties || {};
  switch (event.type) {
    case "server.connected":
      // Nach einem Wiederverbinden fehlt, was dazwischen geschah — es kam
      // nicht als Ereignis. Beim ersten Verbinden lädt init() ohnehin.
      if (chat.connectedOnce) {
        loadPermissions();
        loadMessages();
        loadRelated();
        // Ein Ausgang, der während der Trennung feststand, kam nicht an.
        pollCommandState();
      }
      chat.connectedOnce = true;
      return;

    case "session.created":
      // Eine neue Kind-Sitzung: ihre Rückfragen gehören auf diese Seite. Die
      // Marke wird gesetzt, bevor die erste eintrifft — trifft eine früher
      // ein, trägt die Prüfung über die Elternkette.
      if (props.info && props.info.id && props.info.parentID === sessionID) {
        noteChildSession(props.info.id);
      }
      return;
    case "session.updated":
      if (props.info && props.info.id === sessionID) {
        showSession(props.info);
      }
      return;
    case "session.deleted":
      if (((props.info && props.info.id) || props.sessionID) === sessionID) {
        setFixedRun("error", "Gelöscht");
        elements.message.textContent = "Diese Sitzung wurde gelöscht.";
        setFormEnabled(false);
      }
      return;
    case "session.status":
      if (props.sessionID === sessionID) {
        setRunState(props.status && props.status.type, props.status);
      }
      return;
    case "session.idle":
      if (props.sessionID === sessionID) {
        setRunState("idle");
        // Der Lauf ruht; falls ein Command noch aussteht, ist sein Ausgang
        // jetzt eher zu haben als in fünf Sekunden.
        pollCommandState();
      }
      return;
    case "session.error":
      if (props.sessionID === sessionID && props.error) {
        elements.message.textContent = describeChatError(props.error);
        // Ein Abbruch ist kein Fehler des Laufs, sondern sein gewolltes Ende;
        // er bekommt die Meldung, aber nicht die Marke.
        if (props.error.name !== "MessageAbortedError") {
          run.failed = true;
          renderRunState();
        }
      }
      return;

    case "message.updated": {
      const info = props.info;
      if (!info || info.sessionID !== sessionID) {
        return;
      }
      const entry = chat.messages.get(info.id);
      if (entry) {
        entry.info = info;
      } else {
        chat.messages.set(info.id, { info, parts: new Map() });
      }
      refreshMessage(info.id);
      // Die echte Nachricht ist da; die vorläufige Blase hat ihren Dienst getan.
      dropPendingMessage(info, Boolean(entry));
      return;
    }
    case "message.removed":
      if (props.sessionID === sessionID) {
        chat.messages.delete(props.messageID);
        removeMessageElement(props.messageID);
      }
      return;
    case "message.part.updated": {
      const part = props.part;
      if (!part || part.sessionID !== sessionID) {
        return;
      }
      let entry = chat.messages.get(part.messageID);
      if (!entry) {
        // Der Teil kam vor seiner Nachricht; deren Kopf folgt mit message.updated.
        entry = { info: { id: part.messageID, sessionID, role: "assistant", time: {} }, parts: new Map() };
        chat.messages.set(part.messageID, entry);
      }
      // Ein Update ersetzt den Teil ganz — auch den bis dahin aus Deltas
      // angesammelten Text.
      entry.parts.set(part.id, part);
      noteChildFromPart(part);
      refreshMessage(part.messageID);
      return;
    }
    case "message.part.delta": {
      if (props.sessionID !== sessionID) {
        return;
      }
      const entry = chat.messages.get(props.messageID);
      const part = entry && entry.parts.get(props.partID);
      // Ein Delta setzt den Teil voraus; ohne ihn wird es verworfen, das
      // abschließende Update bringt den vollständigen Text.
      if (!part) {
        return;
      }
      const current = part[props.field];
      part[props.field] = (typeof current === "string" ? current : "") + props.delta;
      refreshMessage(props.messageID);
      return;
    }
    case "message.part.removed": {
      if (props.sessionID !== sessionID) {
        return;
      }
      const entry = chat.messages.get(props.messageID);
      if (entry) {
        entry.parts.delete(props.partID);
        refreshMessage(props.messageID);
      }
      return;
    }

    case "permission.asked":
      chat.permissions.set(props.id, props);
      renderPermissions();
      return;
    case "permission.replied":
      chat.permissions.delete(props.requestID);
      renderPermissions();
      return;

    case "question.asked":
      // Gilt über alle Sitzungen; ob sie hierher gehört, entscheidet
      // offerQuestion — gegebenenfalls über die Elternkette.
      offerQuestion(props);
      return;
    case "question.replied":
    case "question.rejected":
      if (chat.questions.delete(props.requestID)) {
        renderQuestions();
      }
      return;
  }
}

// Merkt sich die Kind-Sitzung eines Subagent-Aufrufs: ihre Rückfragen gehören
// auf diese Seite.
function noteChildFromPart(part) {
  if (!part || part.type !== "tool" || part.tool !== "task") {
    return;
  }
  const child = part.state && part.state.metadata && part.state.metadata.sessionId;
  if (child) {
    noteChildSession(child);
  }
}

// Eine Kind-Sitzung dieser Seite: ihre Rückfragen gehören hierher, und ein
// stehender Subtask-Hinweis bekommt jetzt seinen Link.
function noteChildSession(id) {
  chat.related.set(id, true);
  linkSubtaskHint(id);
}

// --- Unterhaltung -----------------------------------------------------------

async function loadMessages() {
  try {
    const items = await chatApi(`/api/chat/sessions/${encodeURIComponent(sessionID)}/messages`);
    chat.messages = new Map();
    for (const item of items) {
      chat.messages.set(item.info.id, { info: item.info, parts: new Map(item.parts.map((part) => [part.id, part])) });
      item.parts.forEach(noteChildFromPart);
    }
    // Eine Antwort ohne Abschluss und ohne Fehler läuft noch.
    const last = [...chat.messages.values()].filter((entry) => entry.info.role === "assistant").pop();
    const running = last && !(last.info.time && last.info.time.completed) && !last.info.error;
    setRunState(running ? "busy" : "idle");
    // Nach einem Wiederverbinden kann die eigene Nachricht längst angekommen
    // sein, ohne dass ihr Ereignis die Seite je erreicht hat.
    if (pending.id && chat.messages.has(pending.id)) {
      clearPendingMessage();
    }
  } catch (error) {
    elements.message.textContent = error.message;
  }
  renderLog();
  elements.log.scrollTop = elements.log.scrollHeight;
}

function renderLog() {
  const follow = isNearBottom();
  elements.log.replaceChildren();
  const ids = [...chat.messages.keys()].sort(compareIDs);
  if (ids.length === 0 && !pending.node) {
    const empty = document.createElement("p");
    empty.className = "hint";
    empty.textContent = "Noch keine Nachricht. Unten die erste schreiben.";
    elements.log.append(empty);
    return;
  }
  for (const id of ids) {
    elements.log.append(renderMessage(chat.messages.get(id)));
  }
  // Die vorläufige Blase wird hinter die sortierte Liste gehängt, nicht
  // einsortiert: bis die echte Nachricht da ist, gehört sie ans Ende.
  if (pending.node) {
    elements.log.append(pending.node);
  }
  if (follow) {
    elements.log.scrollTop = elements.log.scrollHeight;
  }
}

// Ersetzt eine einzelne Nachricht, statt den ganzen Verlauf neu zu bauen —
// Deltas kommen buchstabenweise. Aufgeklappte Details bleiben aufgeklappt.
function refreshMessage(id) {
  const entry = chat.messages.get(id);
  const old = elements.log.querySelector(`[data-message="${CSS.escape(id)}"]`);
  if (!entry || !old) {
    renderLog();
    return;
  }
  const follow = isNearBottom();
  const next = renderMessage(entry);
  const open = new Set([...old.querySelectorAll("details[open]")].map((details) => details.dataset.part));
  next.querySelectorAll("details").forEach((details) => {
    if (open.has(details.dataset.part)) {
      details.open = true;
    }
  });
  old.replaceWith(next);
  if (follow) {
    elements.log.scrollTop = elements.log.scrollHeight;
  }
}

function removeMessageElement(id) {
  const element = elements.log.querySelector(`[data-message="${CSS.escape(id)}"]`);
  if (element) {
    element.remove();
  }
}

// --- Die eigene Nachricht, sofort ------------------------------------------
//
// Was gesendet wurde, steht sofort da — die echte Nachricht kommt erst über
// den Ereignisstrom zurück. Zugeordnet wird über das messageID, das die
// Weiterleitung erzeugt und in ihrer Antwort zurückgibt: damit verschwindet die
// Blase genau dann, wenn die echte Nachricht da ist, und es entsteht weder eine
// Doppelanzeige noch eine Geisterblase.

const pending = {
  // Der Kasten im Verlauf, solange einer steht.
  node: null,
  // Die Kennung, unter der die echte Nachricht erwartet wird. Leer, solange die
  // Antwort der Weiterleitung noch aussteht oder keine Kennung mitbrachte.
  id: "",
};

function showPendingMessage(text) {
  clearPendingMessage();
  const element = document.createElement("article");
  element.className = "chat-message user pending";
  const head = document.createElement("p");
  head.className = "eyebrow chat-message-head";
  head.textContent = "Du · wird gesendet";
  const body = document.createElement("div");
  body.className = "chat-text";
  body.textContent = text;
  element.append(head, body);
  pending.node = element;
  pending.id = "";
  // Über renderLog(), nicht von Hand angehängt: sonst stünde die erste
  // Nachricht einer Sitzung unter dem Hinweis „Noch keine Nachricht".
  renderLog();
  elements.log.scrollTop = elements.log.scrollHeight;
}

function clearPendingMessage() {
  if (pending.node) {
    pending.node.remove();
  }
  pending.node = null;
  pending.id = "";
  if (elements.log.children.length === 0) {
    renderLog();
  }
}

// Verwirft die Blase, sobald die echte Nachricht da ist. Kennt die Seite deren
// Kennung, zählt nur die; sonst trägt der Rückfallweg — die erste noch
// unbekannte Nutzernachricht ist die eigene. Der Rückfallweg bleibt, weil
// ungemessen ist, ob die alte API ein mitgeschicktes messageID übernimmt.
function dropPendingMessage(info, known) {
  if (!pending.node) {
    return;
  }
  if (pending.id) {
    if (info.id === pending.id) {
      clearPendingMessage();
    }
    return;
  }
  if (!known && info.role === "user") {
    clearPendingMessage();
  }
}

function isNearBottom() {
  const log = elements.log;
  return log.scrollHeight - log.scrollTop - log.clientHeight < log.clientHeight / 4;
}

function compareIDs(a, b) {
  return a < b ? -1 : a > b ? 1 : 0;
}

function renderMessage(entry) {
  const info = entry.info;
  const element = document.createElement("article");
  element.className = `chat-message ${info.role === "user" ? "user" : "assistant"}`;
  element.dataset.message = info.id;

  const head = document.createElement("p");
  head.className = "eyebrow chat-message-head";
  head.textContent = info.role === "user" ? "Du" : `OpenCode · ${info.agent || "Agent"}`;
  element.append(head);

  const parts = [...entry.parts.values()]
    .filter((part) => !HIDDEN_PARTS.has(part.type))
    .sort((a, b) => compareIDs(a.id, b.id));
  for (const part of parts) {
    const node = renderPart(part, info);
    if (node) {
      element.append(node);
    }
  }

  if (info.error) {
    const error = document.createElement("p");
    error.className = "chat-error";
    error.textContent = describeChatError(info.error);
    element.append(error);
  }
  return element;
}

function renderPart(part, info) {
  switch (part.type) {
    case "text": {
      if (part.synthetic || part.ignored) {
        return null;
      }
      return renderText(part, info);
    }
    case "reasoning": {
      if (!part.text) {
        return null;
      }
      // Das Reasoning — die Denkschritte des Modells — ist Beiwerk zur
      // Antwort: kleine, gedämpfte Schrift ohne Rahmen.
      const text = document.createElement("p");
      text.className = "chat-reasoning";
      text.textContent = part.text.trim();
      return text;
    }
    case "tool":
      return renderTool(part);
    case "subtask": {
      const note = document.createElement("p");
      note.className = "hint";
      note.textContent = `Auftrag an Subagent ${part.agent}: ${part.description}`;
      return note;
    }
    case "file": {
      const note = document.createElement("p");
      note.className = "hint";
      note.textContent = `Datei: ${part.filename || part.url}`;
      return note;
    }
    default:
      return null;
  }
}

// Antworttext: solange er entsteht, als schlichter Text — mitten im Satz ist
// Markdown oft unvollständig. Fertig wird er gerendert; bis das HTML da ist,
// bleibt der schlichte Text stehen. Eigene Nachrichten bleiben schlicht.
function renderText(part, info) {
  const text = part.text || "";
  const finished = info.role === "assistant" && Boolean((part.time && part.time.end) || (info.time && info.time.completed));
  const html = markdownCache.get(text);
  if (finished && typeof html === "string") {
    const rendered = document.createElement("div");
    rendered.className = "doc-viewer chat-markdown";
    rendered.innerHTML = html;
    return rendered;
  }
  if (finished && html === undefined && text.trim() !== "") {
    requestMarkdown(text, info.id);
  }
  const plain = document.createElement("div");
  plain.className = "chat-text";
  plain.textContent = text;
  return plain;
}

// Sammelt Texte einen Augenblick und rendert sie in einem Aufruf: ein
// geladener Verlauf bringt viele auf einmal.
function requestMarkdown(text, messageID) {
  if (!markdownWaiting.has(text)) {
    markdownWaiting.set(text, new Set());
  }
  markdownWaiting.get(text).add(messageID);
  if (!markdownTimer) {
    markdownTimer = window.setTimeout(flushMarkdown, 30);
  }
}

async function flushMarkdown() {
  markdownTimer = 0;
  // Höchstens so viele wie der Server je Anfrage annimmt; der Rest folgt.
  const batch = [...markdownWaiting.entries()].slice(0, 200);
  batch.forEach(([text]) => markdownWaiting.delete(text));
  if (batch.length === 0) {
    return;
  }
  try {
    const data = await chatApi("/api/chat/markdown", { body: { texts: batch.map(([text]) => text) } });
    batch.forEach(([text], index) => markdownCache.set(text, data.html[index]));
  } catch (error) {
    // Ohne Rendern bleibt der schlichte Text stehen; lesbar ist er trotzdem.
    batch.forEach(([text]) => markdownCache.set(text, false));
    elements.message.textContent = error.message;
  }
  new Set(batch.flatMap(([, ids]) => [...ids])).forEach((id) => refreshMessage(id));
  if (markdownWaiting.size > 0 && !markdownTimer) {
    markdownTimer = window.setTimeout(flushMarkdown, 30);
  }
}

function renderTool(part) {
  const state = part.state || {};
  const details = document.createElement("details");
  details.className = "chat-tool";
  details.dataset.part = part.id;

  const summary = document.createElement("summary");
  const name = document.createElement("span");
  name.className = "chat-tool-name";
  name.textContent = part.tool;
  const title = document.createElement("span");
  title.className = "chat-tool-title";
  title.textContent = state.title || "";
  const pill = document.createElement("span");
  pill.className = `pill ${{ running: "warn", completed: "ok", error: "error" }[state.status] || "muted"}`;
  pill.textContent = { pending: "Wartet", running: "Läuft", completed: "Fertig", error: "Fehler" }[state.status] || "Unbekannt";
  summary.append(name, title);
  // Ein Aufruf des Werkzeugs task läuft in einer eigenen Sitzung; deren Kennung
  // steht in den Metadaten. Der Link steht in der Kopfzeile, damit er ohne
  // Aufklappen zu sehen ist — der Klick darf sie deshalb nicht umschalten.
  const child = part.tool === "task" && state.metadata && state.metadata.sessionId;
  if (child) {
    const open = document.createElement("a");
    open.className = "chat-back chat-tool-open";
    open.href = `/chat/${encodeURIComponent(child)}`;
    open.textContent = "Subagent öffnen";
    open.addEventListener("click", (event) => event.stopPropagation());
    summary.append(open);
  }
  summary.append(pill);
  details.append(summary);

  if (state.input && Object.keys(state.input).length > 0) {
    details.append(snippet(JSON.stringify(state.input, null, 2)));
  }
  const result = state.status === "error" ? state.error : state.output;
  if (result) {
    details.append(snippet(result));
  }
  return details;
}

function snippet(text) {
  const pre = document.createElement("pre");
  pre.className = "snippet";
  pre.textContent = text;
  return pre;
}

// --- Laufzustand, Senden, Abbrechen -----------------------------------------

function setRunPill(state, text) {
  elements.runPill.className = `pill ${state}`;
  elements.runPill.textContent = text;
}

// Nimmt auf, was die Ereignisse über den Lauf sagen. Die Marke entscheidet
// renderRunState() daraus zusammen mit den offenen Anfragen.
function setRunState(type, status) {
  run.type = type === "busy" || type === "retry" ? type : "idle";
  if (type === "retry") {
    if (status && status.message) {
      elements.message.textContent = status.message;
    }
  }
  if (type === "busy") {
    // Der Lauf geht weiter; ein Fehler von vorhin ist damit erledigt.
    run.failed = false;
  }
  renderRunState();
}

// Die Verbindungsmarken gehören in dieselbe Vorrangliste wie der Laufzustand:
// steht die Verbindung nicht, überschreibt kein Ereignis die Marke.
function setConnectionState(state) {
  run.connection = state === "ok" ? "" : state;
  renderRunState();
}

// Eine Marke, die die Sitzung abschließt. Sie steht über allem.
function setFixedRun(state, text) {
  run.fixed = { state, text };
  renderRunState();
}

// Eine offene Rückfrage der eigenen oder einer Kind-Sitzung oder eine offene
// Freigabe der eigenen Sitzung hält den Lauf an. Freigaben aus Kind-Sitzungen
// kennt die Seite nicht; sie werden auf deren eigener Seite beantwortet.
function hasOpenRequests() {
  if (chat.questions.size > 0) {
    return true;
  }
  for (const request of chat.permissions.values()) {
    if (request.sessionID === sessionID) {
      return true;
    }
  }
  return false;
}

// Vorrang von oben nach unten: Getrennt/Verbinde neu → Fehler → Wartet auf
// Antwort → Arbeitet/Wiederholt → Bereit.
function renderRunState() {
  const busy = run.type === "busy" || run.type === "retry";
  // Die Sichtbarkeit des Abbrechen-Knopfes folgt dem Lauf, nicht der Marke:
  // bei einer offenen Rückfrage läuft der Lauf weiter und bleibt abbrechbar.
  elements.abort.hidden = !busy || Boolean(run.fixed);

  if (run.fixed) {
    setRunPill(run.fixed.state, run.fixed.text);
  } else if (run.connection === "closed") {
    setRunPill("error", "Getrennt");
  } else if (run.connection === "reconnecting") {
    setRunPill("warn", "Verbinde neu");
  } else if (run.failed) {
    setRunPill("error", "Fehler");
  } else if (hasOpenRequests()) {
    setRunPill("warn", "Wartet auf Antwort");
  } else if (run.type === "retry") {
    setRunPill("warn", "Wiederholt");
  } else if (busy) {
    setRunPill("warn", "Arbeitet");
  } else {
    setRunPill("ok", "Bereit");
  }
}

function setFormEnabled(enabled) {
  const usable = enabled && chat.available;
  elements.input.disabled = !usable;
  elements.send.disabled = !usable;
}

async function sendPrompt() {
  const text = elements.input.value;
  if (text.trim() === "" || elements.send.disabled) {
    return;
  }
  elements.send.disabled = true;
  elements.message.textContent = "";
  // Ein Fehler von vorhin endet mit dem nächsten Senden.
  run.failed = false;
  renderRunState();
  const command = commandFromText(text);
  const agent = selectedAgent();
  // Die eigene Nachricht steht sofort da; ihre Kennung kommt mit der Antwort.
  showPendingMessage(text);
  try {
    let data;
    if (command) {
      data = await chatApi(`/api/chat/sessions/${encodeURIComponent(sessionID)}/command`, {
        body: { command: command.name, arguments: command.arguments, agent },
      });
    } else {
      data = await chatApi(`/api/chat/sessions/${encodeURIComponent(sessionID)}/prompt`, { body: { text, agent } });
    }
    // Ohne Kennung bleibt die Blase ohne Schlüssel; verworfen wird sie dann
    // beim ersten unbekannten message.updated mit role: "user".
    pending.id = (data && data.messageID) || "";
    elements.input.value = "";
    closeSuggestions();
    setRunState("busy");
    if (command) {
      if (command.subtask) {
        showSubtaskHint();
      }
      // Der Aufruf läuft abgekoppelt weiter; sein Ausgang kommt nur über den
      // Wächter zurück.
      watchCommandState();
    }
  } catch (error) {
    clearPendingMessage();
    elements.message.textContent = error.message;
  } finally {
    setFormEnabled(true);
  }
}

async function abortRun() {
  elements.abort.disabled = true;
  try {
    await chatApi(`/api/chat/sessions/${encodeURIComponent(sessionID)}/abort`, { method: "POST" });
  } catch (error) {
    elements.message.textContent = error.message;
  } finally {
    elements.abort.disabled = false;
  }
}

// --- Agenten ----------------------------------------------------------------
//
// Angeboten wird, was GET /api/chat/agents liefert: alles außer Subagenten und
// versteckten. Die Vorgabe steht in der Sitzung; gewählt wird sie bei Text und
// Command gleichermaßen mitgesendet.

async function loadAgents() {
  let agents;
  try {
    agents = await chatApi("/api/chat/agents");
  } catch {
    // Ohne Liste bleibt die Auswahl verborgen. Gesendet wird dann ohne agent,
    // und OpenCode nimmt den Agenten der Sitzung — wie bisher.
    return;
  }
  const list = (Array.isArray(agents) ? agents : []).filter((agent) => agent && agent.name);
  if (list.length === 0) {
    return;
  }
  elements.agent.replaceChildren(
    ...list.map((agent) => {
      const option = document.createElement("option");
      option.value = agent.name;
      option.textContent = agent.name;
      if (agent.description) {
        option.title = agent.description;
      }
      return option;
    }),
  );
  // Vorgabe aus der Sitzung; kennt die Liste sie nicht, bleibt der erste Eintrag.
  const preset = chat.session && chat.session.agent;
  if (preset && list.some((agent) => agent.name === preset)) {
    elements.agent.value = preset;
  }
  elements.agentRow.hidden = false;
}

// Der gewählte Agent oder "" — leer heißt: OpenCode entscheidet selbst.
function selectedAgent() {
  return elements.agentRow.hidden ? "" : elements.agent.value;
}

// --- Freigaben --------------------------------------------------------------

async function loadPermissions() {
  try {
    const requests = await chatApi("/api/chat/permissions");
    chat.permissions = new Map((Array.isArray(requests) ? requests : []).map((request) => [request.id, request]));
  } catch (error) {
    elements.message.textContent = error.message;
  }
  renderPermissions();
}

function renderPermissions() {
  elements.permissions.replaceChildren();
  const open = [...chat.permissions.values()].filter((request) => request.sessionID === sessionID);
  elements.permissions.hidden = open.length === 0;
  // Eine offene Freigabe hält den Lauf an; die Marke entscheidet danach die
  // Vorrangliste.
  renderRunState();
  for (const request of open) {
    const box = document.createElement("div");
    box.className = "chat-permission";
    const text = document.createElement("p");
    text.textContent = `OpenCode bittet um Freigabe: ${request.permission}`;
    box.append(text);
    if (request.patterns && request.patterns.length > 0) {
      box.append(snippet(request.patterns.join("\n")));
    }
    const actions = document.createElement("div");
    actions.className = "chat-actions";
    for (const [reply, label, style] of [
      ["once", "Einmal erlauben", "primary"],
      ["always", "Immer erlauben", "secondary"],
      ["reject", "Ablehnen", "secondary"],
    ]) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = style;
      button.textContent = label;
      button.addEventListener("click", () => replyPermission(request.id, reply, actions));
      actions.append(button);
    }
    box.append(actions);
    elements.permissions.append(box);
  }
}

async function replyPermission(id, reply, actions) {
  const buttons = actions.querySelectorAll("button");
  buttons.forEach((button) => {
    button.disabled = true;
  });
  try {
    await chatApi(`/api/chat/permissions/${encodeURIComponent(id)}/reply`, { body: { reply } });
    chat.permissions.delete(id);
    renderPermissions();
  } catch (error) {
    elements.message.textContent = error.message;
    buttons.forEach((button) => {
      button.disabled = false;
    });
  }
}

// --- Rückfragen -------------------------------------------------------------
//
// Eine Rückfrage hält den Lauf an, bis sie beantwortet oder abgelehnt ist.
// Sie steht in einem eigenen Dock über dem Formular, nicht im Verlauf: der
// wird bei jedem Delta neu gebaut und verlöre laufend die halb getroffene
// Auswahl und den getippten Eigentext.
//
// GET /api/chat/questions gilt über alle Sitzungen. Gezeigt werden die
// Anfragen der eigenen Sitzung und ihrer Nachkommen — ein /k-…-Command mit
// subtask: true läuft in einer Kind-Sitzung, und bliebe dessen Rückfrage
// unsichtbar, hinge der Lauf.

// Holt die Kind-Sitzungen und danach die offenen Rückfragen. Die Reihenfolge
// spart Anfragen: was als Kind bekannt ist, braucht keine Elternkette.
async function loadRelated() {
  await loadChildren();
  await loadQuestions();
}

async function loadChildren() {
  try {
    const children = await chatApi(`/api/chat/sessions/${encodeURIComponent(sessionID)}/children`);
    for (const child of Array.isArray(children) ? children : []) {
      if (child && child.id) {
        noteChildSession(child.id);
      }
    }
  } catch {
    // Ohne die Liste bleibt die Prüfung über die Elternkette; sie trägt allein.
  }
}

async function loadQuestions() {
  let requests;
  try {
    requests = await chatApi("/api/chat/questions");
  } catch (error) {
    elements.message.textContent = error.message;
    return;
  }
  const open = (Array.isArray(requests) ? requests : []).filter((request) => request && request.id);
  // Selbstheilung nach einer Trennung: eine Anfrage, die anderswo beantwortet
  // wurde, fehlt hier — ihr Kasten muss weg, sonst hielte er den Laufzustand
  // dauerhaft auf „Wartet auf Antwort". Die enthaltenen bleiben unberührt,
  // Auswahl und Eigentext also gerettet.
  const known = new Set(open.map((request) => request.id));
  for (const id of [...chat.questions.keys()]) {
    if (!known.has(id)) {
      chat.questions.delete(id);
    }
  }
  renderQuestions();
  // Neue werden einzeln angeboten: die Herkunftsprüfung kann warten müssen.
  await Promise.all(open.map(offerQuestion));
}

// Nimmt eine Anfrage an, wenn sie zu dieser Seite gehört.
async function offerQuestion(request) {
  if (!request || !request.id || !request.sessionID || chat.questions.has(request.id)) {
    return;
  }
  if (!(await belongsHere(request.sessionID))) {
    return;
  }
  chat.questions.set(request.id, request);
  renderQuestions();
}

// Gehört die Sitzung zu dieser Seite? Eine Liste bekannter Kinder allein
// verwürfe eine Rückfrage, die vor session.created oder vor dem task-Teil
// eintrifft, und jede aus einer Enkel-Sitzung. Bei unbekannter Sitzung wird
// deshalb die Elternkette verfolgt — höchstens eine Anfrage je fremder
// Sitzung, das Ergebnis wird gemerkt.
async function belongsHere(id) {
  if (id === sessionID) {
    return true;
  }
  const known = chat.related.get(id);
  if (known !== undefined) {
    return known;
  }
  let check = chat.relatedChecks.get(id);
  if (!check) {
    check = walkParents(id);
    chat.relatedChecks.set(id, check);
  }
  return check;
}

async function walkParents(id) {
  const chain = [];
  let current = id;
  let result = false;
  while (current && current !== sessionID) {
    if (chat.related.has(current)) {
      result = chat.related.get(current);
      break;
    }
    if (chain.includes(current)) {
      // Eine Kette, die sich schließt, führt nie zur eigenen Sitzung.
      break;
    }
    chain.push(current);
    let session;
    try {
      session = await chatApi(`/api/chat/sessions/${encodeURIComponent(current)}`);
    } catch {
      // Keine Auskunft ist kein Ergebnis: nichts wird gemerkt, damit eine
      // spätere Rückfrage derselben Sitzung es erneut versuchen kann.
      chat.relatedChecks.delete(id);
      return false;
    }
    current = (session && session.parentID) || "";
  }
  if (current === sessionID) {
    result = true;
  }
  for (const step of chain) {
    chat.related.set(step, result);
  }
  chat.relatedChecks.delete(id);
  return result;
}

// Gleicht das Dock ab, statt es neu zu bauen: ein stehender Kasten behält
// getroffene Auswahl und getippten Eigentext, auch wenn nebenan eine weitere
// Anfrage eintrifft oder eine andere beantwortet wird.
function renderQuestions() {
  const dock = elements.questions;
  for (const box of [...dock.children]) {
    if (!chat.questions.has(box.dataset.question)) {
      box.remove();
    }
  }
  for (const [id, request] of chat.questions) {
    if (!dock.querySelector(`[data-question="${CSS.escape(id)}"]`)) {
      dock.append(renderQuestion(request));
    }
  }
  dock.hidden = dock.children.length === 0;
  renderRunState();
}

// Ein Kasten je Anfrage, alle ihre Fragen untereinander, ein „Antworten" für
// die ganze Anfrage und ein „Ablehnen".
function renderQuestion(request) {
  const box = document.createElement("div");
  box.className = "chat-permission chat-question";
  box.dataset.question = request.id;

  if (request.sessionID !== sessionID) {
    const origin = document.createElement("p");
    origin.className = "chat-question-origin";
    origin.append("Rückfrage aus einer Kind-Sitzung. ");
    const link = document.createElement("a");
    link.className = "chat-back";
    link.href = `/chat/${encodeURIComponent(request.sessionID)}`;
    link.textContent = "Kind-Sitzung öffnen";
    origin.append(link);
    box.append(origin);
  }

  const questions = Array.isArray(request.questions) ? request.questions : [];
  questions.forEach((question, index) => {
    box.append(renderQuestionItem(request.id, question, index));
  });

  const actions = document.createElement("div");
  actions.className = "chat-actions";
  const reply = document.createElement("button");
  reply.type = "button";
  reply.className = "primary";
  reply.textContent = "Antworten";
  reply.addEventListener("click", () => {
    sendQuestion(request.id, actions, "reply", { answers: collectAnswers(request, box) });
  });
  const reject = document.createElement("button");
  reject.type = "button";
  reject.className = "secondary";
  reject.textContent = "Ablehnen";
  reject.addEventListener("click", () => {
    sendQuestion(request.id, actions, "reject", undefined);
  });
  actions.append(reply, reject);
  box.append(actions);
  return box;
}

function renderQuestionItem(requestID, question, index) {
  const item = document.createElement("div");
  item.className = "chat-question-item";
  item.dataset.index = String(index);

  const header = document.createElement("p");
  header.className = "eyebrow";
  header.textContent = question.header || `Frage ${index + 1}`;
  item.append(header);

  const text = document.createElement("p");
  text.className = "chat-question-text";
  text.textContent = question.question || "";
  item.append(text);

  const options = Array.isArray(question.options) ? question.options : [];
  if (options.length > 0) {
    const list = document.createElement("div");
    list.className = "chat-question-options";
    // Der Name bindet die Radios einer Frage zusammen. Er trägt die Kennung
    // der Anfrage mit, sonst griffen zwei Kästen nebeneinander ineinander.
    const group = `question-${requestID}-${index}`;
    for (const option of options) {
      const label = document.createElement("label");
      label.className = "chat-option";
      const input = document.createElement("input");
      input.type = question.multiple ? "checkbox" : "radio";
      input.name = group;
      input.value = option.label;
      const caption = document.createElement("span");
      caption.textContent = option.label;
      label.append(input, caption);
      if (option.description) {
        const description = document.createElement("span");
        description.className = "chat-option-description";
        description.textContent = option.description;
        label.append(description);
      }
      list.append(label);
    }
    item.append(list);
  }

  // Die eigene Antwort steht immer bereit: die Web-App von OpenCode wertet das
  // Schema-Feld custom nirgends aus, ihr Eigenantwortfeld ist immer da.
  const own = document.createElement("input");
  own.type = "text";
  own.className = "chat-question-custom";
  own.placeholder = "Eigene Antwort (optional)";
  item.append(own);
  return item;
}

// Je Frage ein Eintrag in der Reihenfolge der Fragen, darin die label-Werte
// der gewählten Optionen; eine unbeantwortete Frage bekommt eine leere Liste,
// damit die Positionen stimmen. Eine eigene Antwort ist ein eigener Eintrag
// derselben Liste: bei Einfachauswahl ersetzt sie die Wahl, bei
// Mehrfachauswahl steht sie neben den gewählten Labeln.
function collectAnswers(request, box) {
  const questions = Array.isArray(request.questions) ? request.questions : [];
  return questions.map((question, index) => {
    const item = box.querySelector(`[data-index="${index}"]`);
    if (!item) {
      return [];
    }
    const chosen = [...item.querySelectorAll("input[type=radio], input[type=checkbox]")]
      .filter((input) => input.checked)
      .map((input) => input.value);
    const field = item.querySelector(".chat-question-custom");
    const own = field ? field.value.trim() : "";
    if (own === "") {
      return chosen;
    }
    return question.multiple ? [...chosen, own] : [own];
  });
}

async function sendQuestion(id, actions, action, body) {
  const buttons = actions.querySelectorAll("button");
  buttons.forEach((button) => {
    button.disabled = true;
  });
  try {
    await chatApi(`/api/chat/questions/${encodeURIComponent(id)}/${action}`, { method: "POST", body });
    chat.questions.delete(id);
    renderQuestions();
  } catch (error) {
    elements.message.textContent = error.message;
    buttons.forEach((button) => {
      button.disabled = false;
    });
  }
}

// --- Commands ---------------------------------------------------------------
//
// Ein Text, dessen erstes Wort /name ist und dessen name in der gefilterten
// Liste steht, läuft als Command — die Weiche der Web-App von OpenCode. Alles
// andere geht als Text. Die internen Bausteine mit führendem _ stehen nicht in
// der Liste; ein von Hand getipptes /_docs:code ist damit gewöhnlicher Text.
//
// Der Server koppelt den Aufruf ab und antwortet sofort. Scheitert er danach,
// entsteht kein Ereignis — dafür holt der Wächter unten den Ausgang ab.

// So oft wird der Ausgang geholt, solange er noch läuft.
const COMMAND_STATE_DELAY = 5000;

const commandWatch = {
  // Steht ein von dieser Seite gesendeter Command noch aus?
  active: false,
  timer: 0,
  busy: false,
};

// Der Hinweis nach einem Command mit subtask: true. Der Link auf die
// Kind-Sitzung kommt nach, sobald ihre Kennung bekannt ist.
const subtaskHint = { slot: null };

async function loadCommands() {
  try {
    const commands = await chatApi("/api/chat/commands");
    chat.commands = new Map(
      (Array.isArray(commands) ? commands : []).filter((command) => command && command.name).map((command) => [command.name, command]),
    );
  } catch {
    // Ohne die Liste ist jeder Text ein Text. Das ist kein Grund, die Eingabe
    // zu sperren oder eine Meldung über den Verlauf zu legen.
  }
  // Wer schon getippt hat, während die Liste noch unterwegs war, bekommt die
  // Vorschläge jetzt nachgereicht.
  if (document.activeElement === elements.input) {
    updateSuggestions();
  }
}

// Erstes Wort /name und name bekannt → Command, der Rest wird zu arguments.
//
// Getrennt wird an beliebigem Leerraum, nicht nur am Leerzeichen: wer nach dem
// Namen Umschalt+Enter drückt und die Argumente in die nächste Zeile schreibt,
// meint denselben Command — am Leerzeichen allein getrennt ginge „/k-todo\nfoo"
// als gewöhnlicher Text hinaus.
function commandFromText(text) {
  const match = /^\/(\S+)\s*([\s\S]*)$/.exec(text);
  if (!match) {
    return null;
  }
  const command = chat.commands.get(match[1]);
  if (!command) {
    return null;
  }
  return { name: command.name, arguments: match[2], subtask: Boolean(command.subtask) };
}

// Ein Command mit subtask: true läuft in einer Kind-Sitzung. Deren Fortschritt
// erscheint hier nur als task-Teil, und ihre Freigaben zeigt diese Seite gar
// nicht — sie sind allein auf der Seite der Kind-Sitzung zu beantworten.
function showSubtaskHint() {
  elements.message.replaceChildren(
    "Der Command läuft als Unteraufgabe: der Fortschritt entsteht in einer Kind-Sitzung. " +
      "Rückfragen von dort zeigt diese Seite, Freigaben (Bash, Datei schreiben) nicht — die sind auf der Seite der Kind-Sitzung zu beantworten. ",
  );
  const slot = document.createElement("span");
  elements.message.append(slot);
  subtaskHint.slot = slot;
}

// Hängt den Link an, sobald die Kind-Sitzung bekannt ist. Steht der Hinweis
// nicht mehr — eine andere Meldung hat ihn ersetzt —, ist der Platzhalter aus
// dem Dokument gelöst und es passiert nichts.
function linkSubtaskHint(id) {
  const slot = subtaskHint.slot;
  if (!slot || !slot.isConnected || slot.children.length > 0) {
    return;
  }
  const link = document.createElement("a");
  link.className = "chat-back";
  link.href = `/chat/${encodeURIComponent(id)}`;
  link.textContent = "Kind-Sitzung öffnen";
  slot.append(link);
}

// Holt den Ausgang wiederholt, bis er nicht mehr „running" ist. Eine
// Stille-Frist trüge nicht: bei einem subtask-Command entstehen in dieser
// Sitzung womöglich gar keine Ereignisse, und umgekehrt setzte jedes beliebige
// Ereignis die Frist zurück.
function watchCommandState() {
  commandWatch.active = true;
  scheduleCommandState();
}

function scheduleCommandState() {
  if (!commandWatch.active || commandWatch.timer) {
    return;
  }
  commandWatch.timer = window.setTimeout(() => {
    commandWatch.timer = 0;
    pollCommandState();
  }, COMMAND_STATE_DELAY);
}

async function pollCommandState() {
  if (!commandWatch.active || commandWatch.busy) {
    return;
  }
  if (commandWatch.timer) {
    window.clearTimeout(commandWatch.timer);
    commandWatch.timer = 0;
  }
  commandWatch.busy = true;
  let data;
  try {
    data = await chatApi(`/api/chat/sessions/${encodeURIComponent(sessionID)}/command-state`);
  } catch {
    // Keine Auskunft ist kein Ausgang: beim nächsten Mal noch einmal.
    commandWatch.busy = false;
    scheduleCommandState();
    return;
  }
  commandWatch.busy = false;
  const state = data && data.state;
  if (state === "running") {
    scheduleCommandState();
    return;
  }
  commandWatch.active = false;
  if (state === "failed") {
    run.failed = true;
    renderRunState();
    elements.message.textContent = data.message || "Der Command ist gescheitert.";
  }
  // „done": von hier an führen die Ereignisse den Laufzustand. „unknown": kein
  // Eintrag — GUI neu gestartet oder Command aus dem Terminal; die Seite tut
  // nichts und lässt den Laufzustand unberührt.
}

// --- Vorschlagsliste ---------------------------------------------------------
//
// Steht am Anfang des Eingabefelds ein /, zeigt die Seite die passenden
// Commands darunter. Gespeist wird sie aus derselben Liste, die auch die Weiche
// beim Senden befragt — einmal je Seite geladen, ohne zweite Quelle.
//
// Geschlossen fasst sie keine Taste an: Enter sendet, Tab springt weiter, die
// Pfeiltasten bewegen die Schreibmarke.

// Höchstens so viele Vorschläge stehen unter dem Feld; mehr sind beim Tippen
// keine Hilfe mehr.
const SUGGESTION_LIMIT = 8;
// So lang darf eine Beschreibung in der Zeile werden, bevor sie gekürzt wird.
const SUGGESTION_LENGTH = 90;

const suggestions = {
  // Die angezeigten Commands in der Reihenfolge der Liste; leer heißt: zu.
  items: [],
  // Der Eintrag unter den Pfeiltasten, -1 solange die Liste zu ist.
  index: -1,
};

function suggestionsOpen() {
  return suggestions.items.length > 0;
}

// Der getippte Name oder null, wenn die Liste nichts beizutragen hat. Sobald
// ein Leerzeichen oder Zeilenumbruch folgt, ist der Name fertig getippt.
function suggestionQuery(value) {
  if (!value.startsWith("/")) {
    return null;
  }
  const head = value.slice(1);
  if (/\s/.test(head)) {
    return null;
  }
  return head.toLowerCase();
}

// Passend ist, wessen Name den getippten Teil enthält; wer damit anfängt, steht
// vorn. Der leere Anfang nach einem bloßen / zeigt alles.
function matchingCommands(query) {
  const hits = [...chat.commands.values()].filter((command) => command.name.toLowerCase().includes(query));
  hits.sort((a, b) => {
    const rank = Number(!a.name.toLowerCase().startsWith(query)) - Number(!b.name.toLowerCase().startsWith(query));
    return rank !== 0 ? rank : compareIDs(a.name, b.name);
  });
  return hits.slice(0, SUGGESTION_LIMIT);
}

function updateSuggestions() {
  const query = suggestionQuery(elements.input.value);
  const hits = query === null ? [] : matchingCommands(query);
  if (hits.length === 0) {
    closeSuggestions();
    return;
  }
  suggestions.items = hits;
  suggestions.index = 0;
  renderSuggestions();
}

function closeSuggestions() {
  suggestions.items = [];
  suggestions.index = -1;
  elements.suggestions.replaceChildren();
  elements.suggestions.hidden = true;
}

function renderSuggestions() {
  elements.suggestions.replaceChildren(...suggestions.items.map(renderSuggestion));
  elements.suggestions.hidden = false;
  showActiveSuggestion();
}

function renderSuggestion(command, index) {
  const entry = document.createElement("button");
  entry.type = "button";
  entry.className = "chat-suggestion";
  entry.setAttribute("role", "option");
  entry.dataset.index = String(index);

  const name = document.createElement("span");
  name.className = "chat-suggestion-name";
  name.textContent = `/${command.name}`;
  entry.append(name);

  const description = document.createElement("span");
  description.className = "chat-suggestion-description";
  description.textContent = shortenDescription(command.description);
  entry.append(description);

  // Kennzeichnungen: ein Skill ist kein gewöhnlicher Command, und ein Command
  // mit subtask: true läuft in einer Kind-Sitzung. Beide sind Beschriftungen
  // und tragen deshalb .pill statt einer eigenen Fläche.
  const marks = document.createElement("span");
  marks.className = "chat-suggestion-marks";
  if (command.source === "skill") {
    const skill = document.createElement("span");
    skill.className = "pill muted";
    skill.textContent = "Skill";
    marks.append(skill);
  }
  if (command.subtask) {
    const subtask = document.createElement("span");
    subtask.className = "pill warn";
    subtask.textContent = "läuft als Unteraufgabe";
    marks.append(subtask);
  }
  if (marks.children.length > 0) {
    entry.append(marks);
  }

  entry.addEventListener("click", () => applySuggestion(index));
  return entry;
}

function shortenDescription(text) {
  const clean = (text || "").replace(/\s+/g, " ").trim();
  if (clean.length <= SUGGESTION_LENGTH) {
    return clean;
  }
  return `${clean.slice(0, SUGGESTION_LENGTH - 1).trimEnd()}…`;
}

// Die Pfeiltasten laufen um: von unten wieder nach oben und umgekehrt.
function moveSuggestion(step) {
  const count = suggestions.items.length;
  suggestions.index = (suggestions.index + step + count) % count;
  showActiveSuggestion();
}

function showActiveSuggestion() {
  [...elements.suggestions.children].forEach((entry, index) => {
    const active = index === suggestions.index;
    entry.classList.toggle("active", active);
    entry.setAttribute("aria-selected", active ? "true" : "false");
    if (active) {
      entry.scrollIntoView({ block: "nearest" });
    }
  });
}

// Übernimmt den Vorschlag als „/name " — mit Leerzeichen, damit die Argumente
// gleich weitergetippt werden können und die Weiche beim Senden greift.
function applySuggestion(index) {
  const command = suggestions.items[index];
  if (!command) {
    return;
  }
  closeSuggestions();
  elements.input.value = `/${command.name} `;
  elements.input.focus();
  const end = elements.input.value.length;
  elements.input.setSelectionRange(end, end);
}

// Gibt true zurück, wenn die Liste die Taste verbraucht hat. Geschlossen gibt
// sie jede Taste weiter, und auch offen bleiben Umschalt+Enter und
// Umschalt+Tab dem Eingabefeld.
function handleSuggestionKey(event) {
  if (!suggestionsOpen() || event.isComposing) {
    return false;
  }
  switch (event.key) {
    case "ArrowDown":
      moveSuggestion(1);
      break;
    case "ArrowUp":
      moveSuggestion(-1);
      break;
    case "Enter":
    case "Tab":
      if (event.shiftKey) {
        return false;
      }
      applySuggestion(suggestions.index);
      break;
    case "Escape":
      closeSuggestions();
      break;
    default:
      return false;
  }
  event.preventDefault();
  return true;
}
