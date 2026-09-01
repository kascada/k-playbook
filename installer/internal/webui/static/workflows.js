"use strict";

// Seite "Workflows": Reviews, Tasks und Todos untereinander.
//
// Gelesen wird nur. Angelegt und ausgeführt wird jede der drei Sorten über die
// Commands, nicht über die Oberfläche.
//
// Drei Herkünfte in einer Datei: die drei bisherigen Seitenskripte trugen je
// ein eigenes `elements`, dazu gleichnamige Funktionen und Zustände. Jede
// Herkunft steht deshalb in einer eigenen Kapsel und behält ihre Namen bei
// sich. Geteilt wird nur, was tatsächlich zur Seite gehört und nicht zu einer
// ihrer Listen: das Menü und die Sitzung.

// Muss vor den Ladefunktionen laufen: die blenden Blöcke ein, und das Menü
// zieht das nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();

// Ohne Lebenszeichen von dieser Seite beendet sich der Server wenige Sekunden
// nach dem Wechsel hierher — der Weg zurück führte dann ins Leere. Der Aufruf
// steht genau einmal: drei wären drei Intervalle und drei Klick-Listener, von
// denen nur der zuletzt gesetzte Handler zählt.
startSession(showServerLost);

// Der Server ist für alle drei Listen derselbe. Fällt er aus, ist jede von
// ihnen ein alter Stand — die Meldung steht deshalb an jeder, egal wo auf der
// Seite gerade gelesen wird.
function showServerLost(message) {
  for (const id of ["runs-message", "tasks-message", "todos-message"]) {
    document.getElementById(id).textContent = message;
  }
}

// Der Merkspeicher ist eine Bequemlichkeit, kein Zustand der Anwendung: ist er
// gesperrt, arbeitet die Seite ohne ihn weiter. Beide Aufklapp-Blöcke nutzen
// ihn, deshalb steht er außerhalb der Kapseln.
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

// Ein Block, der erst beim Aufklappen lädt. Erledigte sammeln sich an und
// werden nie weniger; für ihre Liste wird jede Datei einmal gelesen.
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

// Bisherige Läufe --------------------------------------------------------

(function reviewsBlock() {
  const elements = {
    runsPill: document.getElementById("runs-pill"),
    runsList: document.getElementById("runs-list"),
    runsMessage: document.getElementById("runs-message"),
  };

  load();

  async function load() {
    try {
      const response = await fetch("/api/reviews", { cache: "no-store" });
      render(await response.json());
    } catch {
      elements.runsMessage.textContent = "Läufe konnten nicht geladen werden.";
    }
  }

  function render(data) {
    elements.runsList.replaceChildren();
    elements.runsMessage.textContent = data.message || "";

    if (!data.available) {
      elements.runsPill.className = "pill muted";
      elements.runsPill.textContent = "Unbekannt";
      elements.runsMessage.textContent = "Keine Projektkonfiguration gefunden.";
      return;
    }

    if (data.message) {
      elements.runsPill.className = "pill warn";
      elements.runsPill.textContent = "Nicht lesbar";
      return;
    }

    const runs = data.runs || [];
    elements.runsPill.className = runs.length ? "pill ok" : "pill muted";
    elements.runsPill.textContent = runs.length ? `${runs.length}` : "keine";

    if (runs.length === 0) {
      elements.runsMessage.textContent = "Noch kein Lauf angelegt.";
      return;
    }

    for (const run of runs) {
      const row = document.createElement("div");
      const term = document.createElement("dt");
      term.textContent = run.name;
      const detail = document.createElement("dd");
      // Ein Verzeichnis ohne run.json stammt aus der Zeit vor diesem Modell. Das
      // gehört gesagt, statt es als leeren Lauf auszugeben.
      detail.textContent = run.hasRunFile
        ? `${run.state} — ${run.entryCount} Einträge`
        : "ohne run.json";
      row.append(term, detail);
      elements.runsList.append(row);
    }
  }
})();

// Tasks ------------------------------------------------------------------

(function tasksBlock() {
  const elements = {
    tasksPill: document.getElementById("tasks-pill"),
    tasksList: document.getElementById("tasks-list"),
    tasksMessage: document.getElementById("tasks-message"),
    doneCard: document.getElementById("tasks-done-card"),
    donePill: document.getElementById("tasks-done-pill"),
    doneList: document.getElementById("tasks-done-list"),
    doneMessage: document.getElementById("tasks-done-message"),
    taskCard: document.getElementById("task-card"),
    taskPath: document.getElementById("task-path"),
    taskTitle: document.getElementById("task-title"),
    taskViewer: document.getElementById("task-viewer"),
  };

  // Der zuletzt angeklickte Task. Zwei rasche Klicks können in umgekehrter
  // Reihenfolge antworten; nur die Antwort zum offenen Task gehört ins Fenster.
  let currentTask = "";

  // Die erledigten werden einmal je Seitenaufruf geholt, beim ersten Aufklappen.
  // Sie ändern sich nur durch einen /k-task-run, und der läuft nicht im Browser.
  let doneRequested = false;

  // Ob der Block aufgeklappt war, überlebt den Seitenwechsel. Wer die Erledigten
  // sucht, sucht sie meist mehrmals hintereinander.
  const doneOpenKey = "k-playbook.tasks.done-open";

  load();
  setUpDoneCard(elements.doneCard, doneOpenKey, loadDone);

  async function load() {
    try {
      const response = await fetch("/api/tasks", { cache: "no-store" });
      render(await response.json());
    } catch {
      elements.tasksMessage.textContent = "Tasks konnten nicht geladen werden.";
    }
  }

  function render(data) {
    elements.tasksList.replaceChildren();
    elements.tasksMessage.textContent = data.message || "";

    if (!data.available) {
      elements.tasksList.classList.add("empty");
      elements.tasksList.textContent = "Keine Projektkonfiguration gefunden.";
      elements.tasksPill.className = "pill muted";
      elements.tasksPill.textContent = "Unbekannt";
      return;
    }

    if (data.message) {
      elements.tasksList.classList.add("empty");
      elements.tasksPill.className = "pill warn";
      elements.tasksPill.textContent = "Nicht lesbar";
      return;
    }

    const tasks = data.tasks || [];
    elements.tasksList.classList.toggle("empty", tasks.length === 0);
    if (tasks.length === 0) {
      elements.tasksList.textContent = "Keine offenen Tasks.";
      elements.tasksPill.className = "pill ok";
      elements.tasksPill.textContent = "keine";
      return;
    }

    fillList(elements.tasksList, tasks, true);

    elements.tasksPill.className = "pill ok";
    elements.tasksPill.textContent = tasks.length === 1 ? "1 offen" : `${tasks.length} offen`;

    const unreviewed = tasks.filter((task) => !task.reviewed).length;
    if (unreviewed > 0) {
      elements.tasksMessage.textContent =
        unreviewed === 1
          ? "Ein Task ist noch nicht durch /k-review-loop gegangen."
          : `${unreviewed} Tasks sind noch nicht durch /k-review-loop gegangen.`;
    }
  }

  // fillList baut die Zeilen einer Liste. Der Review-Stand gehört nur zu den
  // offenen: nach der Ausführung sagt er nichts mehr aus.
  function fillList(container, tasks, withReview) {
    for (const task of tasks) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = "task-link";
      button.dataset.path = task.path;

      // Der Dateiname trägt die Nummer und ist die Ordnung der Liste; der Titel
      // sagt, worum es geht. Beides gehört auf den Knopf.
      const name = document.createElement("span");
      name.className = "task-name";
      name.textContent = task.path.replace(/^done\//, "");
      const title = document.createElement("span");
      title.className = "task-title";
      title.textContent = task.title || "";
      button.append(name, title);

      // Ein Task ohne Review-Log ist nie gegengelesen worden. /k-task-run fragt dann
      // vor der Ausführung nach — das soll man vorher sehen können.
      if (withReview) {
        const review = document.createElement("span");
        review.className = task.reviewed ? "task-review" : "task-review open";
        review.textContent = task.reviewed
          ? task.reviewedAt
            ? `gereviewt ${task.reviewedAt}`
            : "gereviewt"
          : "ohne Review-Loop";
        button.append(review);
      }

      button.addEventListener("click", () => openTask(task.path, task.title));
      container.append(button);
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
      const response = await fetch("/api/tasks/done", { cache: "no-store" });
      renderDone(await response.json());
    } catch {
      // Beim nächsten Aufklappen darf es wieder versucht werden.
      doneRequested = false;
      elements.donePill.className = "pill warn";
      elements.donePill.textContent = "Fehler";
      elements.doneMessage.textContent = "Erledigte Tasks konnten nicht geladen werden.";
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

    const tasks = data.tasks || [];
    elements.doneList.classList.toggle("empty", tasks.length === 0);
    if (tasks.length === 0) {
      elements.doneList.textContent = "Noch nichts erledigt.";
      elements.donePill.className = "pill muted";
      elements.donePill.textContent = "keine";
      return;
    }

    fillList(elements.doneList, tasks, false);
    elements.donePill.className = "pill muted";
    elements.donePill.textContent = tasks.length === 1 ? "1 erledigt" : `${tasks.length} erledigt`;

    // Die eben gebaute Liste kennt den offenen Task noch nicht.
    setActiveTask(currentTask);
  }

  async function openTask(path, title) {
    currentTask = path;
    setActiveTask(path);
    elements.taskCard.classList.remove("hidden");
    elements.taskPath.textContent = path;
    elements.taskTitle.textContent = title || path;
    elements.taskViewer.classList.add("empty");
    elements.taskViewer.textContent = "Wird geladen...";

    try {
      const response = await fetch(`/api/tasks/file?path=${encodeURIComponent(path)}`, { cache: "no-store" });
      const data = await response.json();
      if (currentTask !== path) {
        return;
      }
      if (data.message) {
        elements.taskViewer.textContent = data.message;
        return;
      }
      elements.taskTitle.textContent = data.title || path;
      elements.taskViewer.classList.remove("empty");
      // Der Inhalt kommt gerendert aus dem Backend; rohes HTML aus der Datei ist
      // dort abgeschaltet.
      elements.taskViewer.innerHTML = data.html;
    } catch {
      if (currentTask === path) {
        elements.taskViewer.textContent = "Task konnte nicht geladen werden.";
      }
    }
  }

  function setActiveTask(path) {
    for (const button of document.querySelectorAll(".task-link")) {
      button.classList.toggle("active", button.dataset.path === path);
    }
  }
})();

// Todos ------------------------------------------------------------------

(function todosBlock() {
  const elements = {
    todosPill: document.getElementById("todos-pill"),
    todosList: document.getElementById("todos-list"),
    todosMessage: document.getElementById("todos-message"),
    doneCard: document.getElementById("todos-done-card"),
    donePill: document.getElementById("todos-done-pill"),
    doneList: document.getElementById("todos-done-list"),
    doneMessage: document.getElementById("todos-done-message"),
  };

  // Ab dieser Zeichenzahl wird ein Eintrag gekürzt angezeigt — /k-todo hängt
  // oft einen einzigen, langen Fließtextsatz an. Rund zwei Zeilen.
  const clampLength = 150;

  // Die Erledigten werden einmal je Seitenaufruf geholt, beim ersten Aufklappen.
  let doneRequested = false;

  // Ob der Block aufgeklappt war, überlebt den Seitenwechsel.
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
    // unproblematisch, läse sich neben der gleichnamigen Funktion der Kapsel
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
})();
