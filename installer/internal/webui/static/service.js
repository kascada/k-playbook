"use strict";

// Dienst und Update: die Knöpfe im Kopf und die Sperrfläche, wenn der Dienst
// weg ist. Zwei Seiten laden diese Datei, die Statusseite und /setup.
//
// Die Knöpfe stehen nicht überall, wo die Datei geladen wird: hero.html zeigt
// sie auf der Statusseite und auf /setup nur, solange keine
// Projektkonfiguration besteht. Die Sperrfläche dagegen tragen beide Seiten,
// und showClosed ist ihr Weg beim Serververlust — /setup ruft
// startSession(showClosed) auch ohne Knöpfe. Deshalb bindet die Datei die
// Knöpfe nur, wenn es sie gibt: ein addEventListener auf null bräche sie beim
// Laden ab, und mit ihr fehlte showClosed.
//
// Klassisches Skript, kein Modul: jeder Name auf oberster Ebene teilt sich den
// Namensraum mit session.js, nav.js und den Skripten der jeweiligen Seite.

const serviceElements = {
  shutdown: document.getElementById("shutdown"),
  update: document.getElementById("update"),
  // Nur auf der Statusseite. Dorthin schreibt die Update-Prüfung, wenn sie
  // nicht prüfen konnte.
  statusMessage: document.getElementById("status-message"),
  closed: document.getElementById("closed"),
  closedTitle: document.getElementById("closed-title"),
  closedMessage: document.getElementById("closed-message"),
  closedReconnect: document.getElementById("closed-reconnect"),
  closedHint: document.getElementById("closed-hint"),
};

// updateAvailable steuert, was ein Klick auf den Button tut: prüfen oder
// tatsächlich aktualisieren.
let updateAvailable = false;

serviceElements.closedReconnect.addEventListener("click", onReconnectClick);
if (serviceElements.shutdown) {
  serviceElements.shutdown.addEventListener("click", shutdown);
}
if (serviceElements.update) {
  serviceElements.update.addEventListener("click", onUpdateClick);
  // Die Update-Prüfung braucht das Netz. Sie läuft nebenher, damit die Seite
  // nicht auf einen langsamen Remote wartet.
  checkUpdate();
}

// Der Knopf bleibt während der Prüfung verborgen: sichtbar wird er erst, wenn
// es wirklich ein Update gibt.
async function checkUpdate() {
  try {
    const response = await fetch("/api/update", { cache: "no-store" });
    const data = await response.json();
    renderUpdate(data);
    showUpdateMessage(data.available ? "" : data.message);
  } catch {
    resetUpdateButton();
    showUpdateMessage("Die Update-Prüfung hat keine Antwort bekommen.");
  }
}

async function onUpdateClick() {
  if (!updateAvailable) {
    await checkUpdate();
    return;
  }

  serviceElements.update.disabled = true;
  serviceElements.update.textContent = "Aktualisiere...";
  try {
    const response = await fetch("/api/update", { method: "POST" });
    const data = await response.json();
    renderUpdate(data);
    // Ein gescheitertes Aktualisieren verbirgt den Knopf; ohne Meldung wäre
    // es wortlos verschwunden.
    if (!response.ok) {
      showUpdateMessage(data.message);
    }
    if (data.restartRequired) {
      // Der Dienst beendet sich nach dieser Antwort selbst: zum neuen Stand
      // gehört ein anderes Binary, und ein alter Daemon soll nicht stehen
      // bleiben.
      //
      // Der Bootstrap steht hier in derselben kanonischen Form wie in
      // project.BootstrapHint und in der Dokumentation: ein Zielprojekt hat
      // kein eigenes install-Target, der Aufruf geht über den Clone.
      showClosed(
        "Das Programm wurde aktualisiert. Der Dienst hat sich beendet; " +
          "neu installieren mit: make -C k-playbook install " +
          "(ohne make: k-playbook/bin/install). " +
          "Danach k-playbook erneut aufrufen."
      );
    }
  } catch {
    resetUpdateButton();
    showUpdateMessage("Das Update hat keine Antwort bekommen.");
  }
}

// Drei Zustände kommen an: Update vorhanden, geprüft und gleich, und nicht
// prüfbar (data.message). Sichtbar ist der Knopf nur im ersten; der dritte
// geht als Meldung in die Statuskarte, siehe showUpdateMessage.
function renderUpdate(data) {
  updateAvailable = Boolean(data.available);

  if (updateAvailable) {
    // Hervorgehoben, solange etwas anliegt.
    serviceElements.update.className = "primary attention-highlight";
    serviceElements.update.textContent = "Update verfügbar";
    serviceElements.update.title = `${data.local} -> ${data.remote} (${data.branch})`;
    serviceElements.update.disabled = false;
    return;
  }

  resetUpdateButton();
  serviceElements.update.title = data.message || `Stand ${data.local || "unbekannt"} (${data.branch || "?"})`;
}

function resetUpdateButton() {
  updateAvailable = false;
  serviceElements.update.className = "secondary hidden";
  serviceElements.update.textContent = "Update prüfen";
  serviceElements.update.disabled = false;
}

// Schreibt in die Meldung der Statuskarte; leer verbirgt sie wieder. Auf
// /setup gibt es die Karte nicht, dort bleibt es beim Titel des verborgenen
// Knopfes.
function showUpdateMessage(text) {
  if (!serviceElements.statusMessage) {
    return;
  }
  serviceElements.statusMessage.textContent = text || "";
  serviceElements.statusMessage.classList.toggle("hidden", !text);
}

async function shutdown() {
  serviceElements.shutdown.disabled = true;
  try {
    await fetch("/api/shutdown", { method: "POST" });
  } catch {
    // Das Backend darf die Antwort schuldig bleiben, wenn es sofort zumacht.
  }
  showClosed();
}

// Sperrt die Seite, weil der Dienst weg ist — ob auf Knopfdruck hier, aus
// einem anderen Fenster, per k-playbook stop oder weil er nach einem Update
// zugemacht hat. Er war für alle Fenster derselbe, also gilt das für alle.
// Der Weg zurück ist zuerst „Erneut verbinden"; der Hinweis auf das Terminal
// kommt erst, wenn auch das scheitert.
function showClosed(message = "") {
  serverAvailable = false;
  serviceElements.closedTitle.textContent = "Der Dienst ist beendet, für alle Fenster dieses Projekts.";
  serviceElements.closedMessage.textContent = message;
  serviceElements.closedMessage.classList.toggle("hidden", !message);
  serviceElements.closedHint.classList.add("hidden");
  serviceElements.closedReconnect.disabled = false;
  serviceElements.closedReconnect.textContent = "Erneut verbinden";
  serviceElements.closed.classList.remove("hidden");
}

async function onReconnectClick() {
  serviceElements.closedReconnect.disabled = true;
  serviceElements.closedReconnect.textContent = "Verbinde...";
  if (await reconnect()) {
    // Die Seite lädt neu; hier gibt es nichts mehr zu tun.
    return;
  }
  serviceElements.closedHint.classList.remove("hidden");
  serviceElements.closedReconnect.disabled = false;
  serviceElements.closedReconnect.textContent = "Erneut verbinden";
}
