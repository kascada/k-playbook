"use strict";

// Das Blockmenü der linken Spalte, für jede Seite dasselbe. Der Umschalter
// darüber ist server-gerendert — welche Bereiche es gibt, hängt an der
// Installation, und welcher aktiv ist, weiß nur die Seite selbst.
//
// Das Menü holt sich sein Element hier selbst: diese Datei gehört keiner Seite
// und darf deshalb nicht in ein fremdes `elements` greifen.

// Baut das Menü aus den Blöcken selbst: Reihenfolge, Beschriftung und Status
// stehen bereits in den Karten, ein neuer Block braucht also nichts weiter als
// eine ID und eine Überschrift.
//
// Muss vor den Ladefunktionen einer Seite laufen: die blenden Blöcke ein, und
// das Menü zieht das nur mit, wenn es die Karten schon beobachtet.
function buildBlockNav() {
  const blockNav = blockNavElement();
  if (!blockNav) {
    return;
  }

  document.querySelectorAll(".blocks > .card").forEach((card) => {
    const heading = card.querySelector("h2");
    if (!card.id || !heading) {
      return;
    }

    const dot = document.createElement("span");
    dot.className = "block-nav-dot";
    const label = document.createElement("span");
    label.textContent = heading.textContent;

    const item = document.createElement("button");
    item.type = "button";
    item.className = "block-nav-item";
    item.append(dot, label);
    item.addEventListener("click", () => goToBlock(card, item));
    blockNav.append(item);

    // Ob ein Block sichtbar ist und wie es um ihn steht, entscheiden die
    // Render-Funktionen an der Karte. Beobachten ist billiger, als in jeder
    // einzelnen zusätzlich das Menü nachzuziehen.
    const pill = card.querySelector(".section-head .pill");
    syncNavItem(card, item, dot, pill);
    const sync = () => syncNavItem(card, item, dot, pill);
    new MutationObserver(sync).observe(card, { attributes: true, attributeFilter: ["class"] });
    if (pill) {
      new MutationObserver(sync).observe(pill, { attributes: true, attributeFilter: ["class"] });
    }
  });
}

// Ein verborgener Block hat auch keinen Eintrag; der Punkt übernimmt die
// Statusfarbe seiner Pill, alles außerhalb von ok/warn/error bleibt neutral.
function syncNavItem(card, item, dot, pill) {
  item.classList.toggle("hidden", card.classList.contains("hidden"));
  const state = pill && ["ok", "warn", "error"].find((name) => pill.classList.contains(name));
  dot.className = state ? `block-nav-dot ${state}` : "block-nav-dot";
}

// Markiert wird der angeklickte Eintrag — er zeigt, wohin gesprungen wurde,
// und wandert beim Scrollen von Hand nicht mit.
function goToBlock(card, item) {
  // Ein zugeklappter Block wäre nach dem Sprung nur eine Kopfzeile. Das
  // Aufklappen löst über das toggle-Ereignis zugleich das Nachladen aus.
  if (card.tagName === "DETAILS") {
    card.open = true;
  }

  card.scrollIntoView({ behavior: "smooth", block: "start" });
  markBlockNavItem(item);
}

// Markiert genau einen Eintrag des Menüs. Steht hier und nicht in goToBlock,
// weil nicht jedes Menü aus Karten entsteht: /docs füllt dieselbe Liste aus
// seinen Dateien und braucht dieselbe Markierung.
function markBlockNavItem(item) {
  const blockNav = blockNavElement();
  if (!blockNav) {
    return;
  }

  blockNav.querySelectorAll(".block-nav-item.active").forEach((other) => {
    other.classList.remove("active");
    other.removeAttribute("aria-current");
  });
  item.classList.add("active");
  item.setAttribute("aria-current", "true");
}

function blockNavElement() {
  return document.getElementById("block-nav");
}
