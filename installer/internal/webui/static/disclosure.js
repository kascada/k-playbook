"use strict";

// Der zugeklappte Block, den es auf mehreren Seiten gibt: die erledigten Tasks
// und die abgehakten Todos. Beide werden nie weniger, für beide wird jede
// Datei einmal gelesen — geholt wird deshalb erst beim Aufklappen.
//
// Steht in einer eigenen Datei, weil beide Seiten dasselbe Verhalten brauchen
// und keine der beiden es der anderen leihen kann: jede Seite lädt nur ihr
// eigenes Skript.

// Der Merkspeicher ist eine Bequemlichkeit, kein Zustand der Anwendung: ist er
// gesperrt, arbeitet die Seite ohne ihn weiter.
function remember(key, value) {
  try {
    localStorage.setItem(key, value ? "1" : "0");
  } catch {
    // Kein Speicher, keine Erinnerung.
  }
}

function recall(key) {
  try {
    return localStorage.getItem(key) === "1";
  } catch {
    return false;
  }
}

// Bindet einen Aufklapp-Block an seine Ladefunktion. Ob er offen war, überlebt
// den Seitenwechsel: wer die Erledigten sucht, sucht sie meist mehrmals
// hintereinander.
function setUpDoneCard(card, openKey, load) {
  card.addEventListener("toggle", () => {
    remember(openKey, card.open);
    if (card.open) {
      load();
    }
  });

  // Ein wiederhergestelltes "offen" löst das Ereignis oben nicht verlässlich
  // aus, deshalb wird hier selbst geladen. load() läuft trotzdem nur einmal.
  if (recall(openKey)) {
    card.open = true;
    load();
  }
}
