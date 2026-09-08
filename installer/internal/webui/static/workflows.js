"use strict";

// Seite "Workflows": die Übersicht des Bereichs. Sie sagt, was die drei Sorten
// sind, wie viel in jeder liegt und wo sie stehen; die Listen selbst haben
// eigene Seiten.
//
// Geholt wird hier nur die Zahl. Sie stammt aus derselben Antwort, aus der die
// jeweilige Seite ihre Liste baut — einen Aggregat-Endpunkt gibt es nicht, er
// wäre die Doppelung dieser drei Zahlen.

// Muss vor den Ladefunktionen laufen: die blenden Blöcke ein, und das Menü
// zieht das nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();

const message = document.getElementById("workflows-message");

startSession((lost) => {
  message.textContent = lost;
});

// Die drei Vorräte in der Reihenfolge der Karten. Jeder nennt seinen
// Endpunkt, das Feld mit den Einträgen und die Beschriftung seiner Zahl:
// Läufe sammeln sich an, Tasks und Todos sind offen oder nicht.
const stocks = [
  { pill: "tasks-pill", url: "/api/tasks", field: "tasks", label: (count) => (count === 1 ? "1 offen" : `${count} offen`) },
  { pill: "reviews-pill", url: "/api/reviews", field: "runs", label: (count) => (count === 1 ? "1 Lauf" : `${count} Läufe`) },
  { pill: "todos-pill", url: "/api/todos", field: "todos", label: (count) => (count === 1 ? "1 offen" : `${count} offen`) },
];

for (const stock of stocks) {
  loadCount(stock);
}

async function loadCount(stock) {
  const pill = document.getElementById(stock.pill);
  try {
    const response = await fetch(stock.url, { cache: "no-store" });
    render(pill, stock, await response.json());
  } catch {
    pill.className = "pill warn";
    pill.textContent = "Fehler";
    message.textContent = "Der Stand konnte nicht vollständig geladen werden.";
  }
}

function render(pill, stock, data) {
  if (!data.available) {
    pill.className = "pill muted";
    pill.textContent = "Unbekannt";
    message.textContent = "Keine Projektkonfiguration gefunden.";
    return;
  }

  // Ein Hinweis in der Antwort heißt: gelesen wurde nicht. Die Zahl daneben
  // wäre dann keine Auskunft, sondern eine falsche.
  if (data.message) {
    pill.className = "pill warn";
    pill.textContent = "Nicht lesbar";
    message.textContent = data.message;
    return;
  }

  const count = (data[stock.field] || []).length;
  pill.className = count ? "pill ok" : "pill muted";
  pill.textContent = count ? stock.label(count) : "keine";
}
