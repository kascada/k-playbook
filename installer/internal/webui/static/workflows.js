"use strict";

// Die Zahlen der drei Workflow-Karten: wie viel in jeder Sorte liegt. Die
// Karten selbst stehen im Fragment workflow-cards.html, das /workflows und die
// Statusseite einbinden; beide rufen loadWorkflowCounts() auf.
//
// Geholt wird hier nur die Zahl. Sie stammt aus derselben Antwort, aus der die
// jeweilige Seite ihre Liste baut — einen Aggregat-Endpunkt gibt es nicht, er
// wäre die Doppelung dieser drei Zahlen.
//
// Die Datei tut beim Laden nichts: Blockmenü und Lebenszeichen gehören der
// Seite. Zweimal gerufen, entstünde das Menü doppelt und es liefen zwei
// Lebenszeichen. Klassisches Skript, kein Modul — jeder Name auf oberster
// Ebene teilt sich den Namensraum mit den übrigen Skripten der Seite.

// Die drei Vorräte in der Reihenfolge der Karten. Jeder nennt seinen
// Endpunkt, das Feld mit den Einträgen und die Beschriftung seiner Zahl:
// Läufe sammeln sich an, Tasks und Todos sind offen oder nicht.
const stocks = [
  { pill: "tasks-pill", url: "/api/tasks", field: "tasks", label: (count) => (count === 1 ? "1 offen" : `${count} offen`) },
  { pill: "reviews-pill", url: "/api/reviews", field: "runs", label: (count) => (count === 1 ? "1 Lauf" : `${count} Läufe`) },
  { pill: "todos-pill", url: "/api/todos", field: "todos", label: (count) => (count === 1 ? "1 offen" : `${count} offen`) },
];

// Füllt die drei Zahlen. message ist das Element, in das Fehler und Hinweise
// gehen; es steht auf der Seite und nicht im Fragment der Karten.
function loadWorkflowCounts(message) {
  for (const stock of stocks) {
    loadCount(stock, message);
  }
}

async function loadCount(stock, message) {
  const pill = document.getElementById(stock.pill);
  try {
    const response = await fetch(stock.url, { cache: "no-store" });
    renderCount(pill, stock, await response.json(), message);
  } catch {
    pill.className = "pill warn";
    pill.textContent = "Fehler";
    message.textContent = "Der Stand konnte nicht vollständig geladen werden.";
  }
}

function renderCount(pill, stock, data, message) {
  if (!data.available) {
    pill.className = "pill muted";
    pill.textContent = "Unbekannt";
    message.textContent = "Keine Projektkonfiguration gefunden.";
    return;
  }

  // Eine Meldung in der Antwort heißt: gelesen wurde nicht. Die Zahl daneben
  // wäre dann keine Auskunft, sondern eine falsche.
  if (data.message) {
    pill.className = "pill warn";
    pill.textContent = "Nicht lesbar";
    message.textContent = data.message;
    return;
  }

  // Ein hint ist das Gegenteil: gelesen wurde, es gibt nur etwas dazu zu sagen.
  // Er darf die Zahl deshalb nicht auf "Nicht lesbar" kippen.
  if (data.hint) {
    message.textContent = data.hint;
  }

  const count = (data[stock.field] || []).length;
  pill.className = count ? "pill ok" : "pill muted";
  pill.textContent = count ? stock.label(count) : "keine";
}
