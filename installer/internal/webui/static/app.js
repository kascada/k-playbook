const state = {
  status: null,
  scanned: [],
};

const elements = {
  refresh: document.querySelector("#refresh"),
  shutdown: document.querySelector("#shutdown"),
  backHome: document.querySelector("#back-home"),
  openScan: document.querySelector("#open-scan"),
  cancelScan: document.querySelector("#cancel-scan"),
  gitPull: document.querySelector("#git-pull"),
  gitOutput: document.querySelector("#git-output"),
  reloadDocs: document.querySelector("#reload-docs"),
  docsList: document.querySelector("#docs-list"),
  docViewer: document.querySelector("#doc-viewer"),
  repair: document.querySelector("#repair"),
  statusCard: document.querySelector("#status-card"),
  statusHead: document.querySelector("#status-head"),
  statusPill: document.querySelector("#status-pill"),
  statusCompact: document.querySelector("#status-compact"),
  compactText: document.querySelector("#compact-text"),
  statusDetails: document.querySelector("#status-details"),
  expected: document.querySelector("#expected"),
  current: document.querySelector("#current"),
  symlink: document.querySelector("#symlink"),
  statusMessage: document.querySelector("#status-message"),
  projectArea: document.querySelector("#project-area"),
  scanRoot: document.querySelector("#scan-root"),
  scan: document.querySelector("#scan"),
  scanResults: document.querySelector("#scan-results"),
  saveScan: document.querySelector("#save-scan"),
  manualForm: document.querySelector("#manual-form"),
  manualPath: document.querySelector("#manual-path"),
  manualEnv: document.querySelector("#manual-env"),
  projects: document.querySelector("#projects"),
  toast: document.querySelector("#toast"),
  closed: document.querySelector("#closed"),
};

elements.refresh.addEventListener("click", refreshAll);
elements.shutdown.addEventListener("click", shutdownInstaller);
elements.backHome.addEventListener("click", showHome);
elements.openScan.addEventListener("click", showScan);
elements.cancelScan.addEventListener("click", showHome);
elements.gitPull.addEventListener("click", gitPull);
elements.reloadDocs.addEventListener("click", loadDocs);
elements.repair.addEventListener("click", repairPath);
elements.scan.addEventListener("click", scanProjects);
elements.saveScan.addEventListener("click", saveScannedProjects);
elements.manualForm.addEventListener("submit", addManualProject);

refreshAll();
showHome();

function showHome() {
  for (const element of document.querySelectorAll("[data-view='home']")) {
    element.classList.remove("hidden");
  }
  for (const element of document.querySelectorAll("[data-view='scan']")) {
    element.classList.add("hidden");
  }
}

function showScan() {
  for (const element of document.querySelectorAll("[data-view='home']")) {
    element.classList.add("hidden");
  }
  for (const element of document.querySelectorAll("[data-view='scan']")) {
    element.classList.remove("hidden");
  }
}

async function refreshAll() {
  await refreshStatus();
  if (state.status && state.status.OK) {
    await refreshProjects();
    await loadDocs();
  }
}

async function gitPull() {
  await withBusy(elements.gitPull, async () => {
    elements.gitOutput.textContent = "git pull laeuft...";
    const result = await api("/api/git/pull", { method: "POST" });
    elements.gitOutput.textContent = result.output || "Bereits aktuell.";
    showToast("Repository aktualisiert.");
    await refreshAll();
  });
}

async function shutdownInstaller() {
  if (!window.confirm("Installer wirklich beenden?")) {
    return;
  }

  await withBusy(elements.shutdown, async () => {
    await api("/api/shutdown", { method: "POST" });
    document.body.classList.add("is-closed");
    elements.closed.classList.remove("hidden");
  });
}

async function loadDocs() {
  await withBusy(elements.reloadDocs, async () => {
    const docs = await api("/api/docs");
    renderDocsList(docs);
  });
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
    const projects = await api(`/api/projects/scan?root=${encodeURIComponent(elements.scanRoot.value)}`);
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
    showHome();
  });
  updateSaveScanButton();
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
    showHome();
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
  elements.statusCard.classList.toggle("status-ok", status.OK);
  elements.statusHead.classList.toggle("hidden", status.OK);
  elements.statusCompact.classList.toggle("hidden", !status.OK);
  elements.statusDetails.classList.toggle("hidden", status.OK);
  elements.statusMessage.classList.toggle("hidden", status.OK);
  elements.compactText.textContent = status.Expected ? `${status.Expected} ist bereit.` : "Pfadvertrag ist bereit.";
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
    checkbox.addEventListener("change", () => updateScanRow(row));

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
    row.addEventListener("click", (event) => {
      if (event.target === checkbox || event.target === select) {
        return;
      }
      checkbox.checked = !checkbox.checked;
      updateScanRow(row);
    });
    elements.scanResults.append(row);
  }
  updateSaveScanButton();
}

function updateScanRow(row) {
  const checkbox = row.querySelector("input[type='checkbox']");
  row.classList.toggle("selected", checkbox.checked);
  updateSaveScanButton();
}

function updateSaveScanButton() {
  const selectedCount = document.querySelectorAll(".scan-row input[type='checkbox']:checked").length;
  elements.saveScan.textContent = selectedCount === 1 ? "1 Projekt speichern" : `${selectedCount} Projekte speichern`;
  elements.saveScan.disabled = selectedCount === 0;
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

function renderDocsList(docs) {
  elements.docsList.innerHTML = "";
  elements.docsList.classList.toggle("empty", docs.length === 0);

  if (docs.length === 0) {
    elements.docsList.textContent = "Keine Markdown-Dateien in docs/ gefunden.";
    return;
  }

  for (const doc of docs) {
    const button = document.createElement("button");
    button.className = "doc-link";
    button.type = "button";
    button.textContent = doc.title || doc.path;
    button.title = doc.path;
    button.addEventListener("click", async () => {
      for (const active of document.querySelectorAll(".doc-link.active")) {
        active.classList.remove("active");
      }
      button.classList.add("active");
      await loadDoc(doc.path);
    });
    elements.docsList.append(button);
  }
}

async function loadDoc(path) {
  const doc = await api(`/api/docs/file?path=${encodeURIComponent(path)}`);
  elements.docViewer.classList.remove("empty");
  elements.docViewer.innerHTML = doc.html || "";
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
