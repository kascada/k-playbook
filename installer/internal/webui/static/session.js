"use strict";

// Die Sitzung zwischen Browser und Server, für jede Seite dieselbe.
//
// Der Server ist ein Hintergrunddienst je Projekt: er hängt nicht an diesem
// Fenster und beendet sich erst, wenn ihn eine Stunde lang niemand fragt. Das
// Lebenszeichen hier hält ihn aus dem Leerlauf — solange jemand hinsieht,
// läuft er — und merkt, wenn er weg ist: beendet aus einem anderen Fenster,
// per `k-playbook stop`, nach einem Update oder im Leerlauf.

// Der Server stirbt nicht mehr, wenn das Lebenszeichen ausbleibt; es muss nur
// deutlich unter der Leerlaufgrenze liegen. Deshalb viel seltener als früher.
const HEALTH_INTERVAL = 10000;

// Ein einzelner Fehlschlag sperrt nicht: ein kurzer Aussetzer soll die Seite
// nicht totstellen. Erst nach so vielen in Folge gilt der Server als weg.
const HEALTH_FAILURES_BEFORE_LOCK = 3;

// Hält fest, ob das Backend noch antwortet. Sobald es weg ist, wird die
// Oberfläche gesperrt und nicht weiter gepollt.
let serverAvailable = true;
let healthFailures = 0;

// Was geschehen soll, wenn der Server nicht mehr antwortet. Die Startseite
// legt dafür ein eigenes Fenster über die Seite; ohne eigene Behandlung bleibt
// es beim Sperren.
let serverLostHandler = () => {};

function startSession(handler) {
  if (handler) {
    serverLostHandler = handler;
  }
  window.setInterval(checkHealth, HEALTH_INTERVAL);
}

// Erkennt ein weggefallenes Backend — aber erst, wenn es mehrmals in Folge
// nicht geantwortet hat.
async function checkHealth() {
  if (!serverAvailable) {
    return;
  }

  if (await serverResponds()) {
    healthFailures = 0;
    return;
  }
  healthFailures += 1;
  if (healthFailures >= HEALTH_FAILURES_BEFORE_LOCK) {
    markServerGone("Verbindung zu k-playbook verloren.");
  }
}

// Fragt den Server einmal. Wahr, wenn er antwortet.
async function serverResponds() {
  try {
    const response = await fetch("/api/health", { cache: "no-store" });
    return response.ok;
  } catch {
    return false;
  }
}

function markServerGone(message) {
  serverAvailable = false;
  serverLostHandler(message);
}

// Versucht, einen gesperrten Stand wieder anzubinden: antwortet der Server,
// wird die Seite neu geladen — derselbe Weg wie „Neu einlesen". Falsch, wenn
// er nicht antwortet; der Aufrufer zeigt dann den Weg über das Terminal.
async function reconnect() {
  if (await serverResponds()) {
    window.location.reload();
    return true;
  }
  return false;
}
