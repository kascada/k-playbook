"use strict";

// Seite "Todos": die offenen und die abgehakten Punkte aus der
// Kephalaion-Collection des Projekts (todos/ und todos/done/), dazu Anlegen,
// Abhaken, Wiederöffnen und eine Suche.
//
// Geschrieben wird über POST /api/todos und /api/todos/{id}/done|reopen —
// dieselbe Fachlogik wie /k-todo und `k-playbook todo`. Ohne erreichbares
// Kephalaion gibt es keine Todos; die Seite sagt das, statt eine leere Liste
// zu zeigen.

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
  addForm: document.getElementById("todo-add-form"),
  addText: document.getElementById("todo-add-text"),
  addButton: document.getElementById("todo-add-button"),
  addMessage: document.getElementById("todo-add-message"),
  searchForm: document.getElementById("todo-search-form"),
  searchText: document.getElementById("todo-search-text"),
  searchPill: document.getElementById("todos-search-pill"),
  searchList: document.getElementById("todos-search-list"),
  searchMessage: document.getElementById("todos-search-message"),
};

// Codes der Fachlogik, die einen Zustand benennen statt eines Lesefehlers.
const codeUnavailable = "kephalaion_unavailable";
const codeNotMigrated = "todos_not_migrated";

// Ab dieser Zeichenzahl wird ein Eintrag gekürzt angezeigt — über /k-todo
// kommt oft ein einziger, langer Fließtextsatz hinzu. Rund zwei Zeilen.
const clampLength = 150;

// Die Erledigten werden einmal je Seitenaufruf geholt, beim ersten Aufklappen.
let doneRequested = false;

const doneOpenKey = "k-playbook.todos.done-open";

// Die zuletzt gesuchte Zeichenkette; nach einer Änderung wird die Suche
// wiederholt, damit Treffer und Knöpfe stimmen.
let lastSearch = "";

load();
setUpDoneCard(elements.doneCard, doneOpenKey, loadDone);
elements.addForm.addEventListener("submit", addTodo);
elements.searchForm.addEventListener("submit", (event) => {
  event.preventDefault();
  search(elements.searchText.value.trim());
});

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
  const usable = Boolean(data.available) && data.code !== codeUnavailable;
  elements.addText.disabled = !usable;
  elements.addButton.disabled = !usable;

  if (!data.available) {
    elements.todosList.classList.add("empty");
    elements.todosList.textContent = "Keine Projektkonfiguration gefunden.";
    elements.todosPill.className = "pill muted";
    elements.todosPill.textContent = "Unbekannt";
    return;
  }

  if (data.code === codeUnavailable) {
    elements.todosList.classList.add("empty");
    elements.todosList.textContent = "Ohne erreichbares Kephalaion gibt es keine Todos.";
    elements.todosPill.className = "pill warn";
    elements.todosPill.textContent = "Kephalaion nicht erreichbar";
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

  fillList(elements.todosList, todos, elements.todosMessage);
  elements.todosPill.className = "pill ok";
  elements.todosPill.textContent = todos.length === 1 ? "1 offen" : `${todos.length} offen`;
}

// showHint zeigt einen Hinweis über der Liste — etwa ein noch nicht
// übernommenes data/todos.json oder eine zurückgebliebene TODO.md. Er ist
// ausdrücklich kein Fehler: Liste und Zählpille bleiben stehen.
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
// dahinter klein sein Datum; rechts der Knopf zum Abhaken bzw. Wiederöffnen.
// Ob ein Eintrag erledigt ist, sagt sein Feld done — in der Suche stehen
// beide Sorten in einer Liste.
function fillList(container, todos, messageElement) {
  for (const todo of todos) {
    const done = Boolean(todo.done);
    const item = document.createElement("div");
    item.className = done ? "todo-item with-action done" : "todo-item with-action";
    item.append(buildText(todo.text, todo), buildAction(todo, done, messageElement));
    container.append(item);
  }
}

// buildAction baut den Knopf eines Eintrags.
function buildAction(todo, done, messageElement) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "secondary todo-action";
  button.textContent = done ? "Wieder öffnen" : "Abhaken";
  button.title = done ? `Todo #${todo.id} wieder öffnen` : `Todo #${todo.id} abhaken`;
  button.addEventListener("click", async () => {
    button.disabled = true;
    const data = await postTodo(`/api/todos/${todo.id}/${done ? "reopen" : "done"}`, null);
    if (!data.ok) {
      button.disabled = false;
      messageElement.textContent = describeFailure(data);
      return;
    }
    messageElement.textContent = "";
    refreshAll();
  });
  return button;
}

// postTodo schickt einen schreibenden Aufruf und liefert die Antwort; ein
// Netzfehler wird eine Antwort ohne ok.
async function postTodo(url, body) {
  try {
    const response = await fetch(url, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: body === null ? "{}" : JSON.stringify(body),
    });
    return await response.json();
  } catch {
    return { ok: false, message: "Der Aufruf hat die Oberfläche nicht erreicht." };
  }
}

// describeFailure macht aus einer abgelehnten Änderung einen Satz.
function describeFailure(data) {
  if (data.code === codeNotMigrated) {
    return "Noch nicht übernommen: Erst k-playbook todo migrate übernimmt data/todos.json in die Collection; bis dahin wird nichts angelegt oder geändert.";
  }
  if (data.code === codeUnavailable) {
    return "Kephalaion ist nicht erreichbar; nichts wurde geändert. " + (data.message || "");
  }
  return data.message || "Die Änderung ist fehlgeschlagen.";
}

// refreshAll lädt nach einer Änderung neu, was die Seite zeigt.
function refreshAll() {
  load();
  if (doneRequested) {
    doneRequested = false;
    loadDone();
  }
  if (lastSearch) {
    search(lastSearch);
  }
}

async function addTodo(event) {
  event.preventDefault();
  const text = elements.addText.value.trim();
  if (!text) {
    elements.addMessage.textContent = "Ein Todo braucht einen Text.";
    return;
  }
  elements.addButton.disabled = true;
  const data = await postTodo("/api/todos", { text });
  elements.addButton.disabled = false;
  if (!data.ok) {
    elements.addMessage.textContent = describeFailure(data);
    return;
  }
  elements.addText.value = "";
  elements.addMessage.textContent = `Angelegt als #${data.todo.id}.`;
  refreshAll();
}

async function search(query) {
  lastSearch = query;
  elements.searchList.replaceChildren();
  elements.searchMessage.textContent = "";
  if (!query) {
    elements.searchList.hidden = true;
    elements.searchPill.className = "pill muted";
    elements.searchPill.textContent = "bereit";
    return;
  }
  elements.searchPill.className = "pill muted";
  elements.searchPill.textContent = "Suchen...";
  let data;
  try {
    const response = await fetch(`/api/todos/search?q=${encodeURIComponent(query)}`, { cache: "no-store" });
    data = await response.json();
  } catch {
    elements.searchPill.className = "pill warn";
    elements.searchPill.textContent = "Fehler";
    elements.searchMessage.textContent = "Die Suche hat die Oberfläche nicht erreicht.";
    return;
  }
  if (query !== lastSearch) {
    return;
  }
  elements.searchList.hidden = false;
  if (data.code === codeUnavailable || data.message) {
    elements.searchList.classList.add("empty");
    elements.searchList.textContent = data.code === codeUnavailable
      ? "Ohne erreichbares Kephalaion gibt es keine Todos."
      : "Die Todos konnten nicht gelesen werden.";
    elements.searchPill.className = "pill warn";
    elements.searchPill.textContent = data.code === codeUnavailable ? "Kephalaion nicht erreichbar" : "Nicht lesbar";
    elements.searchMessage.textContent = data.message || "";
    return;
  }
  const todos = data.todos || [];
  elements.searchList.classList.toggle("empty", todos.length === 0);
  if (todos.length === 0) {
    elements.searchList.textContent = "Kein Todo enthält diesen Text.";
    elements.searchPill.className = "pill muted";
    elements.searchPill.textContent = "keine Treffer";
    return;
  }
  fillList(elements.searchList, todos, elements.searchMessage);
  elements.searchPill.className = "pill ok";
  elements.searchPill.textContent = todos.length === 1 ? "1 Treffer" : `${todos.length} Treffer`;
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
      date.title = "Datum aus einer früheren Übernahme, kein Abhak-Datum.";
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

  if (data.code === codeUnavailable) {
    elements.doneList.classList.add("empty");
    elements.doneList.textContent = "Ohne erreichbares Kephalaion gibt es keine Todos.";
    elements.donePill.className = "pill warn";
    elements.donePill.textContent = "Kephalaion nicht erreichbar";
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

  fillList(elements.doneList, todos, elements.doneMessage);
  elements.donePill.className = "pill muted";
  elements.donePill.textContent = todos.length === 1 ? "1 erledigt" : `${todos.length} erledigt`;
}
