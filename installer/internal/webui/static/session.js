"use strict";

// Die Sitzung zwischen Browser und Server, für jede Seite dieselbe.
//
// Der Server lebt nur, solange ein Fenster ihn braucht: bleibt das Lebenszeichen
// aus, macht er zu. Deshalb muss **jede** Seite es senden — sonst beendet sich
// der Server wenige Sekunden nach einem Seitenwechsel, und der Weg zurück führt
// auf eine tote Seite.

const HEALTH_INTERVAL = 1800;

// Hält fest, ob das Backend noch antwortet. Sobald es weg ist, wird die
// Oberfläche gesperrt und nicht weiter gepollt.
let serverAvailable = true;

// Ein Wechsel innerhalb der Oberfläche ist kein Abschied. Ohne diese
// Unterscheidung meldete jeder Klick auf eine andere Seite „Fenster weg", und
// der Server stünde bis zum Aufbau der nächsten Seite unter Countdown.
let leavingForOwnPage = false;

// Was geschehen soll, wenn der Server nicht mehr antwortet. Die Startseite
// legt dafür ein eigenes Fenster über die Seite; ohne eigene Behandlung bleibt
// es beim Sperren.
let serverLostHandler = () => {};

function startSession(handler) {
  if (handler) {
    serverLostHandler = handler;
  }
  window.setInterval(checkHealth, HEALTH_INTERVAL);
  window.addEventListener("pagehide", notifyClientGone);
  document.addEventListener("click", noteInternalNavigation);
}

// Erkennt ein weggefallenes Backend: schlägt der Aufruf fehl, ist der Server
// beendet worden.
async function checkHealth() {
  if (!serverAvailable) {
    return;
  }

  try {
    await fetch("/api/health", { cache: "no-store" });
  } catch {
    markServerGone("Verbindung zu k-playbook verloren.");
  }
}

function markServerGone(message) {
  serverAvailable = false;
  serverLostHandler(message);
}

// Meldet dem Backend, dass dieses Fenster verschwindet. sendBeacon überlebt
// das Entladen der Seite, fetch nicht zuverlässig.
function notifyClientGone() {
  if (!serverAvailable || leavingForOwnPage) {
    return;
  }

  if (navigator.sendBeacon) {
    navigator.sendBeacon("/api/client-gone");
    return;
  }

  fetch("/api/client-gone", { method: "POST", keepalive: true }).catch(() => {});
}

// Ein Klick auf einen Verweis dieser Oberfläche führt zur nächsten Seite und
// nicht aus ihr heraus. Der Zurück-Knopf löst keinen Klick aus und meldet
// deshalb weiterhin ab — die nächste Seite ist rechtzeitig da und hebt die
// Abmeldung mit ihrem ersten Lebenszeichen wieder auf.
function noteInternalNavigation(event) {
  const link = event.target.closest && event.target.closest("a[href]");
  if (!link || link.target === "_blank" || event.metaKey || event.ctrlKey || event.shiftKey) {
    return;
  }
  if (new URL(link.href, window.location.href).origin === window.location.origin) {
    leavingForOwnPage = true;
  }
}
