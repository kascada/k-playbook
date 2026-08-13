// Seite "Tasks": die offenen Aufgaben untereinander, eine davon gelesen.
//
// Gelesen wird nur. Angelegt und ausgeführt werden Tasks über die Commands,
// nicht über die Oberfläche.

const elements = {
  tasksPill: document.getElementById("tasks-pill"),
  tasksList: document.getElementById("tasks-list"),
  tasksMessage: document.getElementById("tasks-message"),
  taskCard: document.getElementById("task-card"),
  taskPath: document.getElementById("task-path"),
  taskTitle: document.getElementById("task-title"),
  taskViewer: document.getElementById("task-viewer"),
};

// Der zuletzt angeklickte Task. Zwei rasche Klicks können in umgekehrter
// Reihenfolge antworten; nur die Antwort zum offenen Task gehört ins Fenster.
let currentTask = "";

// Ohne Lebenszeichen von dieser Seite beendet sich der Server wenige Sekunden
// nach dem Wechsel hierher — der Weg zurück führte dann ins Leere.
startSession((message) => {
  elements.tasksMessage.textContent = message;
});
load();

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

  for (const task of tasks) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "task-link";
    button.dataset.path = task.path;

    // Der Dateiname trägt die Nummer und ist die Ordnung der Liste; der Titel
    // sagt, worum es geht. Beides gehört auf den Knopf.
    const name = document.createElement("span");
    name.className = "task-name";
    name.textContent = task.path;
    const title = document.createElement("span");
    title.className = "task-title";
    title.textContent = task.title || "";

    // Ein Task ohne Review-Log ist nie gegengelesen worden. /k-run fragt dann
    // vor der Ausführung nach — das soll man vorher sehen können.
    const review = document.createElement("span");
    review.className = task.reviewed ? "task-review" : "task-review open";
    review.textContent = task.reviewed
      ? task.reviewedAt
        ? `gereviewt ${task.reviewedAt}`
        : "gereviewt"
      : "ohne Review-Loop";

    button.append(name, title, review);

    button.addEventListener("click", () => openTask(task.path, task.title));
    elements.tasksList.append(button);
  }

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
  for (const button of elements.tasksList.querySelectorAll(".task-link")) {
    button.classList.toggle("active", button.dataset.path === path);
  }
}
