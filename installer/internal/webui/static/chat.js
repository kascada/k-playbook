"use strict";

// Seite "Chat": die Sitzungen dieses Projekts beim OpenCode-Dienst und dessen
// Zustand. Eine Sitzung öffnet sich auf ihrer eigenen Seite /chat/<id>, dort
// liegt die Unterhaltung (chat-session.js). Gemeinsames steht in
// chat-common.js.

const elements = {
  newSession: document.getElementById("chat-new-session"),
  sessionsPill: document.getElementById("chat-sessions-pill"),
  sessionList: document.getElementById("chat-session-list"),
  sessionsMessage: document.getElementById("chat-sessions-message"),
  connectionPill: document.getElementById("chat-connection-pill"),
  statusFacts: document.getElementById("chat-status-facts"),
  statusMessage: document.getElementById("chat-status-message"),
  statusHint: document.getElementById("chat-status-hint"),
};

// Muss vor den Ladefunktionen laufen: die setzen Pills, und das Menü zieht das
// nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();

startSession((message) => {
  setConnection("error", "Getrennt");
  elements.statusMessage.textContent = message;
  elements.newSession.disabled = true;
});

// Die obersten Sitzungen des Projekts, wie der Dienst sie liefert.
let sessions = [];

elements.newSession.addEventListener("click", createSession);

init();

async function init() {
  const available = await loadStatus();
  if (!available) {
    elements.sessionList.textContent = "Ohne erreichbaren OpenCode-Dienst gibt es keine Sitzungen.";
    elements.sessionsPill.className = "pill muted";
    elements.sessionsPill.textContent = "Keine";
    return;
  }
  elements.newSession.disabled = false;
  await loadSessions();
  // Die Liste folgt dem Dienst: neue, umbenannte und gelöschte Sitzungen
  // erscheinen ohne Neuladen.
  openChatEvents(applyEvent, (state) => {
    if (state === "ok") {
      setConnection("ok", "Verbunden");
    } else if (state === "closed") {
      setConnection("error", "Getrennt");
    } else {
      setConnection("warn", "Verbinde neu");
    }
  });
}

// --- Dienst -----------------------------------------------------------------

async function loadStatus() {
  let data;
  try {
    data = await chatApi("/api/chat/status");
  } catch (error) {
    setConnection("error", "Fehler");
    elements.statusMessage.textContent = error.message;
    return false;
  }

  elements.statusFacts.replaceChildren();
  addChatFact(elements.statusFacts, "Adresse", data.url);
  if (data.version) {
    addChatFact(elements.statusFacts, "Version", data.version);
  }
  if (data.directory) {
    addChatFact(elements.statusFacts, "Verzeichnis", data.directory);
  }
  addChatFact(elements.statusFacts, "Anmeldung", data.auth ? "mit Passwort" : "ohne Passwort");
  if (data.container) {
    addChatFact(elements.statusFacts, "Umgebung", "Container – nur ein Dienst im Container selbst");
  }
  elements.statusMessage.textContent = data.message || "";
  renderHint(data.hint);

  if (!data.installed) {
    setConnection("muted", "Kein Projekt");
  } else if (data.blocked) {
    setConnection("error", "Gesperrt");
  } else if (!data.available) {
    setConnection("error", "Nicht erreichbar");
  } else {
    setConnection("ok", "Verbunden");
  }
  return data.available;
}

function setConnection(state, text) {
  elements.connectionPill.className = `pill ${state}`;
  elements.connectionPill.textContent = text;
}

// Der Hinweis nennt, was zu tun ist, wenn kein Dienst benutzbar ist: Text und
// die Befehle zum Kopieren. Ausgeführt wird nichts.
function renderHint(hint) {
  elements.statusHint.replaceChildren();
  elements.statusHint.hidden = !hint;
  if (!hint) {
    return;
  }
  const text = document.createElement("p");
  text.textContent = hint.text;
  elements.statusHint.append(text);
  if (hint.commands && hint.commands.length > 0) {
    const commands = document.createElement("pre");
    commands.className = "snippet";
    commands.textContent = hint.commands.join("\n");
    elements.statusHint.append(commands);
  }
}

// --- Sitzungen --------------------------------------------------------------

async function loadSessions() {
  try {
    const list = await chatApi("/api/chat/sessions");
    sessions = (Array.isArray(list) ? list : []).filter((session) => !session.parentID);
    elements.sessionsMessage.textContent = "";
  } catch (error) {
    elements.sessionsMessage.textContent = error.message;
  }
  renderSessions();
}

function applyEvent(event) {
  const props = event.properties || {};
  switch (event.type) {
    case "server.connected":
      return;
    case "session.created":
    case "session.updated": {
      const info = props.info;
      // Kind-Sitzungen der Subagenten gehören zu ihrer Elternsitzung.
      if (!info || info.parentID) {
        return;
      }
      const index = sessions.findIndex((session) => session.id === info.id);
      if (index >= 0) {
        sessions[index] = info;
      } else {
        sessions.push(info);
      }
      renderSessions();
      return;
    }
    case "session.deleted": {
      const id = (props.info && props.info.id) || props.sessionID;
      sessions = sessions.filter((session) => session.id !== id);
      renderSessions();
      return;
    }
  }
}

function renderSessions() {
  elements.sessionList.replaceChildren();
  const sorted = [...sessions].sort((a, b) => sessionUpdatedAt(b) - sessionUpdatedAt(a));
  elements.sessionsPill.className = "pill muted";
  elements.sessionsPill.textContent = sorted.length === 1 ? "1 Sitzung" : `${sorted.length} Sitzungen`;
  if (sorted.length === 0) {
    elements.sessionList.classList.add("empty");
    elements.sessionList.textContent = "Noch keine Sitzung in diesem Projekt.";
    return;
  }
  elements.sessionList.classList.remove("empty");
  for (const session of sorted) {
    // Ein Verweis statt eines Knopfs: die Sitzung ist eine eigene Seite und
    // lässt sich so auch in einem neuen Tab öffnen.
    const item = document.createElement("a");
    item.className = "chat-session";
    item.href = `/chat/${encodeURIComponent(session.id)}`;
    const title = document.createElement("span");
    title.className = "chat-session-title";
    title.textContent = sessionTitle(session);
    const meta = document.createElement("span");
    meta.className = "chat-session-meta";
    meta.textContent = formatChatTime(sessionUpdatedAt(session));
    item.append(title, meta);
    elements.sessionList.append(item);
  }
}

async function createSession() {
  elements.newSession.disabled = true;
  elements.sessionsMessage.textContent = "";
  try {
    const session = await chatApi("/api/chat/sessions", { body: {} });
    window.location.href = `/chat/${encodeURIComponent(session.id)}`;
  } catch (error) {
    elements.sessionsMessage.textContent = error.message;
    elements.newSession.disabled = false;
  }
}
