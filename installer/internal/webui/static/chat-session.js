"use strict";

// Seite einer Chat-Sitzung (/chat/<id>): Verlauf, Eingabe und Freigaben.
//
// Die Seite hält nichts dauerhaft: sie lädt den Verlauf der Sitzung und spielt
// danach die Ereignisse des Stroms ein. Die Regeln dafür folgen der Web-App von
// OpenCode und stehen in installer/docs/architecture.md unter „Chat in der
// Oberfläche". Gemeinsames mit der Liste steht in chat-common.js.

const elements = {
  title: document.getElementById("chat-title"),
  runPill: document.getElementById("chat-run-pill"),
  log: document.getElementById("chat-log"),
  permissions: document.getElementById("chat-permissions"),
  form: document.getElementById("chat-form"),
  input: document.getElementById("chat-input"),
  send: document.getElementById("chat-send"),
  abort: document.getElementById("chat-abort"),
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
  // messageID → { info, parts: Map(partID → part) }. IDs von OpenCode sind
  // aufsteigend sortierbar, die Reihenfolge ergibt sich daraus.
  messages: new Map(),
  // requestID → PermissionRequest, über alle Sitzungen.
  permissions: new Map(),
  connectedOnce: false,
};

elements.form.addEventListener("submit", (event) => {
  event.preventDefault();
  sendPrompt();
});
elements.input.addEventListener("keydown", (event) => {
  if (event.key === "Enter" && !event.shiftKey && !event.isComposing) {
    event.preventDefault();
    sendPrompt();
  }
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
    showTitle(await chatApi(`/api/chat/sessions/${encodeURIComponent(sessionID)}`));
  } catch (error) {
    setRunPill("error", "Nicht gefunden");
    elements.message.textContent = error.message;
    return;
  }

  setRunState("idle");
  setFormEnabled(true);
  openChatEvents(applyEvent, (state) => {
    if (state === "closed") {
      setRunPill("error", "Getrennt");
    } else if (state === "reconnecting") {
      setRunPill("warn", "Verbinde neu");
    }
  });
  await Promise.all([loadMessages(), loadPermissions()]);
}

function showUnavailable(text) {
  setRunPill("error", "Nicht erreichbar");
  elements.message.textContent = `${text} Was zu tun ist, steht unter „Alle Sitzungen".`;
  setFormEnabled(false);
}

function showTitle(session) {
  elements.title.textContent = sessionTitle(session);
  document.title = `${sessionTitle(session)} - k-playbook`;
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
      }
      chat.connectedOnce = true;
      return;

    case "session.updated":
      if (props.info && props.info.id === sessionID) {
        showTitle(props.info);
      }
      return;
    case "session.deleted":
      if (((props.info && props.info.id) || props.sessionID) === sessionID) {
        setRunPill("error", "Gelöscht");
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
      }
      return;
    case "session.error":
      if (props.sessionID === sessionID && props.error) {
        elements.message.textContent = describeChatError(props.error);
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
  }
}

// --- Unterhaltung -----------------------------------------------------------

async function loadMessages() {
  try {
    const items = await chatApi(`/api/chat/sessions/${encodeURIComponent(sessionID)}/messages`);
    chat.messages = new Map();
    for (const item of items) {
      chat.messages.set(item.info.id, { info: item.info, parts: new Map(item.parts.map((part) => [part.id, part])) });
    }
    // Eine Antwort ohne Abschluss und ohne Fehler läuft noch.
    const last = [...chat.messages.values()].filter((entry) => entry.info.role === "assistant").pop();
    const running = last && !(last.info.time && last.info.time.completed) && !last.info.error;
    setRunState(running ? "busy" : "idle");
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
  if (ids.length === 0) {
    const empty = document.createElement("p");
    empty.className = "hint";
    empty.textContent = "Noch keine Nachricht. Unten die erste schreiben.";
    elements.log.append(empty);
    return;
  }
  for (const id of ids) {
    elements.log.append(renderMessage(chat.messages.get(id)));
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
  summary.append(name, title, pill);
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

function setRunState(type, status) {
  const busy = type === "busy" || type === "retry";
  if (type === "retry") {
    setRunPill("warn", "Wiederholt");
    if (status && status.message) {
      elements.message.textContent = status.message;
    }
  } else if (busy) {
    setRunPill("warn", "Arbeitet");
  } else {
    setRunPill("ok", "Bereit");
  }
  elements.abort.hidden = !busy;
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
  try {
    await chatApi(`/api/chat/sessions/${encodeURIComponent(sessionID)}/prompt`, { body: { text } });
    elements.input.value = "";
    setRunState("busy");
  } catch (error) {
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
