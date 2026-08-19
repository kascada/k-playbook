// Seite "Neuer Review": Auswahl zusammenstellen und den Lauf anlegen.
//
// Angelegt wird nur — gestartet wird nichts. Wie ein einzelner Eintrag läuft,
// ist ein eigener Schritt.

const elements = {
  pickPill: document.getElementById("pick-pill"),
  pickTools: document.getElementById("pick-tools"),
  pickReviews: document.getElementById("pick-reviews"),
  pickMessage: document.getElementById("pick-message"),
  pickCreate: document.getElementById("pick-create"),
  toolsHint: document.getElementById("tools-hint"),
  runsPill: document.getElementById("runs-pill"),
  runsList: document.getElementById("runs-list"),
  runsMessage: document.getElementById("runs-message"),
};

// Die Auswahl lebt hier und nicht im DOM: nach dem Anlegen wird alles neu
// gezeichnet, und was angehakt war, soll dabei nicht verlorengehen.
const picked = new Set();
let toolsPickedInitially = false;

function key(name, kind) {
  return `${kind}:${name}`;
}

async function load() {
  try {
    const response = await fetch("/api/reviews", { cache: "no-store" });
    render(await response.json());
  } catch {
    elements.pickMessage.textContent = "Stand konnte nicht geladen werden.";
  }
}

async function create() {
  const entries = [...picked].map((entry) => {
    const [kind, name] = splitKey(entry);
    return { name, kind };
  });

  elements.pickCreate.disabled = true;
  elements.pickPill.className = "pill muted";
  elements.pickPill.textContent = "Anlegen...";
  try {
    const response = await fetch("/api/reviews", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ entries }),
    });
    render(await response.json());
  } catch {
    elements.pickMessage.textContent = "Anlegen fehlgeschlagen.";
    elements.pickCreate.disabled = false;
  }
}

// Der Name darf keinen Doppelpunkt enthalten, die Art besteht aus Buchstaben —
// deshalb trennt der erste Doppelpunkt zuverlässig.
function splitKey(value) {
  const index = value.indexOf(":");
  return [value.slice(0, index), value.slice(index + 1)];
}

function render(data) {
  if (!data.available) {
    elements.pickPill.className = "pill muted";
    elements.pickPill.textContent = "Keine Installation";
    elements.pickMessage.textContent = "Für dieses Verzeichnis liegt keine Konfiguration vor.";
    return;
  }

  renderRuns(data);
  pickToolsInitially(data.tools || []);
  renderGroup(elements.pickTools, data.tools || []);
  renderGroup(elements.pickReviews, data.reviews || []);

  const languages = data.languages || [];
  elements.toolsHint.textContent = languages.length
    ? `Zuständig für ${languages.join(", ")}. Was fehlt, lässt sich nicht auswählen.`
    : "Keine Sprachen gewählt — nur sprachunabhängige Werkzeuge stehen zur Wahl.";

  if (data.created) {
    elements.pickPill.className = "pill ok";
    elements.pickPill.textContent = "Angelegt";
    elements.pickMessage.textContent = `Lauf angelegt: ${data.created}`;
    picked.clear();
    renderGroup(elements.pickTools, data.tools || []);
    renderGroup(elements.pickReviews, data.reviews || []);
  } else if (data.message) {
    elements.pickPill.className = "pill warn";
    elements.pickPill.textContent = "Nicht angelegt";
    elements.pickMessage.textContent = data.message;
  } else if (data.exists) {
    elements.pickPill.className = "pill warn";
    elements.pickPill.textContent = "Heute schon gelaufen";
    elements.pickMessage.textContent =
      `Für ${data.today} gibt es bereits einen Lauf. Ein Tag, ein Lauf — das vorhandene Verzeichnis erst wegräumen oder umbenennen.`;
  } else {
    elements.pickPill.className = "pill muted";
    elements.pickPill.textContent = data.today;
    elements.pickMessage.textContent = "";
  }

  syncCreateButton(data);
}

function pickToolsInitially(tools) {
  if (toolsPickedInitially) {
    return;
  }
  toolsPickedInitially = true;
  for (const tool of tools) {
    if (tool.available) {
      picked.add(key(tool.name, tool.kind));
    }
  }
}

function syncCreateButton(data) {
  const blocked = data.exists && !data.created;
  elements.pickCreate.disabled = blocked || picked.size === 0;
}

function renderGroup(container, choices) {
  container.replaceChildren();

  if (choices.length === 0) {
    const empty = document.createElement("p");
    empty.className = "message";
    empty.textContent = "Nichts vorhanden.";
    container.append(empty);
    return;
  }

  for (const choice of choices) {
    const id = key(choice.name, choice.kind);
    const label = document.createElement("label");
    label.className = choice.available ? "choice" : "choice disabled";

    const input = document.createElement("input");
    input.type = "checkbox";
    input.checked = picked.has(id);
    input.disabled = !choice.available;
    input.addEventListener("change", () => {
      if (input.checked) {
        picked.add(id);
      } else {
        picked.delete(id);
      }
      label.classList.toggle("selected", input.checked);
      elements.pickCreate.disabled = picked.size === 0;
    });

    const text = document.createElement("div");
    const title = document.createElement("div");
    title.className = "choice-label";
    title.textContent = choice.name;
    const description = document.createElement("p");
    description.className = "choice-description";
    description.textContent = choice.available
      ? choice.detail
      : `${choice.detail} — ${choice.reason}`;
    text.append(title, description);

    if (input.checked) {
      label.classList.add("selected");
    }
    label.append(input, text);
    container.append(label);
  }
}

function renderRuns(data) {
  elements.runsList.replaceChildren();
  const runs = data.runs || [];

  elements.runsPill.className = runs.length ? "pill ok" : "pill muted";
  elements.runsPill.textContent = runs.length ? `${runs.length}` : "keine";

  if (runs.length === 0) {
    elements.runsMessage.textContent = "Noch kein Lauf angelegt.";
    return;
  }
  elements.runsMessage.textContent = "";

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

elements.pickCreate.addEventListener("click", create);
// Ohne Lebenszeichen von dieser Seite beendet sich der Server wenige Sekunden
// nach dem Wechsel hierher — der Weg zurück führte dann ins Leere.
startSession((message) => {
  elements.pickMessage.textContent = message;
});
load();
