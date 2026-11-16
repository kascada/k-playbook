"use strict";

// Der zugeklappte Block der abgehakten Todos. Sie werden nie weniger, und
// jedes Dokument wird einmal gelesen — geholt wird deshalb erst beim
// Aufklappen.
//
// Steht in einer eigenen Datei, damit eine weitere Seite mit einem solchen
// Block dasselbe Verhalten einbinden kann: jede Seite lädt nur ihr eigenes
// Skript.

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
