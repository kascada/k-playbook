"use strict";

// Gemeinsames der beiden Chat-Seiten — der Liste (/chat, chat.js) und einer
// Sitzung (/chat/<id>, chat-session.js): Anfragen an /api/chat/*, der
// Ereignisstrom des OpenCode-Dienstes und kleine Formatierer.

// Nach so vielen Millisekunden versucht eine Seite einen abgewiesenen
// Ereignisstrom erneut. Einen abgerissenen verbindet EventSource selbst neu.
const CHAT_RECONNECT_DELAY = 5000;

// Holt JSON von einem Chat-Endpunkt. Wirft mit der Meldung des Servers oder
// von OpenCode, damit jede Stelle sie gleich anzeigen kann.
async function chatApi(path, options = {}) {
  const request = { cache: "no-store", method: options.method || "GET" };
  if (options.body !== undefined) {
    request.method = options.method || "POST";
    request.headers = { "Content-Type": "application/json" };
    request.body = JSON.stringify(options.body);
  }
  const response = await fetch(path, request);
  let data = null;
  try {
    data = await response.json();
  } catch {
    data = null;
  }
  if (!response.ok) {
    const text = data && (data.message || (data.data && data.data.message) || data.name);
    throw new Error(text || `Anfrage fehlgeschlagen (Status ${response.status}).`);
  }
  return data;
}

// Öffnet den Ereignisstrom und hält ihn offen. onEvent bekommt jedes
// Ereignis, onState "ok", "reconnecting" oder "closed".
function openChatEvents(onEvent, onState) {
  const connect = () => {
    const source = new EventSource("/api/chat/events");
    source.onmessage = (message) => {
      let event;
      try {
        event = JSON.parse(message.data);
      } catch {
        return;
      }
      if (event.type === "server.connected") {
        onState("ok");
      }
      onEvent(event);
    };
    source.onerror = () => {
      if (source.readyState === EventSource.CLOSED) {
        // Abgewiesen, etwa weil der Dienst weg ist: EventSource gibt dann auf.
        onState("closed");
        window.setTimeout(connect, CHAT_RECONNECT_DELAY);
        return;
      }
      onState("reconnecting");
    };
  };
  connect();
}

function sessionTitle(session) {
  return session.title || session.id;
}

function sessionUpdatedAt(session) {
  return (session.time && (session.time.updated || session.time.created)) || 0;
}

function formatChatTime(milliseconds) {
  if (!milliseconds) {
    return "";
  }
  return new Date(milliseconds).toLocaleString("de-DE", { dateStyle: "short", timeStyle: "short" });
}

function describeChatError(error) {
  if (error.name === "MessageAbortedError") {
    return "Abgebrochen.";
  }
  return (error.data && error.data.message) || error.name || "Unbekannter Fehler.";
}

// Eine Zeile in einer .facts-Liste: dt und dd stehen in einem div, wie die
// Regeln im Stylesheet es erwarten.
function addChatFact(list, term, detail) {
  const row = document.createElement("div");
  const dt = document.createElement("dt");
  dt.textContent = term;
  const dd = document.createElement("dd");
  dd.textContent = detail;
  row.append(dt, dd);
  list.append(row);
}
