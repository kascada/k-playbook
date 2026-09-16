"use strict";

// Seite "Workflows": die Übersicht des Bereichs. Sie sagt, was die drei Sorten
// sind, wie viel in jeder liegt und wo sie stehen; die Listen selbst haben
// eigene Seiten. Die Zahlen holt workflows.js, das auch die Statusseite lädt.

const workflowsMessage = document.getElementById("workflows-message");

// Muss vor den Ladefunktionen laufen: die blenden Blöcke ein, und das Menü
// zieht das nur mit, wenn es die Karten schon beobachtet. Die Übersicht hat
// selbst kein Blockmenü; der Aufruf bleibt dann ohne Wirkung.
buildBlockNav();

startSession((lost) => {
  workflowsMessage.textContent = lost;
});

loadWorkflowCounts(workflowsMessage);
