"use strict";

// Seite "Todos": die offenen und die abgehakten Punkte aus
// k-playbook-local/data/todos.json.
//
// Gelesen wird hier nur. Geschrieben wird die Ablage über /k-todo, über das
// Subkommando `k-playbook todo` oder über die Oberfläche — nie von Hand.

// Muss vor den Ladefunktionen laufen: die blenden Blöcke ein, und das Menü
// zieht das nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();

startSession((message) => {
  document.getElementById("todos-message").textContent = message;
});

const elements = {
  todosPill: document.getElementById("todos-pill"),
  todosList: document.getElementById("todos-list"),
  todosMessage: document.getElementById("todos-message"),
  todosHint: document.getElementById("todos-hint"),
  doneCard: document.getElementById("todos-done-card"),
  donePill: document.getElementById("todos-done-pill"),
  doneList: document.getElementById("todos-done-list"),
  doneMessage: document.getElementById("todos-done-message"),
  doneHint: document.getElementById("todos-done-hint"),
};

// Ab dieser Zeichenzahl wird ein Eintrag gekürzt angezeigt — über /k-todo
// kommt oft ein einziger, langer Fließtextsatz hinzu. Rund zwei Zeilen.
const clampLength = 150;

// Die Erledigten werden einmal je Seitenaufruf geholt, beim ersten Aufklappen.
let doneRequested = false;

const doneOpenKey = "k-playbook.todos.done-open";

load();
setUpDoneCard(elements.doneCard, doneOpenKey, loadDone);

async function load() {
  try {
    const response = await fetch("/api/todos", { cache: "no-store" });
    render(await response.json());
  } catch {
    elements.todosMessage.textContent = "Todos konnten nicht geladen werden.";
  }
}

function render(data) {
  elements.todosList.replaceChildren();
  elements.todosMessage.textContent = data.message || "";
  showHint(elements.todosHint, data.hint);

  if (!data.available) {
    elements.todosList.classList.add("empty");
    elements.todosList.textContent = "Keine Projektkonfiguration gefunden.";
    elements.todosPill.className = "pill muted";
    elements.todosPill.textContent = "Unbekannt";
    return;
  }

  if (data.message) {
    elements.todosList.classList.add("empty");
    elements.todosPill.className = "pill warn";
    elements.todosPill.textContent = "Nicht lesbar";
    return;
  }

  const todos = data.todos || [];
  elements.todosList.classList.toggle("empty", todos.length === 0);
  if (todos.length === 0) {
    elements.todosList.textContent = "Keine offenen Todos.";
    elements.todosPill.className = "pill ok";
    elements.todosPill.textContent = "keine";
    return;
  }

  fillList(elements.todosList, todos);
  elements.todosPill.className = "pill ok";
  elements.todosPill.textContent = todos.length === 1 ? "1 offen" : `${todos.length} offen`;
}

// showHint zeigt einen Hinweis über der Liste — etwa eine zurückgebliebene
// Datei der früheren Markdown-Ablage. Er ist ausdrücklich kein Fehler: Liste
// und Zählpille bleiben stehen.
// Der Fehlerfall ist data.message, und nur der blendet die Liste aus.
function showHint(element, hint) {
  if (!element) {
    return;
  }
  element.textContent = hint || "";
  element.hidden = !hint;
}

// fillList baut die Zeilen einer Liste. Ein langer Eintrag steht zunächst
// gekürzt da — die Liste soll auf einen Blick überschaubar bleiben, auch wenn
// ein einzelner Punkt ein langer Fließtext ist.
//
// Vor dem Text steht die Kennung, unter der /k-todo den Eintrag anspricht,
// dahinter klein sein Datum.
function fillList(container, todos, done) {
  for (const todo of todos) {
    const item = document.createElement("div");
    item.className = done ? "todo-item done" : "todo-item";
    item.append(buildText(todo.text, todo));
    container.append(item);
  }
}

// buildMeta baut die Kennung und das Datum eines Eintrags.
//
// Gezeigt wird das Datum, das den Eintrag beschreibt: bei einem offenen sein
// created, bei einem abgehakten sein done. Ein done aus der Migration ist kein
// Abhak-Tag — das sagt der Titel, statt ein Datum zu behaupten, das es nie gab.
function buildMeta(todo) {
  const meta = document.createElement("span");
  meta.className = "todo-meta";
  meta.textContent = `#${todo.id}`;

  const stamp = todo.done || todo.created || "";
  if (stamp) {
    const date = document.createElement("span");
    date.className = "todo-date";
    date.textContent = stamp;
    if (todo.done && todo.doneMigrated) {
      date.title = "Datum der Übersetzung aus der früheren Markdown-Ablage, kein Abhak-Datum.";
    } else if (todo.done) {
      date.title = "Abgehakt am";
    } else {
      date.title = "Angelegt am";
    }
    meta.append(" ", date);
  }
  return meta;
}

// buildText baut den Absatz eines Eintrags. Der Umschalter hängt direkt am
// Ende des (gekürzten) Textes — keine eigene Spalte, nur ein paar Wörter mehr
// in derselben Zeile.
function buildText(fullText, todo) {
  const paragraph = document.createElement("p");
  paragraph.className = "todo-text";
  paragraph.append(buildMeta(todo), " ");

  if (fullText.length <= clampLength) {
    paragraph.append(fullText);
    return paragraph;
  }

  let expanded = false;
  const content = document.createElement("span");
  const toggle = document.createElement("button");
  toggle.type = "button";
  toggle.className = "todo-toggle";

  // Eigener Name, nicht "render": das ist innerhalb dieser Funktion zwar
  // unproblematisch, läse sich neben der gleichnamigen Funktion der Seite aber
  // leicht falsch.
  const renderText = () => {
    content.textContent = expanded ? fullText : `${fullText.slice(0, clampLength).trimEnd()}… `;
    toggle.textContent = expanded ? "Weniger anzeigen" : "Vollständig anzeigen";
  };
  toggle.addEventListener("click", () => {
    expanded = !expanded;
    renderText();
  });

  renderText();
  paragraph.append(content, toggle);
  return paragraph;
}

async function loadDone() {
  if (doneRequested) {
    return;
  }
  doneRequested = true;

  elements.donePill.className = "pill muted";
  elements.donePill.textContent = "Laden...";

  try {
    const response = await fetch("/api/todos/done", { cache: "no-store" });
    renderDone(await response.json());
  } catch {
    // Beim nächsten Aufklappen darf es wieder versucht werden.
    doneRequested = false;
    elements.donePill.className = "pill warn";
    elements.donePill.textContent = "Fehler";
    elements.doneMessage.textContent = "Erledigte Todos konnten nicht geladen werden.";
  }
}

function renderDone(data) {
  elements.doneList.replaceChildren();
  elements.doneMessage.textContent = data.message || "";
  showHint(elements.doneHint, data.hint);

  if (!data.available) {
    elements.doneList.classList.add("empty");
    elements.doneList.textContent = "Keine Projektkonfiguration gefunden.";
    elements.donePill.className = "pill muted";
    elements.donePill.textContent = "Unbekannt";
    return;
  }

  if (data.message) {
    elements.doneList.classList.add("empty");
    elements.donePill.className = "pill warn";
    elements.donePill.textContent = "Nicht lesbar";
    return;
  }

  const todos = data.todos || [];
  elements.doneList.classList.toggle("empty", todos.length === 0);
  if (todos.length === 0) {
    elements.doneList.textContent = "Noch nichts abgehakt.";
    elements.donePill.className = "pill muted";
    elements.donePill.textContent = "keine";
    return;
  }

  fillList(elements.doneList, todos, true);
  elements.donePill.className = "pill muted";
  elements.donePill.textContent = todos.length === 1 ? "1 erledigt" : `${todos.length} erledigt`;
}
