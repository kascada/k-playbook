const state = {
  status: null,
  scanned: [],
};

const elements = {
  refresh: document.querySelector("#refresh"),
  repair: document.querySelector("#repair"),
  statusPill: document.querySelector("#status-pill"),
  expected: document.querySelector("#expected"),
  current: document.querySelector("#current"),
  symlink: document.querySelector("#symlink"),
  statusMessage: document.querySelector("#status-message"),
  projectArea: document.querySelector("#project-area"),
  scan: document.querySelector("#scan"),
  scanResults: document.querySelector("#scan-results"),
  saveScan: document.querySelector("#save-scan"),
  manualForm: document.querySelector("#manual-form"),
  manualPath: document.querySelector("#manual-path"),
  manualEnv: document.querySelector("#manual-env"),
  projects: document.querySelector("#projects"),
  toast: document.querySelector("#toast"),
};

elements.refresh.addEventListener("click", refreshAll);
elements.repair.addEventListener("click", repairPath);
elements.scan.addEventListener("click", scanProjects);
elements.saveScan.addEventListener("click", saveScannedProjects);
elements.manualForm.addEventListener("submit", addManualProject);

refreshAll();

async function refreshAll() {
  await refreshStatus();
  if (state.status && state.status.OK) {
    await refreshProjects();
  }
}

async function refreshStatus() {
  const status = await api("/api/status");
  state.status = status;
  renderStatus(status);
}

async function repairPath() {
  if (!window.confirm("Symlink fuer ~/dev/k-playbook anlegen?")) {
    return;
  }

  await withBusy(elements.repair, async () => {
    const status = await api("/api/repair-path", { method: "POST" });
    state.status = status;
    renderStatus(status);
    if (status.OK) {
      showToast("Pfadvertrag repariert.");
      await refreshProjects();
    }
  });
}

async function refreshProjects() {
  const file = await api("/api/projects");
  renderProjects(file.projects || file.Projects || []);
}

async function scanProjects() {
  await withBusy(elements.scan, async () => {
    const projects = await api("/api/projects/scan");
    state.scanned = projects;
    renderScanned(projects);
  });
}

async function saveScannedProjects() {
  const selected = [...document.querySelectorAll(".scan-row")]
    .map((row) => ({
      path: row.dataset.path,
      environment: row.querySelector("select").value,
      selected: row.querySelector("input[type='checkbox']").checked,
    }))
    .filter((project) => project.selected);

  if (selected.length === 0) {
    showToast("Keine Projekte ausgewaehlt.", true);
    return;
  }

  await withBusy(elements.saveScan, async () => {
    const file = await api("/api/projects/scan", {
      method: "POST",
      body: JSON.stringify({ projects: selected }),
    });
    renderProjects(file.projects || file.Projects || []);
    showToast("Auswahl gespeichert.");
  });
}

async function addManualProject(event) {
  event.preventDefault();
  const path = elements.manualPath.value.trim();
  if (!path) {
    showToast("Bitte Projektpfad angeben.", true);
    return;
  }

  await withBusy(elements.manualForm.querySelector("button"), async () => {
    const file = await api("/api/projects", {
      method: "POST",
      body: JSON.stringify({
        path,
        environment: elements.manualEnv.value,
        selected: true,
      }),
    });
    elements.manualPath.value = "";
    renderProjects(file.projects || file.Projects || []);
    showToast("Projekt gespeichert.");
  });
}

function renderStatus(status) {
  elements.expected.textContent = status.Expected || "-";
  elements.current.textContent = status.Current || "nicht erkannt";
  elements.symlink.textContent = status.ExpectedIsSymlink ? status.ExpectedSymlinkTarget : "-";
  elements.statusMessage.textContent = status.Message || "";
  elements.statusPill.textContent = status.Code || "UNKNOWN";
  elements.statusPill.className = "pill";

  if (status.OK) {
    elements.statusPill.classList.add("ok");
  } else if (status.Fixable) {
    elements.statusPill.classList.add("warn");
  } else {
    elements.statusPill.classList.add("error");
  }

  elements.repair.classList.toggle("hidden", status.OK || !status.Fixable);
  elements.projectArea.classList.toggle("hidden", !status.OK);
}

function renderScanned(projects) {
  elements.scanResults.innerHTML = "";
  elements.scanResults.classList.toggle("empty", projects.length === 0);
  elements.saveScan.classList.toggle("hidden", projects.length === 0);

  if (projects.length === 0) {
    elements.scanResults.textContent = "Keine Projekte unter ~/dev gefunden.";
    return;
  }

  for (const project of projects) {
    const row = document.createElement("div");
    row.className = "scan-row";
    row.dataset.path = project.path || project.Path;

    const checkbox = document.createElement("input");
    checkbox.type = "checkbox";

    const details = document.createElement("div");
    const path = document.createElement("div");
    path.className = "path";
    path.textContent = project.path || project.Path;
    const meta = document.createElement("div");
    meta.className = "meta";
    meta.textContent = detectedLabel(project);
    details.append(path, meta);

    const select = environmentSelect(project.environment || project.Environment || "plain");
    row.append(checkbox, details, select);
    elements.scanResults.append(row);
  }
}

function renderProjects(projects) {
  elements.projects.innerHTML = "";
  elements.projects.classList.toggle("empty", projects.length === 0);

  if (projects.length === 0) {
    elements.projects.textContent = "Keine Projekte gespeichert.";
    return;
  }

  for (const project of projects) {
    const row = document.createElement("div");
    row.className = "project-row";

    const details = document.createElement("div");
    const path = document.createElement("div");
    path.className = "path";
    path.textContent = project.path || project.Path;
    const meta = document.createElement("div");
    meta.className = "meta";
    meta.textContent = detectedLabel(project);
    details.append(path, meta);

    const pill = document.createElement("span");
    pill.className = "pill ok";
    pill.textContent = (project.environment || project.Environment || "unknown") + (project.selected === false || project.Selected === false ? " / off" : "");
    row.append(details, pill);
    elements.projects.append(row);
  }
}

function detectedLabel(project) {
  const environment = project.environment || project.Environment || "unknown";
  const detected = project.detected || project.Detected || [];
  return detected.length > 0 ? `${environment} - ${detected.join(", ")}` : environment;
}

function environmentSelect(value) {
  const select = document.createElement("select");
  const options = [
    ["", "Automatisch erkennen"],
    ["plain", "Normal"],
    ["venv", "Python venv"],
    ["devcontainer", "DevContainer"],
    ["unknown", "Unbekannt"],
  ];
  for (const [optionValue, label] of options) {
    const option = document.createElement("option");
    option.value = optionValue;
    option.textContent = label;
    option.selected = optionValue === value;
    select.append(option);
  }
  return select;
}

async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  const data = await response.json();
  if (!response.ok) {
    throw new Error(data.error || "Unbekannter Fehler");
  }
  return data;
}

async function withBusy(button, callback) {
  button.disabled = true;
  try {
    await callback();
  } catch (error) {
    showToast(error.message, true);
  } finally {
    button.disabled = false;
  }
}

function showToast(message, error = false) {
  elements.toast.textContent = message;
  elements.toast.className = "toast" + (error ? " error" : "");
  window.clearTimeout(showToast.timeout);
  showToast.timeout = window.setTimeout(() => {
    elements.toast.classList.add("hidden");
  }, 4200);
}
