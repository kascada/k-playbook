// Seite "Todos": die offenen Punkte aus TODO.md untereinander, darunter die
// abgehakten.
//
// Gelesen wird nur. Einträge entstehen über /k-todo, nicht über die
// Oberfläche.

const elements = {
  todosPill: document.getElementById("todos-pill"),
  todosList: document.getElementById("todos-list"),
  todosMessage: document.getElementById("todos-message"),
  doneCard: document.getElementById("done-card"),
  donePill: document.getElementById("done-pill"),
  doneList: document.getElementById("done-list"),
  doneMessage: document.getElementById("done-message"),
};

// Ab dieser Zeichenzahl wird ein Eintrag gekürzt angezeigt — /k-todo hängt
// oft einen einzigen, langen Fließtextsatz an. Rund zwei Zeilen.
const clampLength = 150;

// Die Erledigten werden einmal je Seitenaufruf geholt, beim ersten Aufklappen.
let doneRequested = false;

// Ob der Block aufgeklappt war, überlebt den Seitenwechsel.
const doneOpenKey = "k-playbook.todos.done-open";

// Ohne Lebenszeichen von dieser Seite beendet sich der Server wenige Sekunden
// nach dem Wechsel hierher.
startSession((message) => {
  elements.todosMessage.textContent = message;
});
load();
setUpDone();

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

// fillList baut die Zeilen einer Liste. Ein langer Eintrag steht zunächst
// gekürzt da — die Liste soll auf einen Blick überschaubar bleiben, auch wenn
// ein einzelner Punkt ein langer Fließtext ist.
function fillList(container, todos, done) {
  for (const todo of todos) {
    const item = document.createElement("div");
    item.className = done ? "todo-item done" : "todo-item";
    item.append(buildText(todo.text));
    container.append(item);
  }
}

// buildText baut den Absatz eines Eintrags. Der Umschalter hängt direkt am
// Ende des (gekürzten) Textes — keine eigene Spalte, nur ein paar Wörter mehr
// in derselben Zeile.
function buildText(fullText) {
  const paragraph = document.createElement("p");
  paragraph.className = "todo-text";

  if (fullText.length <= clampLength) {
    paragraph.textContent = fullText;
    return paragraph;
  }

  let expanded = false;
  const content = document.createElement("span");
  const toggle = document.createElement("button");
  toggle.type = "button";
  toggle.className = "todo-toggle";

  // Eigener Name, nicht "render": das ist innerhalb dieser Funktion zwar
  // unproblematisch, läse sich neben der gleichnamigen Top-Level-Funktion
  // aber leicht falsch.
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

function setUpDone() {
  elements.doneCard.addEventListener("toggle", () => {
    remember(doneOpenKey, elements.doneCard.open);
    if (elements.doneCard.open) {
      loadDone();
    }
  });

  // Ein wiederhergestelltes "offen" löst das Ereignis oben nicht verlässlich
  // aus, deshalb wird hier selbst geladen. loadDone() läuft trotzdem nur
  // einmal.
  if (recall(doneOpenKey)) {
    elements.doneCard.open = true;
    loadDone();
  }
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
