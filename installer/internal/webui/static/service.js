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

// updateMode steuert, was ein Klick auf den Button tut: "" prüft erneut,
// "pull" holt den neuen Stand des Clones, "program" installiert das zum Clone
// passende Programm und startet den Dienst daraus neu.
let updateMode = "";

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
// es wirklich etwas zu tun gibt.
async function checkUpdate() {
  try {
    const response = await fetch("/api/update", { cache: "no-store" });
    const data = await response.json();
    renderUpdate(data);
    showUpdateMessage(updateNotice(data));
  } catch {
    resetUpdateButton();
    showUpdateMessage("Die Update-Prüfung hat keine Antwort bekommen.");
  }
}

// updateNotice ist der Text der Statuskarte zur Prüfung. Ein älteres Programm
// wird immer genannt — mit Knopf als Hinweis, ohne Knopf mit dem Weg über das
// Terminal —, sonst die Meldung der Prüfung, wenn sie nicht prüfen konnte.
function updateNotice(data) {
  const program = data.program || {};
  if (program.outdated) {
    const text =
      `Das laufende Programm (${program.running}) ist älter als die Installation (${program.clone}).`;
    return program.installable ? text : `${text} ${program.hint}`;
  }
  return data.available ? "" : data.message;
}

async function onUpdateClick() {
  if (updateMode === "program") {
    await updateProgram(false);
    return;
  }
  if (updateMode !== "pull") {
    await checkUpdate();
    return;
  }

  serviceElements.update.disabled = true;
  serviceElements.update.textContent = "Aktualisiere...";
  try {
    const response = await fetch("/api/update", { method: "POST" });
    const data = await response.json();
    if (data.restarted) {
      await followRestart(data);
      return;
    }
    renderUpdate(data);
    // Ein gescheitertes Aktualisieren verbirgt den Knopf; ohne Meldung wäre
    // es wortlos verschwunden. Nach einem Pull, der ein neueres Programm
    // verlangt, das nicht installiert wurde, steht der Grund in der Meldung.
    if (!response.ok || data.installFailed || (data.program && data.program.outdated)) {
      showUpdateMessage(withOutput(data.message, data.installOutput));
    }
  } catch {
    resetUpdateButton();
    showUpdateMessage("Das Update hat keine Antwort bekommen.");
  }
}

// updateProgram installiert das zum Clone passende Programm und startet den
// Dienst daraus neu. Laufen Befehle oder Chats, antwortet der Server mit 428
// und der Rückfrage; bestätigt geht derselbe Aufruf noch einmal hinaus.
async function updateProgram(confirmed) {
  serviceElements.update.disabled = true;
  serviceElements.update.textContent = "Installiere Programm...";
  try {
    const url = confirmed ? "/api/update/program?confirm=1" : "/api/update/program";
    const response = await fetch(url, { method: "POST" });
    const data = await response.json();
    if (response.status === 428 && data.busy) {
      if (window.confirm(data.message)) {
        await updateProgram(true);
      } else {
        await checkUpdate();
      }
      return;
    }
    if (data.restarted) {
      await followRestart(data);
      return;
    }
    // Gescheitert: der Dienst läuft weiter. Die Meldung nennt Fehler, Ausgabe
    // und den Befehl zum Nachholen; die Sperrfläche wäre hier falsch.
    // Fällt die Antwort ohne Text aus, bleibt es beim Bootstrap in derselben
    // kanonischen Form wie in project.BootstrapHint: make -C k-playbook install
    // (ohne make: k-playbook/bin/install).
    await checkUpdate();
    showUpdateMessage(
      withOutput(
        data.message ||
          "Das Programm wurde nicht aktualisiert. Nachholen im Terminal: make -C k-playbook install " +
            "(ohne make: k-playbook/bin/install).",
        data.installOutput
      )
    );
  } catch {
    resetUpdateButton();
    showUpdateMessage(
      "Die Programmaktualisierung hat keine Antwort bekommen. Läuft der Dienst nicht mehr, im Terminal " +
        "make -C k-playbook install (ohne make: k-playbook/bin/install) und danach k-playbook aufrufen."
    );
  }
}

function withOutput(message, output) {
  return output ? `${message}\n\nAusgabe:\n${output}` : message;
}

// followRestart führt die Seite zum neuen Dienst. Dessen Port ist ein anderer:
// der Server bindet bei jedem Start auf 127.0.0.1:0. Ein reconnect() auf die
// alte Adresse fände nur den alten Dienst, der sich gerade beendet.
//
// Übernommen wird der Rechnername, unter dem diese Seite geöffnet ist, und nur
// der Port kommt aus der Antwort. Direkt geöffnet ist das 127.0.0.1; hinter
// einer Weiterleitung, die den Port gleich weiterreicht — der DevContainer von
// VS Code tut das, sobald er den neuen Port bemerkt —, bleibt es localhost.
//
// Gewechselt wird erst, wenn der neue Dienst von hier aus antwortet. Gefragt
// wird mit mode "no-cors": die Antwort bleibt unlesbar, aber ob überhaupt eine
// kommt, zeigt sie. Kommt keine, zeigt die Sperrfläche die neue Adresse und den
// Weg über das Terminal. Browser-Speicher der alten Adresse geht nicht mit
// über; das ist hingenommen.
async function followRestart(data) {
  // Das Lebenszeichen der alten Seite schweigt ab hier: der alte Dienst
  // beendet sich, und seine Sperrfläche wäre die falsche Nachricht.
  serverAvailable = false;
  serviceElements.update.disabled = true;
  serviceElements.update.textContent = `Neu gestartet (${data.version || "neue Version"})`;
  showUpdateMessage(`${data.message} Die Seite wechselt zum neuen Dienst.`);

  const port = new URL(data.url).port;
  const base = `${window.location.protocol}//${window.location.hostname}:${port}`;
  const next = `${base}${window.location.pathname}${window.location.search}${window.location.hash}`;
  for (let attempt = 0; attempt < 20; attempt += 1) {
    try {
      await fetch(`${base}/api/health`, { mode: "no-cors", cache: "no-store" });
      window.location.assign(next);
      return;
    } catch {
      await new Promise((resolve) => window.setTimeout(resolve, 500));
    }
  }
  showClosed(
    `Der Dienst läuft neu unter ${next}, ist von diesem Browser aus aber nicht erreichbar. ` +
      "Im DevContainer muss der neue Port weitergeleitet sein; sonst im Terminal k-playbook aufrufen."
  );
}

// Drei Zustände kommen an: etwas zu tun (älteres Programm oder Update des
// Clones), geprüft und gleich, und nicht prüfbar (data.message). Sichtbar ist
// der Knopf nur im ersten; der dritte geht als Meldung in die Statuskarte,
// siehe showUpdateMessage.
//
// Ein älteres Programm hat Vorrang vor einem Update des Clones: ein Pull allein
// hilft dann nicht, der Clone ist dem Programm schon voraus. Zeigt der PATH auf
// ein anderes Programm, gibt es dafür keinen Knopf, nur den Hinweis — ein
// installiertes Programm, das der nächste Aufruf nicht startet, brächte den
// Fehler zurück.
function renderUpdate(data) {
  const program = data.program || {};
  if (program.outdated && program.installable) {
    updateMode = "program";
    serviceElements.update.className = "primary attention-highlight";
    serviceElements.update.textContent = `Programm aktualisieren (${program.running} → ${program.clone})`;
    serviceElements.update.title =
      `Installiert das zum Clone passende Programm nach ${program.target} ` +
      "über k-playbook/bin/install und startet den Dienst daraus neu.";
    serviceElements.update.disabled = false;
    return;
  }

  if (data.available) {
    updateMode = "pull";
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
  updateMode = "";
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
// einem anderen Fenster, per k-playbook stop oder weil er nach einer
// Programmaktualisierung aus einem anderen Fenster neu gestartet ist; dann
// läuft er unter neuer Adresse, die dieses Fenster nicht kennt. Er war für alle Fenster derselbe, also gilt das für alle.
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
