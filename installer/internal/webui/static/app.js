const state = {
  status: null,
  runtime: null,
  scanned: [],
  devcontainers: null,
  securityTools: null,
  gitStatus: null,
  projects: [],
  currentDocPath: "",
};

const elements = {
  refresh: document.querySelector("#refresh"),
  shutdown: document.querySelector("#shutdown"),
  backHome: document.querySelector("#back-home"),
  backProjects: document.querySelector("#back-projects"),
  openScan: document.querySelector("#open-scan"),
  smokeAll: document.querySelector("#smoke-all"),
  smokeOutput: document.querySelector("#smoke-output"),
  runtimeBanner: document.querySelector("#runtime-banner"),
  projectScopeNote: document.querySelector("#project-scope-note"),
  cancelScan: document.querySelector("#cancel-scan"),
  gitPullTop: document.querySelector("#git-pull-top"),
  gitPull: document.querySelector("#git-pull"),
  gitOutput: document.querySelector("#git-output"),
  opencodeInstallTop: document.querySelector("#opencode-install-top"),
  opencodeInstall: document.querySelector("#opencode-install"),
  opencodeRefresh: document.querySelector("#opencode-refresh"),
  opencodePill: document.querySelector("#opencode-pill"),
  opencodeSummary: document.querySelector("#opencode-summary"),
  securityToolsRefresh: document.querySelector("#security-tools-refresh"),
  securityToolsPill: document.querySelector("#security-tools-pill"),
  securityToolsSummary: document.querySelector("#security-tools-summary"),
  reloadDocs: document.querySelector("#reload-docs"),
  docsList: document.querySelector("#docs-list"),
  docOverlay: document.querySelector("#doc-overlay"),
  docTitle: document.querySelector("#doc-title"),
  docPath: document.querySelector("#doc-path"),
  closeDoc: document.querySelector("#close-doc"),
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
  projectDetailTitle: document.querySelector("#project-detail-title"),
  projectDetailPath: document.querySelector("#project-detail-path"),
  projectDetailStatus: document.querySelector("#project-detail-status"),
  projectSmokeOutput: document.querySelector("#project-smoke-output"),
  openProjectConfig: document.querySelector("#open-project-config"),
  reloadProjectConfig: document.querySelector("#reload-project-config"),
  projectConfig: document.querySelector("#project-config"),
  manualForm: document.querySelector("#manual-form"),
  manualPath: document.querySelector("#manual-path"),
  manualEnv: document.querySelector("#manual-env"),
  projects: document.querySelector("#projects"),
  toast: document.querySelector("#toast"),
  closed: document.querySelector("#closed"),
  closedTitle: document.querySelector("#closed-title"),
  closedMessage: document.querySelector("#closed-message"),
};

let serverAvailable = true;
let currentProjectPath = "";

elements.refresh.addEventListener("click", refreshAll);
elements.shutdown.addEventListener("click", shutdownInstaller);
elements.backHome.addEventListener("click", goBackToHome);
elements.backProjects.addEventListener("click", goBackToHome);
elements.openScan.addEventListener("click", showScan);
elements.smokeAll.addEventListener("click", smokeAllProjects);
elements.cancelScan.addEventListener("click", showHome);
elements.gitPullTop.addEventListener("click", gitPull);
elements.gitPull.addEventListener("click", gitPull);
elements.opencodeInstallTop.addEventListener("click", installOpenCode);
elements.opencodeInstall.addEventListener("click", installOpenCode);
elements.opencodeRefresh.addEventListener("click", loadOpenCodeStatus);
elements.securityToolsRefresh.addEventListener("click", loadSecurityToolsStatus);
elements.reloadDocs.addEventListener("click", loadDocs);
elements.closeDoc.addEventListener("click", closeDocOverlay);
elements.docOverlay.addEventListener("click", (event) => {
  if (event.target === elements.docOverlay) {
    closeDocOverlay();
  }
});
elements.repair.addEventListener("click", repairPath);
elements.scan.addEventListener("click", scanProjects);
elements.reloadProjectConfig.addEventListener("click", () => loadProjectConfig(currentProjectPath));
elements.manualForm.addEventListener("submit", addManualProject);

startApp();
showHome({ replaceHistory: true });
startHealthChecks();
window.addEventListener("popstate", handleHistoryNavigation);
window.addEventListener("pagehide", notifyClientGone);
window.addEventListener("keydown", (event) => {
  if (event.key === "Escape" && !elements.docOverlay.classList.contains("hidden")) {
    closeDocOverlay();
  }
});

function showHome(options = {}) {
  currentProjectPath = "";
  showView("home");
  updateHistory({ view: "home" }, options);
}

function showScan(options = {}) {
  showView("scan");
  updateHistory({ view: "scan" }, options);
}

function showProjectDetail(options = {}) {
  showView("project-detail");
  updateHistory({ view: "project-detail", projectPath: currentProjectPath }, options);
}

function showView(name) {
  for (const element of document.querySelectorAll("[data-view]")) {
    element.classList.toggle("hidden", element.dataset.view !== name);
  }
}

function goBackToHome() {
  const current = window.history.state || {};
  if (current.view === "scan" || current.view === "project-detail") {
    window.history.back();
    return;
  }
  showHome({ replaceHistory: true });
}

function handleHistoryNavigation(event) {
  const viewState = event.state || { view: "home" };
  if (viewState.view === "scan") {
    showScan({ skipHistory: true });
    return;
  }
  if (viewState.view === "project-detail" && viewState.projectPath) {
    openProjectDetail(viewState.projectPath, { skipHistory: true });
    return;
  }
  showHome({ skipHistory: true });
}

function updateHistory(viewState, options = {}) {
  if (options.skipHistory || !window.history || !window.history.pushState) {
    return;
  }

  const current = window.history.state || {};
  if (!options.replaceHistory && sameHistoryState(current, viewState)) {
    return;
  }

  const method = options.replaceHistory ? "replaceState" : "pushState";
  window.history[method](viewState, "", window.location.href);
}

function sameHistoryState(left, right) {
  return left.view === right.view && (left.projectPath || "") === (right.projectPath || "");
}

async function refreshAll() {
  await loadRuntimeStatus();
  await refreshStatus();
  if (state.status && statusOK(state.status)) {
    await loadInitialHomeStatus();
  }
}

async function startApp() {
  try {
    await refreshAll();
  } catch (error) {
    showToast(`Initialer Status konnte nicht geladen werden: ${error.message}`, true);
  }
}

async function loadInitialHomeStatus() {
  await runOptionalInitialCheck(loadGitStatus);
  await runOptionalInitialCheck(loadOpenCodeStatus);
  await runOptionalInitialCheck(refreshProjects);
  await runOptionalInitialCheck(loadDevcontainerStatus);
  await runOptionalInitialCheck(loadSecurityToolsStatus);
  await runOptionalInitialCheck(loadDocs);
}

async function runOptionalInitialCheck(callback) {
  try {
    await callback();
  } catch (error) {
    showToast(error.message, true);
  }
}

async function gitPull() {
  await withBusyMany([elements.gitPull, elements.gitPullTop], async () => {
    renderLoading(elements.gitOutput, "git pull laeuft...");
    try {
      const result = await api("/api/git/pull", { method: "POST" });
      elements.gitOutput.classList.remove("empty");
      const messages = [result.output || "Bereits aktuell."];
      if (result.installerMessage) {
        messages.push(result.installerMessage);
      }
      await refreshAll();
      elements.gitOutput.classList.remove("empty");
      elements.gitOutput.textContent = messages.join("\n\n");
      showToast(result.installerRestartRequired ? "Repository aktualisiert. Installer bitte neu starten." : "Repository aktualisiert.");
    } catch (error) {
      renderInlineMessage(elements.gitOutput, `git pull fehlgeschlagen: ${error.message}`);
      throw error;
    }
  });
}

async function loadGitStatus() {
  try {
    renderLoading(elements.gitOutput, "Update-Status wird geprueft...");
    const status = await api("/api/git/status");
    state.gitStatus = status;
    renderGitStatus(status);
  } catch (error) {
    state.gitStatus = null;
    renderGitStatus(null, error.message);
  }
}

function renderGitStatus(status, error = "") {
  elements.gitOutput.classList.remove("empty");
  const updateAvailable = Boolean(status && (status.updateAvailable || status.UpdateAvailable));
  const buttons = [
    [elements.gitPullTop, "k-playbook aktualisieren"],
    [elements.gitPull, "Git pull"],
  ];
  for (const [button, defaultLabel] of buttons) {
    button.classList.toggle("primary", updateAvailable);
    button.classList.toggle("secondary", !updateAvailable);
    button.classList.toggle("attention-highlight", updateAvailable);
    button.textContent = updateAvailable ? "Zur neuen Version aktualisieren" : defaultLabel;
  }

  if (error) {
    elements.gitOutput.textContent = `Update-Check nicht moeglich: ${error}`;
    return;
  }
  if (!status) {
    elements.gitOutput.textContent = "Fuehrt im k-playbook-Repo ein sicheres `git pull --ff-only` aus.";
    return;
  }

  const message = status.message || status.Message || "Git-Status geprueft.";
  elements.gitOutput.textContent = updateAvailable
    ? `${message} Aktualisieren fuehrt git pull --ff-only aus.`
    : message;
}

async function shutdownInstaller() {
  if (!window.confirm("Installer wirklich beenden?")) {
    return;
  }

  await withBusy(elements.shutdown, async () => {
    await api("/api/shutdown", { method: "POST" });
    showClosed();
  });
}

function startHealthChecks() {
  window.setInterval(checkHealth, 1800);
}

async function checkHealth() {
  if (!serverAvailable) {
    return;
  }

  try {
    await fetch("/api/health", { cache: "no-store" });
  } catch {
    serverAvailable = false;
    showClosed("Verbindung zum lokalen Installer verloren.");
  }
}

function showClosed(message = "") {
  serverAvailable = false;
  document.body.classList.add("is-closed");
  elements.closedTitle.textContent = "Dieses Browserfenster kann jetzt geschlossen werden.";
  elements.closedMessage.textContent = message;
  elements.closedMessage.classList.toggle("hidden", !message);
  elements.closed.classList.remove("hidden");
}

function notifyClientGone() {
  if (!serverAvailable) {
    return;
  }

  if (navigator.sendBeacon) {
    navigator.sendBeacon("/api/client-gone");
    return;
  }

  fetch("/api/client-gone", { method: "POST", keepalive: true }).catch(() => {});
}

async function loadDocs() {
  await withBusy(elements.reloadDocs, async () => {
    closeDocOverlay();
    renderLoading(elements.docsList, "Docs werden geladen...");
    try {
      const docs = await api("/api/docs");
      renderDocsList(docs);
    } catch (error) {
      renderInlineMessage(elements.docsList, `Docs konnten nicht geladen werden: ${error.message}`);
      throw error;
    }
  });
}

async function loadOpenCodeStatus() {
  await withBusy(elements.opencodeRefresh, async () => {
    renderOpenCodeLoading();
    try {
      const status = await api("/api/opencode/status");
      state.opencode = status;
      renderOpenCode(status);
    } catch (error) {
      renderInlineMessage(elements.opencodeSummary, `Registrierung konnte nicht geprueft werden: ${error.message}`);
      throw error;
    }
  });
}

async function loadRuntimeStatus() {
  const runtime = await api("/api/runtime");
  state.runtime = runtime;
  renderRuntime(runtime);
}

function renderRuntime(runtime) {
  const insideContainer = Boolean(runtime && (runtime.insideContainer || runtime.InsideContainer));
  const insideDevcontainer = Boolean(runtime && (runtime.insideDevcontainer || runtime.InsideDevcontainer));
  document.body.classList.toggle("runtime-container", insideContainer);

  if (!insideContainer) {
    elements.runtimeBanner.classList.add("hidden");
    elements.runtimeBanner.textContent = "";
    elements.projectScopeNote.classList.add("hidden");
    elements.projectScopeNote.textContent = "";
    elements.openScan.disabled = false;
    elements.smokeAll.disabled = false;
    return;
  }

  const currentProject = runtime.currentProject || runtime.CurrentProject || "";
  const title = insideDevcontainer ? "DevContainer-Modus" : "Container-Modus";
  const message = runtime.message || runtime.Message || "Installer laeuft im Container-Kontext.";
  elements.runtimeBanner.classList.remove("hidden");
  elements.runtimeBanner.textContent = currentProject ? `${title}: ${message} Aktuelles Projekt: ${currentProject}` : `${title}: ${message}`;
  elements.projectScopeNote.classList.remove("hidden");
  elements.projectScopeNote.textContent = currentProject
    ? `Container-Modus: Nur ${currentProject} ist bearbeitbar. Andere gespeicherte Projekte sind Host-Kontext und hier deaktiviert.`
    : "Container-Modus: Kein aktuelles Projekt erkannt. Gespeicherte Host-Projekte sind hier deaktiviert.";
  elements.openScan.disabled = !currentProject;
  elements.smokeAll.disabled = !currentProject;
}

async function loadSecurityToolsStatus() {
  await withBusy(elements.securityToolsRefresh, async () => {
    renderSecurityToolsLoading();
    try {
      const status = await api("/api/security-tools/status");
      state.securityTools = status;
      renderSecurityTools(status);
    } catch (error) {
      renderInlineMessage(elements.securityToolsSummary, `Security-Tools konnten nicht geprueft werden: ${error.message}`);
      throw error;
    }
  });
}

async function installOpenCode() {
  await withBusyMany([elements.opencodeInstall, elements.opencodeInstallTop], async () => {
    const result = await api("/api/opencode/install", { method: "POST" });
    state.opencode = result.status;
    renderOpenCode(result.status);
    showToast(result.message || "OpenCode-Registrierung aktualisiert.");
  });
}

async function loadDevcontainerStatus() {
  const status = await api("/api/devcontainer/status");
  state.devcontainers = status;
  renderProjects(state.projects);
}

async function installDevcontainer(projectPath, button) {
  if (!window.confirm(`DevContainer-Integration fuer ${projectPath} eintragen?`)) {
    return;
  }

  await withBusy(button, async () => {
    const result = await api("/api/devcontainer/install", {
      method: "POST",
      body: JSON.stringify({ path: projectPath }),
    });
    state.devcontainers = result.status;
    renderProjects(state.projects);
    if (currentProjectPath === projectPath) {
      renderProjectDetailStatus(findProject(projectPath));
    }
    showToast(result.message || "DevContainer-Integration aktualisiert.");
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
    if (statusOK(status)) {
      showToast("Pfadvertrag repariert.");
      await loadInitialHomeStatus();
    }
  });
}

async function refreshProjects() {
  renderLoading(elements.projects, "Projekt-Auswahl wird geladen...");
  try {
    const file = await api("/api/projects");
    state.runtime = file.runtime || file.Runtime || state.runtime;
    renderRuntime(state.runtime);
    state.projects = file.projects || file.Projects || [];
    renderProjects(state.projects);
  } catch (error) {
    renderInlineMessage(elements.projects, `Projekt-Auswahl konnte nicht geladen werden: ${error.message}`);
    throw error;
  }
}

function renderLoading(element, text) {
  element.innerHTML = "";
  element.classList.add("empty");
  const loading = document.createElement("span");
  loading.className = "loading-inline";
  const spinner = document.createElement("span");
  spinner.className = "loading-spinner";
  spinner.setAttribute("aria-hidden", "true");
  loading.append(spinner, text);
  element.append(loading);
}

function renderInlineMessage(element, text) {
  element.innerHTML = "";
  element.classList.add("empty");
  element.textContent = text;
}

async function scanProjects() {
  await withBusy(elements.scan, async () => {
    renderLoading(elements.scanResults, "Projekte werden gesucht...");
    try {
      const projects = await api(`/api/projects/scan?root=${encodeURIComponent(elements.scanRoot.value)}`);
      state.scanned = projects;
      renderScanned(projects);
    } catch (error) {
      renderInlineMessage(elements.scanResults, `Scan fehlgeschlagen: ${error.message}`);
      throw error;
    }
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
    const preview = await api("/api/projects/preview", {
      method: "POST",
      body: JSON.stringify({
        path,
        environment: elements.manualEnv.value,
        selected: true,
      }),
    });
    const environment = preview.environment || preview.Environment || "unknown";
    if (!elements.manualEnv.value && environment === "unknown") {
      showToast("Projektart konnte nicht sicher erkannt werden. Bitte Umgebung auswaehlen.", true);
      return;
    }

    const file = await saveProject(path, environment);
    elements.manualPath.value = "";
    elements.manualEnv.value = "";
    state.projects = file.projects || file.Projects || [];
    renderProjects(state.projects);
    await loadDevcontainerStatus();
    showToast("Projekt gespeichert. K-PLAYBOOK.yaml ist vorhanden.");
    showHome({ replaceHistory: true });
  });
}

async function saveProject(path, environment) {
  return api("/api/projects", {
    method: "POST",
    body: JSON.stringify({ path, environment, selected: true }),
  });
}

function renderStatus(status) {
  const ok = statusOK(status);
  const fixable = Boolean(status.fixable || status.Fixable);
  const expected = status.expected || status.Expected || "";
  const current = status.current || status.Current || "";
  const expectedIsSymlink = Boolean(status.expectedIsSymlink || status.ExpectedIsSymlink);
  const expectedSymlinkTarget = status.expectedSymlinkTarget || status.ExpectedSymlinkTarget || "";
  elements.expected.textContent = expected || "-";
  elements.current.textContent = current || "nicht erkannt";
  elements.symlink.textContent = expectedIsSymlink ? expectedSymlinkTarget : "-";
  elements.statusMessage.textContent = status.message || status.Message || "";
  elements.statusPill.textContent = status.code || status.Code || "UNKNOWN";
  elements.statusPill.className = "pill";

  if (ok) {
    elements.statusPill.classList.add("ok");
  } else if (fixable) {
    elements.statusPill.classList.add("warn");
  } else {
    elements.statusPill.classList.add("error");
  }

  elements.repair.classList.toggle("hidden", ok || !fixable);
  elements.projectArea.classList.toggle("hidden", !ok);
  elements.statusCard.classList.toggle("status-ok", ok);
  elements.statusHead.classList.toggle("hidden", ok);
  elements.statusCompact.classList.toggle("hidden", !ok);
  elements.statusDetails.classList.toggle("hidden", ok);
  elements.statusMessage.classList.toggle("hidden", ok);
  elements.compactText.textContent = expected ? `${expected} ist bereit.` : "Pfadvertrag ist bereit.";
}

function statusOK(status) {
  return Boolean(status && (status.ok || status.OK));
}

function renderScanned(projects) {
  elements.scanResults.innerHTML = "";
  elements.scanResults.classList.toggle("empty", projects.length === 0);

  if (projects.length === 0) {
    elements.scanResults.textContent = "Keine Projekte unter ~/dev gefunden.";
    return;
  }

  for (const project of projects) {
    const row = document.createElement("div");
    row.className = "scan-row";
    row.dataset.path = project.path || project.Path;

    const details = document.createElement("div");
    const path = document.createElement("div");
    path.className = "path";
    path.textContent = project.path || project.Path;
    const meta = document.createElement("div");
    meta.className = "meta";
    meta.textContent = detectedLabel(project);
    details.append(path, meta);

    const detectedEnvironment = project.environment || project.Environment || "unknown";
    const select = environmentSelect(detectedEnvironment === "unknown" ? "" : detectedEnvironment, true);

    const add = document.createElement("button");
    add.type = "button";
    add.className = "primary small-button";
    add.textContent = "Hinzufuegen";

    const action = document.createElement("div");
    action.className = "scan-row-action";
    action.append(select, add);

    select.addEventListener("change", () => selectScanRow(row));
    add.addEventListener("click", async (event) => {
      event.stopPropagation();
      await addScannedProject(row, select, add);
    });

    row.append(details, action);
    row.addEventListener("click", (event) => {
      if (event.target === select || event.target === add) {
        return;
      }
      selectScanRow(row);
    });
    elements.scanResults.append(row);
  }
}

function selectScanRow(row) {
  for (const current of document.querySelectorAll(".scan-row")) {
    current.classList.toggle("selected", current === row);
  }
}

async function addScannedProject(row, select, button) {
  selectScanRow(row);
  const environment = select.value;
  if (!environment) {
    showToast("Projektart unbekannt. Bitte Normal oder DevContainer auswaehlen.", true);
    return;
  }

  await withBusy(button, async () => {
    const file = await saveProject(row.dataset.path, environment);
    state.projects = file.projects || file.Projects || [];
    renderProjects(state.projects);
    await loadDevcontainerStatus();
    showToast("Projekt gespeichert. K-PLAYBOOK.yaml ist vorhanden.");
    showHome({ replaceHistory: true });
  });
}

function renderProjects(projects) {
  state.projects = projects;
  elements.projects.innerHTML = "";
  elements.projects.classList.toggle("empty", projects.length === 0);

  if (projects.length === 0) {
    elements.projects.textContent = "Keine Projekte gespeichert.";
    return;
  }

  for (const project of projects) {
    elements.projects.append(projectSummaryCard(project));
  }
}

function projectSummaryCard(project) {
  const card = document.createElement("div");
  const projectPath = project.path || project.Path;
  const editable = isProjectEditablePath(projectPath);
  card.className = editable ? "project-row clickable-project" : "project-row project-disabled";
  if (editable) {
    card.tabIndex = 0;
    card.setAttribute("role", "button");
    card.setAttribute("aria-label", `Details fuer ${projectPath} anzeigen`);
    card.addEventListener("click", () => openProjectDetail(projectPath));
    card.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        openProjectDetail(projectPath);
      }
    });
  }
  card.append(projectHeader(project, { showDetails: editable, showRemove: false }));
  card.append(projectStatusList(projectStatus(project)));
  if (!editable) {
    const note = document.createElement("div");
    note.className = "disabled-note";
    note.textContent = "Host-Projekt: im Container-Modus nicht bearbeitbar.";
    card.append(note);
  }
  return card;
}

function projectEditorCard(project) {
  const card = document.createElement("div");
  const editable = isProjectEditablePath(project.path || project.Path);
  card.className = "project-row project-editor";
  card.append(projectHeader(project, { showDetails: false, showRemove: editable, showSmoke: editable }));
  if (!editable) {
    const note = document.createElement("div");
    note.className = "disabled-note";
    note.textContent = "Dieses Projekt gehoert nicht zum aktuellen Container-Kontext und ist hier nur sichtbar.";
    card.append(note);
  }

  for (const item of projectStatus(project)) {
    card.append(projectEditorStatusRow(project, item));
  }

  return card;
}

function projectHeader(project, options = {}) {
  const showDetails = options.showDetails !== false;
  const showRemove = options.showRemove !== false;
  const showSmoke = options.showSmoke === true;

  const header = document.createElement("div");
  header.className = "project-header";
  const details = document.createElement("div");
  const path = document.createElement("div");
  path.className = "path";
  path.textContent = project.path || project.Path;
  const meta = document.createElement("div");
  meta.className = "meta";
  meta.textContent = detectedLabel(project);
  details.append(path, meta);

  const environment = document.createElement("span");
  environment.className = "project-env";
  environment.textContent = environmentLabel(project.environment || project.Environment || "unknown") + (project.selected === false || project.Selected === false ? " / off" : "");
  const headerActions = document.createElement("div");
  headerActions.className = "project-header-actions";
  headerActions.append(environment);
  if (showDetails) {
    headerActions.append(projectDetailsButton(project));
  }
  if (showSmoke) {
    headerActions.append(projectSmokeButton(project));
  }
  if (showRemove) {
    headerActions.append(removeProjectButton(project));
  }
  header.append(details, headerActions);
  return header;
}

function projectStatusList(items) {
  const list = document.createElement("div");
  list.className = "project-status-list";

  for (const item of items) {
    const row = document.createElement("div");
    row.className = `project-status-item ${item.state}`;
    const label = document.createElement("strong");
    label.textContent = item.label;
    const value = document.createElement("span");
    value.textContent = item.value;
    row.append(label, value);
    list.append(row);
  }

  return list;
}

function projectDetailsButton(project) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "secondary small-button";
  button.textContent = "Details";
  button.addEventListener("click", () => openProjectDetail(project.path || project.Path));
  return button;
}

function projectSmokeButton(project) {
  const projectPath = project.path || project.Path;
  const button = document.createElement("button");
  button.type = "button";
  button.className = "secondary small-button";
  button.textContent = "Smoke-Test";
  button.addEventListener("click", () => smokeProject(projectPath, button));
  return button;
}

async function openProjectDetail(projectPath, options = {}) {
  if (!isProjectEditablePath(projectPath)) {
    showToast("Im Container-Modus ist nur das aktuelle Projekt bearbeitbar.", true);
    return;
  }
  currentProjectPath = projectPath;
  const project = findProject(projectPath);
  elements.projectDetailTitle.textContent = project ? (project.name || project.Name || "Projekt") : "Projekt";
  elements.projectDetailPath.textContent = projectPath;
  renderProjectDetailStatus(project);
  elements.projectSmokeOutput.classList.add("hidden");
  showProjectDetail(options);
  await loadProjectConfig(projectPath);
}

function findProject(projectPath) {
  return state.projects.find((project) => (project.path || project.Path) === projectPath) || null;
}

function isProjectEditablePath(projectPath) {
  const runtime = state.runtime || {};
  const insideContainer = Boolean(runtime.insideContainer || runtime.InsideContainer);
  if (!insideContainer) {
    return true;
  }
  const currentProject = runtime.currentProject || runtime.CurrentProject || "";
  return Boolean(currentProject && projectPath === currentProject);
}

function renderProjectDetailStatus(project) {
  elements.projectDetailStatus.innerHTML = "";
  if (!project) {
    elements.projectDetailStatus.classList.add("empty");
    elements.projectDetailStatus.textContent = "Projekt nicht gefunden.";
    return;
  }
  elements.projectDetailStatus.classList.remove("empty");
  elements.projectDetailStatus.append(projectEditorCard(project));
}

async function smokeAllProjects() {
  await withBusy(elements.smokeAll, async () => {
    renderLoading(elements.smokeOutput, "Smoke-Test fuer alle Projekte laeuft...");
    elements.smokeOutput.classList.remove("hidden");
    try {
      const result = await api("/api/projects/smoke-all", { method: "POST" });
      renderSmokeAllResult(elements.smokeOutput, result);
      showToast(result.message || result.Message || "Smoke-Test abgeschlossen.", !(result.ok || result.OK));
    } catch (error) {
      renderInlineMessage(elements.smokeOutput, `Smoke-Test fehlgeschlagen: ${error.message}`);
      throw error;
    }
  });
}

async function smokeProject(projectPath, button) {
  await withBusy(button, async () => {
    const output = currentProjectPath === projectPath ? elements.projectSmokeOutput : elements.smokeOutput;
    output.classList.remove("hidden");
    renderLoading(output, `Smoke-Test laeuft: ${projectPath}`);
    try {
      const result = await api("/api/projects/smoke", {
        method: "POST",
        body: JSON.stringify({ path: projectPath }),
      });
      renderSmokeAllResult(output, { ok: result.ok || result.OK, message: result.message || result.Message, projects: [result] });
      showToast(result.message || result.Message || "Smoke-Test abgeschlossen.", !(result.ok || result.OK));
    } catch (error) {
      renderInlineMessage(output, `Smoke-Test fehlgeschlagen: ${error.message}`);
      throw error;
    }
  });
}

function renderSmokeAllResult(target, result) {
  const projects = result.projects || result.Projects || [];
  target.innerHTML = "";
  target.classList.remove("empty", "hidden");
  const title = document.createElement("strong");
  title.textContent = result.message || result.Message || "Smoke-Test abgeschlossen.";
  target.append(title);
  for (const project of projects) {
    target.append(renderSmokeProjectResult(project));
  }
}

function renderSmokeProjectResult(project) {
  const wrapper = document.createElement("div");
  wrapper.className = "smoke-project";
  const title = document.createElement("div");
  title.className = "smoke-project-title";
  title.textContent = `${project.ok || project.OK ? "OK" : "WARN"} - ${project.path || project.Path || project.name || project.Name}`;
  wrapper.append(title);
  const checks = project.checks || project.Checks || [];
  for (const check of checks) {
    const row = document.createElement("div");
    const severity = check.severity || check.Severity || "warn";
    row.className = `smoke-check ${severity}`;
    const name = document.createElement("strong");
    name.textContent = check.name || check.Name || "check";
    const message = document.createElement("span");
    const output = check.output || check.Output || "";
    message.textContent = [check.message || check.Message || "", output ? `(${output})` : ""].filter(Boolean).join(" ");
    row.append(name, message);
    wrapper.append(row);
  }
  return wrapper;
}

async function loadProjectConfig(projectPath) {
  if (!projectPath) {
    elements.projectConfig.classList.add("empty");
    elements.openProjectConfig.classList.add("hidden");
    elements.openProjectConfig.removeAttribute("href");
    elements.projectConfig.textContent = "Keine YAML geladen.";
    return;
  }
  await withBusy(elements.reloadProjectConfig, async () => {
    renderLoading(elements.projectConfig, "YAML wird geladen...");
    try {
      const config = await api(`/api/projects/config?path=${encodeURIComponent(projectPath)}`);
      const configPath = config.path || config.Path || "";
      elements.projectConfig.classList.remove("empty");
      elements.openProjectConfig.classList.toggle("hidden", !configPath);
      if (configPath) {
        elements.openProjectConfig.href = `vscode://file/${encodeURI(configPath)}`;
      }
      elements.projectConfig.textContent = config.content || config.Content || "";
    } catch (error) {
      elements.openProjectConfig.classList.add("hidden");
      renderInlineMessage(elements.projectConfig, `YAML konnte nicht geladen werden: ${error.message}`);
      throw error;
    }
  });
}

function projectStatus(project) {
  const items = [
    projectPlaybookStatus(project),
    projectRootStatus(project),
    projectSetupStatus(project),
    projectRemediationStatus(project),
    projectStructureStatus(project),
    projectDocsStatus(project),
    projectTasksStatus(project),
    projectTodoStatus(project),
    projectReviewsStatus(project),
    projectEnforcementStatus(project),
    projectGitStatus(project),
  ];
  const devcontainer = projectDevcontainerStatus(project);
  if (devcontainer) {
    items.push(devcontainer);
  }
  const recommendations = projectRecommendationsStatus(project);
  if (recommendations) {
    items.push(recommendations);
  }
  return items;
}

function projectRootStatus(project) {
  const root = project.projectRoot || project.ProjectRoot || {};
  const ok = Boolean(root.ok || root.OK);
  const repoRoot = root.repoRoot || root.RepoRoot || "";
  const vcs = root.vcs || root.VCS || "";
  const candidates = root.candidates || root.Candidates || [];
  return {
    key: "project-root",
    label: "Git",
    value: ok ? (vcs === "none" ? "kein Git" : repoRoot) : repoRoot ? "ungueltig" : "fehlt",
    state: ok ? "ok" : "error",
    detail: root.message || root.Message || "Git-/Code-Pfad aus K-PLAYBOOK.yaml.",
    repoRoot,
    vcs,
    candidates,
  };
}

function projectPlaybookStatus(project) {
  const playbook = project.playbook || project.Playbook || {};
  const ok = Boolean(playbook.ok || playbook.OK);
  const found = Boolean(playbook.found || playbook.Found);
  const schemaVersion = playbook.schemaVersion || playbook.SchemaVersion || "";
  const layout = playbook.layout || playbook.Layout || "";
  return {
    key: "playbook",
    label: "Playbook-Konfig",
    value: ok ? `Schema ${schemaVersion || "ok"}` : found ? "unplausibel" : "fehlt",
    state: ok ? "ok" : "error",
    detail: [
      playbook.message || playbook.Message || (ok ? "K-PLAYBOOK.yaml plausibel." : "K-PLAYBOOK.yaml ist nicht plausibel."),
      layout ? `Layout: ${layout}` : "",
    ].filter(Boolean).join(" "),
  };
}

function projectSetupStatus(project) {
  const setup = project.setup || project.Setup || {};
  const ok = Boolean(setup.ok || setup.OK);
  const severity = setup.severity || setup.Severity || "warn";
  const command = setup.command || setup.Command || "/k-setup";
  return {
    key: "setup",
    label: "K-PLAYBOOK.yaml",
    value: ok ? "vorhanden" : "fehlt",
    state: ok ? "ok" : severity === "error" ? "error" : "warn",
    detail: setup.message || setup.Message || (ok ? "Projektkonfiguration ist vorhanden." : "Projektkonfiguration fehlt."),
    command,
  };
}

function projectRemediationStatus(project) {
  const setup = project.setup || project.Setup || {};
  const setupOK = Boolean(setup.ok || setup.OK);
  const remediation = project.remediation || project.Remediation || {};
  const rawMode = remediation.mode || remediation.Mode || "";
  const mode = rawMode || "direct-allowed";
  const ok = Boolean(remediation.ok || remediation.OK);
  const value = ok ? remediationLabel(mode) : rawMode ? `ungueltig (${rawMode})` : "fehlt";
  return {
    key: "remediation",
    label: "Remediation-Policy",
    value,
    state: ok ? "ok" : "warn",
    detail: remediation.message || remediation.Message || "Steuert /k-remediation.",
    mode,
    setupOK,
  };
}

function projectStructureStatus(project) {
  const structure = project.structure || project.Structure;
  const ok = Boolean(structure && (structure.ok || structure.OK));
  const missing = structure ? (structure.missing || structure.Missing || []) : [];
  return {
    key: "structure",
    label: "Projektstruktur",
    value: ok ? "vollstaendig" : "unvollstaendig",
    state: ok ? "ok" : "warn",
    detail: missing.length > 0
      ? `Fehlt: ${compactList(missing)}`
      : (structure && (structure.message || structure.Message)) || "Feste k-playbook-Struktur fehlt teilweise.",
    missing,
  };
}

function projectDocsStatus(project) {
  const docs = project.docs || project.Docs || {};
  const ok = Boolean(docs.ok || docs.OK);
  const command = docs.command || docs.Command || "/k-code2docs";
  return {
    key: "docs",
    label: "Dokumentation",
    value: ok ? "vorhanden" : "fehlt",
    state: ok ? "ok" : "warn",
    detail: docs.message || docs.Message || (ok ? "Projekt-Dokumentation ist vorhanden." : "Projekt-Dokumentation fehlt."),
    command,
  };
}

function projectTasksStatus(project) {
  const tasks = project.tasks || project.Tasks || {};
  const open = numberValue(tasks.open ?? tasks.Open);
  const done = numberValue(tasks.done ?? tasks.Done);
  const next = tasks.next || tasks.Next || "";
  return {
    key: "tasks",
    label: "Tasks",
    value: open > 0 ? `${open} offen` : "keine offen",
    state: open > 0 ? "warn" : "ok",
    detail: [
      tasks.message || tasks.Message || (open > 0 ? `${open} offene Tasks.` : "Keine offenen Tasks."),
      done > 0 ? `${done} erledigt.` : "",
      next ? `Naechster Task: ${next}` : "",
    ].filter(Boolean).join(" "),
  };
}

function projectTodoStatus(project) {
  const todo = project.todo || project.Todo || {};
  const open = numberValue(todo.open ?? todo.Open);
  return {
    key: "todo",
    label: "TODO.md",
    value: open > 0 ? `${open} offen` : "keine offen",
    state: open > 0 ? "warn" : "ok",
    detail: todo.message || todo.Message || (open > 0 ? `${open} offene TODO-Checkboxen.` : "Keine offenen TODO-Checkboxen."),
  };
}

function projectReviewsStatus(project) {
  const reviews = project.reviews || project.Reviews || {};
  const ok = Boolean(reviews.ok || reviews.OK);
  const count = numberValue(reviews.reviews ?? reviews.Reviews);
  const hasLog = Boolean(reviews.hasLog || reviews.HasLog);
  const hasKnownDecisions = Boolean(reviews.hasKnownDecisions || reviews.HasKnownDecisions);
  return {
    key: "reviews",
    label: "Reviews",
    value: ok ? `${count} vorhanden` : "unvollstaendig",
    state: ok ? "ok" : "warn",
    detail: [
      reviews.message || reviews.Message || "Review-Struktur geprueft.",
      hasLog ? "Log vorhanden." : "",
      hasKnownDecisions ? "Known Decisions vorhanden." : "",
    ].filter(Boolean).join(" "),
  };
}

function projectEnforcementStatus(project) {
  const enforcement = project.enforcement || project.Enforcement || {};
  const ok = Boolean(enforcement.ok || enforcement.OK);
  const rules = numberValue(enforcement.rules ?? enforcement.Rules);
  return {
    key: "enforcement",
    label: "Enforcement-Regeln",
    value: rules > 0 ? `${rules} vorhanden` : "keine projektlokalen",
    state: ok ? "ok" : "muted",
    detail: enforcement.message || enforcement.Message || (rules > 0 ? `${rules} Enforcement-Regeln vorhanden.` : "Keine projektlokalen Enforcement-Regeln vorhanden."),
  };
}

function projectGitStatus(project) {
  const git = project.git || project.Git || {};
  const worktree = Boolean(git.worktree || git.Worktree);
  const changed = numberValue(git.changed ?? git.Changed);
  const untracked = numberValue(git.untracked ?? git.Untracked);
  const branch = git.branch || git.Branch || "";
  const clean = worktree && changed === 0 && untracked === 0;
  return {
    key: "git",
    label: "Git",
    value: !worktree ? "kein Worktree" : clean ? "sauber" : `${changed + untracked} offen`,
    state: !worktree ? "muted" : clean ? "ok" : "warn",
    detail: [
      git.message || git.Message || "Git-Status geprueft.",
      branch ? `Branch: ${branch}.` : "",
    ].filter(Boolean).join(" "),
  };
}

function projectRecommendationsStatus(project) {
  const recommendations = project.recommendations || project.Recommendations || [];
  if (recommendations.length === 0) {
    return null;
  }
  return {
    key: "recommendations",
    label: "Empfehlungen",
    value: compactList(recommendations),
    state: "warn",
    detail: `Empfohlen: ${recommendations.join(", ")}`,
  };
}

function projectDevcontainerStatus(project) {
  const environment = project.environment || project.Environment || "unknown";
  if (environment !== "devcontainer") {
    return null;
  }

  const status = project.devcontainer || project.Devcontainer || null;
  const missing = status ? (status.missing || status.Missing || []) : devcontainerMissing(project.path || project.Path);
  const checked = Boolean(status) || missing !== null;
  return {
    key: "devcontainer",
    label: "Playbook im Container",
    value: !checked ? "wird geprueft" : missing.length === 0 ? "erreichbar" : "nicht erreichbar",
    state: !checked ? "muted" : missing.length === 0 ? "ok" : "warn",
    detail: status && (status.message || status.Message)
      ? (status.message || status.Message)
      : !checked ? "Status wird geprueft." : missing.length > 0 ? `Fehlt: ${missing.join(", ")}` : "k-playbook ist im Container erreichbar.",
    checked,
    missing: missing || [],
  };
}

function numberValue(value) {
  return Number.isFinite(Number(value)) ? Number(value) : 0;
}

function projectEditorStatusRow(project, item) {
  if (!isProjectEditablePath(project.path || project.Path)) {
    return projectReadonlyStatusRow(item);
  }
  switch (item.key) {
    case "project-root":
      return projectRootEditorRow(project, item);
    case "setup":
      return projectSetupEditorRow(project, item);
    case "remediation":
      return projectRemediationEditorRow(project, item);
    case "structure":
      return projectStructureEditorRow(project, item);
    case "docs":
      return projectDocsEditorRow(project, item);
    case "devcontainer":
      return projectDevcontainerEditorRow(project, item);
    default:
      return projectReadonlyStatusRow(item);
  }
}

function projectRootEditorRow(project, item) {
  const projectPath = project.path || project.Path;
  const action = document.createElement("div");
  action.className = "project-check-action";
  action.append(statusLabel(item));

  if (item.state === "ok") {
    const change = document.createElement("button");
    change.type = "button";
    change.className = "secondary small-button";
    change.textContent = "Aendern";
    change.addEventListener("click", () => chooseProjectRoot(projectPath, item, change));
    action.append(change);
    return projectStatusRow(item, action);
  }

  const detect = document.createElement("button");
  detect.type = "button";
  detect.className = "primary small-button";
  detect.textContent = "Ermitteln";
  detect.addEventListener("click", () => chooseProjectRoot(projectPath, item, detect));
  const noGit = document.createElement("button");
  noGit.type = "button";
  noGit.className = "secondary small-button";
  noGit.textContent = "Kein Git";
  noGit.addEventListener("click", () => chooseNoGitProjectRoot(projectPath, item, noGit));
  action.append(detect, noGit);
  return projectStatusRow(item, action);
}

function projectReadonlyStatusRow(item) {
  return projectStatusRow(item, statusLabel(item));
}

function projectStatusRow(item, action) {
  const row = document.createElement("div");
  row.className = "project-check-row";

  const text = document.createElement("div");
  const title = document.createElement("span");
  title.className = "project-check-title";
  title.textContent = item.label;
  const detail = document.createElement("span");
  detail.textContent = item.detail;
  text.append(title, detail);

  if (action.classList && action.classList.contains("project-check-action")) {
    row.append(text, action);
  } else {
    const actionWrap = document.createElement("div");
    actionWrap.className = "project-check-action";
    actionWrap.append(action);
    row.append(text, actionWrap);
  }
  return row;
}

function statusLabel(item) {
  const label = document.createElement("span");
  label.className = "status-label " + item.state;
  label.textContent = item.state === "ok" ? "OK ✓" : item.state === "error" ? "FEHLER !" : item.state === "muted" ? "Pruefen..." : "WARN !";
  return label;
}

function projectSetupEditorRow(project, item) {
  if (item.state === "ok") {
    return projectReadonlyStatusRow(item);
  }

  const action = document.createElement("div");
  action.className = "project-check-action";
  action.append(statusLabel(item));
  const help = document.createElement("button");
  help.type = "button";
  help.className = "secondary small-button";
  help.textContent = "Hilfe";
  help.addEventListener("click", () => {
    window.alert([
      "K-PLAYBOOK.yaml fehlt",
      "",
      item.detail,
      "",
      `Projekt: ${project.path || project.Path}`,
      "",
      "Empfohlen: Projekt aus der Installer-Liste entfernen und danach neu hinzufuegen. Neue Einbindungen legen die minimale K-PLAYBOOK.yaml direkt an.",
    ].join("\n"));
  });
  action.append(help);
  return projectStatusRow(item, action);
}

function projectRemediationEditorRow(project, item) {
  if (!item.setupOK) {
    return projectReadonlyStatusRow(item);
  }

  const action = document.createElement("div");
  action.className = "project-check-action";
  const select = remediationSelect(item.mode);
  const help = document.createElement("button");
  help.type = "button";
  help.className = "secondary small-button";
  help.textContent = "Hilfe";
  select.addEventListener("change", () => updateProjectRemediation(project.path || project.Path, select.value, select));
  help.addEventListener("click", () => showRemediationHelp(select.value));
  action.append(select, help);

  return projectStatusRow(item, action);
}

function projectStructureEditorRow(project, item) {
  if (item.state === "ok") {
    return projectReadonlyStatusRow(item);
  }

  const projectPath = project.path || project.Path;
  const action = document.createElement("div");
  action.className = "project-check-action";
  const button = document.createElement("button");
  button.type = "button";
  button.className = "primary";
  button.textContent = "Vervollstaendigen";
  button.addEventListener("click", () => completeProjectStructure(projectPath, button));
  action.append(button);

  return projectStatusRow(item, action);
}

async function completeProjectStructure(projectPath, button) {
  await withBusy(button, async () => {
    const file = await api("/api/projects/structure", {
      method: "POST",
      body: JSON.stringify({ path: projectPath }),
    });
    state.projects = file.projects || file.Projects || [];
    renderProjects(state.projects);
    if (currentProjectPath === projectPath) {
      renderProjectDetailStatus(findProject(projectPath));
      await loadProjectConfig(projectPath);
    }
    showToast("Projektstruktur vervollstaendigt.");
  });
}

async function updateProjectRemediation(projectPath, mode, select) {
  await withBusy(select, async () => {
    const file = await api("/api/projects/remediation", {
      method: "POST",
      body: JSON.stringify({ path: projectPath, mode }),
    });
    state.projects = file.projects || file.Projects || [];
    renderProjects(state.projects);
    if (currentProjectPath === projectPath) {
      renderProjectDetailStatus(findProject(projectPath));
      await loadProjectConfig(projectPath);
    }
    showToast("Remediation-Policy aktualisiert.");
  });
}

async function chooseProjectRoot(projectPath, item, button) {
  await withBusy(button, async () => {
    const candidates = await api(`/api/projects/repo-root-candidates?path=${encodeURIComponent(projectPath)}`);
    const selected = await promptProjectRoot(projectPath, candidates, item);
    if (!selected) {
      return;
    }
    await updateProjectRoot(projectPath, selected.repoRoot, selected.vcs, button);
  });
}

async function chooseNoGitProjectRoot(projectPath, item, button) {
  const answer = window.prompt(
    "Dieses Projekt hat kein Git. Welcher relative Pfad ist der Code-Pfad?",
    item.repoRoot || ".",
  );
  if (answer === null || answer.trim() === "") {
    return;
  }
  await updateProjectRoot(projectPath, answer.trim(), "none", button);
}

function promptProjectRoot(projectPath, candidates, item) {
  const candidateList = Array.isArray(candidates) ? candidates : [];
  if (candidateList.length === 1) {
    const candidate = candidateList[0];
    if (window.confirm(`Gefundenen Git-Pfad eintragen?\n\nProjekt: ${projectPath}\nPfad: ${candidate.repoRoot || candidate.RepoRoot}`)) {
      return Promise.resolve({ repoRoot: candidate.repoRoot || candidate.RepoRoot, vcs: candidate.vcs || candidate.VCS || "git" });
    }
    return Promise.resolve(null);
  }

  const lines = [
    "Git-Pfad relativ zum K-PLAYBOOK.yaml-Ordner eingeben.",
    "",
  ];
  if (candidateList.length > 1) {
    lines.push("Gefundene Git-Repos:");
    candidateList.forEach((candidate, index) => lines.push(`${index + 1}. ${candidate.repoRoot || candidate.RepoRoot}`));
    lines.push("", "Gib eine Nummer oder einen relativen Pfad ein.");
  } else {
    lines.push("Kein Git-Repo automatisch gefunden. Gib einen relativen Pfad ein oder nutze 'Kein Git'.");
  }
  const current = item.repoRoot || ".";
  const answer = window.prompt(lines.join("\n"), current);
  if (answer === null) {
    return Promise.resolve(null);
  }
  const trimmed = answer.trim();
  if (trimmed === "") {
    return Promise.resolve(null);
  }
  const index = Number(trimmed);
  if (Number.isInteger(index) && index >= 1 && index <= candidateList.length) {
    const candidate = candidateList[index - 1];
    return Promise.resolve({ repoRoot: candidate.repoRoot || candidate.RepoRoot, vcs: candidate.vcs || candidate.VCS || "git" });
  }
  return Promise.resolve({ repoRoot: trimmed, vcs: "git" });
}

async function updateProjectRoot(projectPath, repoRoot, vcs, control) {
  await withBusy(control, async () => {
    const file = await api("/api/projects/repo-root", {
      method: "POST",
      body: JSON.stringify({ path: projectPath, repoRoot, vcs }),
    });
    state.projects = file.projects || file.Projects || [];
    renderProjects(state.projects);
    if (currentProjectPath === projectPath) {
      renderProjectDetailStatus(findProject(projectPath));
      await loadProjectConfig(projectPath);
    }
    showToast("Git-Konfiguration aktualisiert.");
  });
}

function removeProjectButton(project) {
  const projectPath = project.path || project.Path;
  const button = document.createElement("button");
  button.type = "button";
  button.className = "secondary danger small-button";
  button.textContent = "Entfernen";
  button.addEventListener("click", (event) => {
    event.stopPropagation();
    removeProject(projectPath, button);
  });
  return button;
}

async function removeProject(projectPath, button) {
  if (!window.confirm(`Projekt aus der Installer-Liste entfernen?\n\n${projectPath}\n\nEs werden keine Projektdateien geloescht.`)) {
    return;
  }

  await withBusy(button, async () => {
    const file = await api("/api/projects", {
      method: "DELETE",
      body: JSON.stringify({ path: projectPath }),
    });
    state.projects = file.projects || file.Projects || [];
    renderProjects(state.projects);
    await loadDevcontainerStatus();
    if (currentProjectPath === projectPath) {
      currentProjectPath = "";
      await loadProjectConfig("");
      showHome({ replaceHistory: true });
    }
    showToast("Projekt aus der Liste entfernt.");
  });
}

function projectDocsEditorRow(project, item) {
  if (item.state === "ok") {
    return projectReadonlyStatusRow(item);
  }

  const projectPath = project.path || project.Path;
  return commandActionRow({
    title: item.label,
    detail: `${item.detail} Im Projekt ${item.command} ausfuehren.`,
    command: item.command,
    helpTitle: "Projekt-Dokumentation erzeugen",
    helpContextLabel: "Oeffne den Assistenten/OpenCode im Projekt",
    helpContext: projectPath,
    helpText: `${item.command} erzeugt bzw. aktualisiert projektlokale Dokumentation. Wenn zuerst die vorhandenen Projekt-Tools inventarisiert werden sollen, nutze vorher /k-tools-scan.`,
  });
}

function projectDevcontainerEditorRow(project, item) {
  if (item.state === "ok" || !item.checked) {
    return projectReadonlyStatusRow(item);
  }

  const projectPath = project.path || project.Path;
  const action = document.createElement("div");
  action.className = "project-check-action";
  action.append(statusLabel(item));
  const button = document.createElement("button");
  button.type = "button";
  button.className = "primary";
  button.textContent = "Eintrag setzen";
  button.addEventListener("click", () => installDevcontainer(projectPath, button));
  action.append(button);

  return projectStatusRow(item, action);
}

function commandActionRow({ title, detail, command, helpTitle, helpContextLabel, helpContext, helpText, className = "project-check-row" }) {
  const row = document.createElement("div");
  row.className = className;

  const text = document.createElement("div");
  const titleElement = document.createElement("span");
  titleElement.className = "project-check-title";
  titleElement.textContent = title;
  const detailElement = document.createElement("span");
  detailElement.textContent = detail;
  text.append(titleElement, detailElement);

  const action = document.createElement("div");
  action.className = "project-check-action";
  const copy = document.createElement("button");
  copy.type = "button";
  copy.className = "icon-button";
  copy.textContent = command;
  copy.title = `${command} in die Zwischenablage kopieren`;
  copy.addEventListener("click", () => copyText(command));
  action.append(copy);

  const help = document.createElement("button");
  help.type = "button";
  help.className = "secondary";
  help.textContent = "Hilfe";
  help.addEventListener("click", () => showCommandHelp({ title: helpTitle || title, command, contextLabel: helpContextLabel, context: helpContext, text: helpText }));
  action.append(help);

  row.append(text, action);
  return row;
}

async function copyText(value) {
  try {
    await navigator.clipboard.writeText(value);
    showToast(`${value} kopiert.`);
  } catch {
    showToast(`Kopieren fehlgeschlagen. Befehl: ${value}`, true);
  }
}

function showCommandHelp({ title, command, contextLabel, context, text }) {
  const lines = [title, ""];
  if (contextLabel && context) {
    lines.push(`1. ${contextLabel}:`, context, "");
    lines.push("2. Fuehre dort aus:", command);
  } else {
    lines.push("Fuehre aus:", command);
  }
  if (text) {
    lines.push("", text);
  }
  window.alert(lines.join("\n"));
}

function devcontainerMissing(projectPath) {
  const status = state.devcontainers;
  if (!status) {
    return null;
  }
  const missing = status.missing || status.Missing || [];
  const project = missing.find((entry) => (entry.path || entry.Path) === projectPath);
  if (!project) {
    return [];
  }
  return project.missing || project.Missing || [];
}

function renderOpenCode(status) {
  const ok = Boolean(status.ok || status.OK);
  const buttons = [elements.opencodeInstall, elements.opencodeInstallTop];
  elements.opencodePill.textContent = ok ? "OK ✓" : "WARN !";
  elements.opencodePill.className = "status-label " + (ok ? "ok" : "warn");
  for (const button of buttons) {
    button.disabled = ok;
    button.classList.toggle("attention-highlight", !ok);
  }
  elements.opencodeInstallTop.classList.toggle("hidden", ok);

  const rows = [
    ["OpenCode Commands", `${status.linkedCommands || 0}/${status.repoCommands || 0} verlinkt`],
    ["OpenCode Skills", status.skillsPathOk ? "skills.paths ok" : "skills.paths fehlt"],
    ["Claude Commands", `${status.claudeLinkedCommands || 0}/${status.repoCommands || 0} verlinkt`],
    ["Claude Skills", `${status.claudeLinkedSkills || 0}/${status.repoSkills || 0} verlinkt`],
    ["Config", status.configExists ? status.configFile : `${status.configFile} wird angelegt`],
  ];
  if ((status.missingCommands || []).length > 0) {
    rows.push(["Fehlende Commands", compactList(status.missingCommands)]);
  }
  if ((status.wrongCommands || []).length > 0) {
    rows.push(["Falsche Links", compactList(status.wrongCommands)]);
  }
  if ((status.nonSymlinkCommands || []).length > 0) {
    rows.push(["Nicht-Symlinks", compactList(status.nonSymlinkCommands)]);
  }
  if ((status.staleCommands || []).length > 0) {
    rows.push(["OpenCode verwaist", compactList(status.staleCommands)]);
  }
  if ((status.claudeMissingCommands || []).length > 0) {
    rows.push(["Claude Commands fehlen", compactList(status.claudeMissingCommands)]);
  }
  if ((status.claudeMissingSkills || []).length > 0) {
    rows.push(["Claude Skills fehlen", compactList(status.claudeMissingSkills)]);
  }
  if ((status.claudeStaleCommands || []).length > 0) {
    rows.push(["Claude Commands verwaist", compactList(status.claudeStaleCommands)]);
  }
  if ((status.claudeStaleSkills || []).length > 0) {
    rows.push(["Claude Skills verwaist", compactList(status.claudeStaleSkills)]);
  }
  if (!status.configEditable && !status.skillsPathOk) {
    rows.push(["Manuell noetig", "OpenCode-Konfig enthaelt bereits skills und wird nicht automatisch veraendert."]);
  }
  if (status.restartRequired) {
    rows.push(["Wichtig", "Betroffene Assistenten neu starten."]);
  }

  elements.opencodeSummary.innerHTML = "";
  elements.opencodeSummary.classList.remove("empty");
  for (const [label, value] of rows) {
    const row = document.createElement("div");
    row.className = "summary-item";
    const key = document.createElement("strong");
    key.textContent = label;
    const text = document.createElement("span");
    text.textContent = value;
    row.append(key, text);
    elements.opencodeSummary.append(row);
  }
}

function renderOpenCodeLoading() {
  elements.opencodePill.textContent = "Pruefen...";
  elements.opencodePill.className = "status-label muted";
  elements.opencodeInstallTop.classList.add("hidden");
  renderLoading(elements.opencodeSummary, "Assistenten-Registrierung wird geprueft...");
}

function renderSecurityTools(status) {
  const ok = status.ok || status.OK;
  const scopeOk = status.scopeOk !== false && status.ScopeOK !== false;
  const missingRequired = status.missingRequired ?? status.MissingRequired ?? 0;
  const tools = status.tools || status.Tools || [];

  elements.securityToolsPill.textContent = ok ? "OK ✓" : scopeOk ? "WARN !" : "SCOPE !";
  elements.securityToolsPill.className = "status-label " + (ok ? "ok" : "warn");
  elements.securityToolsSummary.innerHTML = "";
  elements.securityToolsSummary.classList.remove("empty");

  const summary = document.createElement("div");
  summary.className = "security-summary";
  const summaryText = document.createElement("p");
  summaryText.className = "message";
  summaryText.textContent = status.message || status.Message || (ok ? "Alle Pflicht-Tools sind vorhanden." : `${missingRequired} Pflicht-Tools fehlen.`);
  summary.append(summaryText);

  const toolMatrix = status.toolMatrix || status.ToolMatrix || "";
  if (toolMatrix) {
    const matrix = document.createElement("p");
    matrix.className = "message subtle";
    matrix.textContent = `Tool-Matrix: ${toolMatrix}`;
    summary.append(matrix);
  }

  if (!scopeOk) {
    const scope = document.createElement("p");
    scope.className = "message warn-text";
    const virtualEnv = status.virtualEnv || status.VirtualEnv || "";
    const warnings = status.pathWarnings || status.PathWarnings || [];
    scope.textContent = [
      virtualEnv ? `VIRTUAL_ENV: ${virtualEnv}` : "",
      warnings.length > 0 ? `PATH enthaelt: ${warnings.join(", ")}` : "",
    ].filter(Boolean).join(" | ");
    summary.append(scope);
  }
  elements.securityToolsSummary.append(summary);

  if (missingRequired > 0) {
    elements.securityToolsSummary.append(commandActionRow({
      title: "Fehlende Security-Tools installieren",
      detail: "Installation separat im Assistenten oder Terminal starten. Die Installer-GUI installiert bewusst nichts selbst.",
      command: "/k-install-security-tools --install missing",
      helpTitle: "Security-Tools installieren",
      helpText: "Empfohlen ist der Slash-Command in OpenCode: /k-install-security-tools --install missing. Ohne OpenCode kann derselbe Installer direkt im Terminal gestartet werden: bash ~/dev/k-playbook/scripts/install-security-tools.sh --install missing --method auto",
      className: "command-action-row",
    }));
  }

  const list = document.createElement("div");
  list.className = "security-tool-list";
  for (const tool of tools) {
    const row = document.createElement("div");
    row.className = "security-tool-row";

    const info = document.createElement("div");
    const name = document.createElement("strong");
    name.textContent = tool.name || tool.Name;
    const detail = document.createElement("span");
    const role = tool.role || tool.Role || "";
    const path = tool.path || tool.Path || "";
    const version = tool.version || tool.Version || "";
    const dockerImage = tool.dockerImage || tool.DockerImage || "";
    const parts = [role];
    if (path) {
      parts.push(version, path);
    }
    if (dockerImage && dockerImage !== "-") {
      parts.push(`Docker: ${dockerImage}`);
    }
    detail.textContent = parts.filter(Boolean).join(" - ");
    info.append(name, detail);

    const present = tool.present || tool.Present;
    const label = document.createElement("span");
    label.className = "status-label " + (present ? "ok" : (tool.required || tool.Required) ? "warn" : "muted");
    label.textContent = present ? "OK ✓" : (tool.required || tool.Required) ? "FEHLT !" : "OPTIONAL";

    row.append(info, label);
    list.append(row);
  }
  elements.securityToolsSummary.append(list);
}

function renderSecurityToolsLoading() {
  elements.securityToolsPill.textContent = "Pruefen...";
  elements.securityToolsPill.className = "status-label muted";
  renderLoading(elements.securityToolsSummary, "Security-Tools werden geprueft...");
}

function compactList(values) {
  if (values.length <= 4) {
    return values.join(", ");
  }
  return `${values.slice(0, 4).join(", ")} +${values.length - 4} weitere`;
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
    button.classList.toggle("active", doc.path === state.currentDocPath);
    button.addEventListener("click", async () => {
      for (const active of document.querySelectorAll(".doc-link.active")) {
        active.classList.remove("active");
      }
      button.classList.add("active");
      await loadDoc(doc.path, doc.title || doc.path);
    });
    elements.docsList.append(button);
  }
}

async function loadDoc(path, title = "") {
  openDocOverlay(title || path, path);
  renderLoading(elements.docViewer, "Dokument wird geladen...");
  try {
    const doc = await api(`/api/docs/file?path=${encodeURIComponent(path)}`);
    state.currentDocPath = doc.path || path;
    elements.docTitle.textContent = doc.title || title || path;
    elements.docPath.textContent = doc.path || path;
    elements.docViewer.classList.remove("empty");
    elements.docViewer.innerHTML = doc.html || "";
  } catch (error) {
    renderInlineMessage(elements.docViewer, `Dokument konnte nicht geladen werden: ${error.message}`);
    throw error;
  }
}

function openDocOverlay(title, path) {
  elements.docTitle.textContent = title;
  elements.docPath.textContent = path;
  elements.docOverlay.classList.remove("hidden");
  document.body.classList.add("doc-overlay-open");
  elements.closeDoc.focus({ preventScroll: true });
}

function closeDocOverlay() {
  elements.docOverlay.classList.add("hidden");
  document.body.classList.remove("doc-overlay-open");
}

function detectedLabel(project) {
  const environment = project.environment || project.Environment || "unknown";
  const detected = project.detected || project.Detected || [];
  const label = environmentLabel(environment);
  return detected.length > 0 ? `${label} - ${detected.join(", ")}` : label;
}

function environmentSelect(value, requireChoice = false) {
  const select = document.createElement("select");
  const options = requireChoice ? [
    ["", "Unbekannt - bitte auswaehlen"],
    ["plain", "Normal"],
    ["devcontainer", "DevContainer"],
  ] : [
    ["", "Automatisch erkennen"],
    ["plain", "Normal"],
    ["devcontainer", "DevContainer"],
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

function remediationSelect(value = "direct-allowed") {
  const select = document.createElement("select");
  populateRemediationSelect(select, value);
  return select;
}

function populateRemediationSelect(select, value = "direct-allowed") {
  if (!select) {
    return;
  }
  select.innerHTML = "";
  for (const [optionValue, label] of remediationOptions()) {
    const option = document.createElement("option");
    option.value = optionValue;
    option.textContent = label;
    option.selected = optionValue === value;
    select.append(option);
  }
}

function remediationOptions() {
  return [
    ["direct-allowed", "Direkt erlaubt"],
    ["task-first", "Erst Tasks"],
    ["task-branch-pr", "Task + Branch/PR"],
  ];
}

function remediationLabel(value) {
  const option = remediationOptions().find(([optionValue]) => optionValue === value);
  return option ? option[1] : value;
}

function showRemediationHelp(mode) {
  const help = {
    "direct-allowed": "Kleine sichere Sofort-Fixes duerfen nach Code-Sichtung direkt umgesetzt werden. Groessere Punkte werden Tasks. Default fuer schnelle Einzelprojekte.",
    "task-first": "Findings werden zuerst als Tasks/Buendel geplant. Direkte Fixes nur nach expliziter Einzelfreigabe.",
    "task-branch-pr": "Strengster Modus: keine direkten Fixes aus /k-remediation. Es entstehen Tasks mit Branch-/PR-Hinweis.",
  };
  window.alert(help[mode] || help["direct-allowed"]);
}

function environmentLabel(value) {
  switch (value) {
    case "plain":
    case "venv":
      return "Normal";
    case "devcontainer":
      return "DevContainer";
    default:
      return "Unbekannt";
  }
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

async function withBusyMany(buttons, callback) {
  for (const button of buttons) {
    button.disabled = true;
  }
  try {
    await callback();
  } catch (error) {
    showToast(error.message, true);
  } finally {
    for (const button of buttons) {
      button.disabled = false;
    }
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
