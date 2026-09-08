"use strict";

// Seite "Tasks": die offenen, die erledigten und der gelesene Inhalt.
//
// Gelesen wird nur. Angelegt und ausgeführt werden Tasks über die Commands,
// nicht über die Oberfläche.

// Muss vor den Ladefunktionen laufen: die blenden Blöcke ein, und das Menü
// zieht das nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();

// Das Lebenszeichen dieses Fensters: es hält den Dienst aus dem Leerlauf und
// merkt, wenn er weg ist.
startSession((message) => {
  document.getElementById("tasks-message").textContent = message;
});

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
        ? "Ein Task ist noch nicht durch /k-task-refine gegangen."
        : `${unreviewed} Tasks sind noch nicht durch /k-task-refine gegangen.`;
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
        : "ohne Task-Refine";
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
