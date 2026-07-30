const state = {
  status: null,
  scanned: [],
  devcontainers: null,
  securityTools: null,
  gitStatus: null,
  projects: [],
};

const elements = {
  refresh: document.querySelector("#refresh"),
  shutdown: document.querySelector("#shutdown"),
  backHome: document.querySelector("#back-home"),
  openScan: document.querySelector("#open-scan"),
  cancelScan: document.querySelector("#cancel-scan"),
  gitPullTop: document.querySelector("#git-pull-top"),
  gitPull: document.querySelector("#git-pull"),
  gitOutput: document.querySelector("#git-output"),
  opencodeInstall: document.querySelector("#opencode-install"),
  opencodeRefresh: document.querySelector("#opencode-refresh"),
  opencodePill: document.querySelector("#opencode-pill"),
  opencodeSummary: document.querySelector("#opencode-summary"),
  securityToolsRefresh: document.querySelector("#security-tools-refresh"),
  securityToolsPill: document.querySelector("#security-tools-pill"),
  securityToolsSummary: document.querySelector("#security-tools-summary"),
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

elements.refresh.addEventListener("click", refreshAll);
elements.shutdown.addEventListener("click", shutdownInstaller);
elements.backHome.addEventListener("click", showHome);
elements.openScan.addEventListener("click", showScan);
elements.cancelScan.addEventListener("click", showHome);
elements.gitPullTop.addEventListener("click", gitPull);
elements.gitPull.addEventListener("click", gitPull);
elements.opencodeInstall.addEventListener("click", installOpenCode);
elements.opencodeRefresh.addEventListener("click", loadOpenCodeStatus);
elements.securityToolsRefresh.addEventListener("click", loadSecurityToolsStatus);
elements.reloadDocs.addEventListener("click", loadDocs);
elements.repair.addEventListener("click", repairPath);
elements.scan.addEventListener("click", scanProjects);
elements.manualForm.addEventListener("submit", addManualProject);

refreshAll();
showHome();
startHealthChecks();
window.addEventListener("pagehide", notifyClientGone);

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
    await loadGitStatus();
    await refreshProjects();
    await loadDevcontainerStatus();
    await loadOpenCodeStatus();
    await loadSecurityToolsStatus();
    await loadDocs();
  }
}

async function gitPull() {
  await withBusyMany([elements.gitPull, elements.gitPullTop], async () => {
    elements.gitOutput.textContent = "git pull laeuft...";
    const result = await api("/api/git/pull", { method: "POST" });
    elements.gitOutput.textContent = result.output || "Bereits aktuell.";
    showToast("Repository aktualisiert.");
    await refreshAll();
  });
}

async function loadGitStatus() {
  try {
    const status = await api("/api/git/status");
    state.gitStatus = status;
    renderGitStatus(status);
  } catch (error) {
    state.gitStatus = null;
    renderGitStatus(null, error.message);
  }
}

function renderGitStatus(status, error = "") {
  const updateAvailable = Boolean(status && (status.updateAvailable || status.UpdateAvailable));
  const buttons = [
    [elements.gitPullTop, "k-playbook aktualisieren"],
    [elements.gitPull, "Git pull"],
  ];
  for (const [button, defaultLabel] of buttons) {
    button.classList.toggle("primary", updateAvailable);
    button.classList.toggle("secondary", !updateAvailable);
    button.classList.toggle("update-available", updateAvailable);
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
    const docs = await api("/api/docs");
    renderDocsList(docs);
  });
}

async function loadOpenCodeStatus() {
  await withBusy(elements.opencodeRefresh, async () => {
    const status = await api("/api/opencode/status");
    renderOpenCode(status);
  });
}

async function loadSecurityToolsStatus() {
  await withBusy(elements.securityToolsRefresh, async () => {
    const status = await api("/api/security-tools/status");
    state.securityTools = status;
    renderSecurityTools(status);
  });
}

async function installOpenCode() {
  await withBusy(elements.opencodeInstall, async () => {
    const result = await api("/api/opencode/install", { method: "POST" });
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
    if (status.OK) {
      showToast("Pfadvertrag repariert.");
      await refreshProjects();
      await loadDevcontainerStatus();
    }
  });
}

async function refreshProjects() {
  const file = await api("/api/projects");
  state.projects = file.projects || file.Projects || [];
  renderProjects(state.projects);
}

async function scanProjects() {
  await withBusy(elements.scan, async () => {
    const projects = await api(`/api/projects/scan?root=${encodeURIComponent(elements.scanRoot.value)}`);
    state.scanned = projects;
    renderScanned(projects);
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
    showHome();
  });
}

async function saveProject(path, environment) {
  return api("/api/projects", {
    method: "POST",
    body: JSON.stringify({ path, environment, selected: true }),
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
    showHome();
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
    const card = document.createElement("div");
    card.className = "project-row";

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
    headerActions.append(environment, removeProjectButton(project));
    header.append(details, headerActions);
    card.append(header);

    const devcontainerRow = projectDevcontainerRow(project);
    if (devcontainerRow) {
      card.append(devcontainerRow);
    }
    const setupRow = projectSetupRow(project);
    if (setupRow) {
      card.append(setupRow);
    }
    const remediationRow = projectRemediationRow(project);
    if (remediationRow) {
      card.append(remediationRow);
    }
    const structureRow = projectStructureRow(project);
    if (structureRow) {
      card.append(structureRow);
    }
    const docsRow = projectDocsRow(project);
    if (docsRow) {
      card.append(docsRow);
    }
    elements.projects.append(card);
  }
}

function projectStructureRow(project) {
  const structure = project.structure || project.Structure;
  if (!structure || structure.ok || structure.OK) {
    return null;
  }

  const projectPath = project.path || project.Path;
  const missing = structure.missing || structure.Missing || [];
  const row = document.createElement("div");
  row.className = "project-check-row";

  const text = document.createElement("div");
  const title = document.createElement("span");
  title.className = "project-check-title";
  title.textContent = "Projektstruktur unvollstaendig";
  const detail = document.createElement("span");
  detail.textContent = missing.length > 0
    ? `Fehlt: ${compactList(missing)}`
    : (structure.message || structure.Message || "Feste k-playbook-Struktur fehlt teilweise.");
  text.append(title, detail);

  const action = document.createElement("div");
  action.className = "project-check-action";
  const button = document.createElement("button");
  button.type = "button";
  button.className = "primary";
  button.textContent = "Vervollstaendigen";
  button.addEventListener("click", () => completeProjectStructure(projectPath, button));
  action.append(button);

  row.append(text, action);
  return row;
}

async function completeProjectStructure(projectPath, button) {
  await withBusy(button, async () => {
    const file = await api("/api/projects/structure", {
      method: "POST",
      body: JSON.stringify({ path: projectPath }),
    });
    state.projects = file.projects || file.Projects || [];
    renderProjects(state.projects);
    showToast("Projektstruktur vervollstaendigt.");
  });
}

function projectRemediationRow(project) {
  const setup = project.setup || project.Setup;
  if (!setup || !(setup.ok || setup.OK)) {
    return null;
  }

  const remediation = project.remediation || project.Remediation || {};
  const mode = remediation.mode || remediation.Mode || "direct-allowed";
  const row = document.createElement("div");
  row.className = "project-check-row";

  const text = document.createElement("div");
  const title = document.createElement("span");
  title.className = "project-check-title";
  title.textContent = "Remediation-Policy";
  const detail = document.createElement("span");
  detail.textContent = remediation.message || remediation.Message || "Steuert /k-remediation.";
  text.append(title, detail);

  const action = document.createElement("div");
  action.className = "project-check-action";
  const select = remediationSelect(mode);
  const help = document.createElement("button");
  help.type = "button";
  help.className = "secondary small-button";
  help.textContent = "?";
  help.title = "Remediation-Policy erklaeren";
  select.addEventListener("change", () => updateProjectRemediation(project.path || project.Path, select.value, select));
  help.addEventListener("click", () => showRemediationHelp(select.value));
  action.append(select, help);

  row.append(text, action);
  return row;
}

async function updateProjectRemediation(projectPath, mode, select) {
  await withBusy(select, async () => {
    const file = await api("/api/projects/remediation", {
      method: "POST",
      body: JSON.stringify({ path: projectPath, mode }),
    });
    state.projects = file.projects || file.Projects || [];
    renderProjects(state.projects);
    showToast("Remediation-Policy aktualisiert.");
  });
}

function removeProjectButton(project) {
  const projectPath = project.path || project.Path;
  const button = document.createElement("button");
  button.type = "button";
  button.className = "secondary danger small-button";
  button.textContent = "Entfernen";
  button.addEventListener("click", () => removeProject(projectPath, button));
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
    showToast("Projekt aus der Liste entfernt.");
  });
}

function projectSetupRow(project) {
  const setup = project.setup || project.Setup;
  if (!setup || setup.ok || setup.OK) {
    return null;
  }

  const projectPath = project.path || project.Path;
  const command = setup.command || setup.Command || "/k-setup";
  const severity = setup.severity || setup.Severity || "warn";
  if (severity === "error") {
    return projectSetupErrorRow(setup);
  }
  return commandActionRow({
    title: "Projekt-Setup fehlt",
    detail: `${setup.message || setup.Message || "K-PLAYBOOK.yaml fehlt."} Im Projekt den Assistenten oeffnen und ${command} ausfuehren.`,
    command,
    helpTitle: "Projekt-Setup ausfuehren",
    helpContextLabel: "Oeffne den Assistenten/OpenCode im Projekt",
    helpContext: projectPath,
    helpText: `Der Installer startet ${command} nicht selbst, weil der Slash-Command im Zielprojekt-Kontext Rueckfragen stellt und Dateien dort anlegt.`,
  });
}

function projectSetupErrorRow(setup) {
  const row = document.createElement("div");
  row.className = "project-check-row";

  const text = document.createElement("div");
  const title = document.createElement("span");
  title.className = "project-check-title";
  title.textContent = "Fehler: K-PLAYBOOK.yaml fehlt";
  const detail = document.createElement("span");
  detail.textContent = setup.message || setup.Message || "Projekt aus der Installer-Liste entfernen und neu einbinden.";
  text.append(title, detail);

  const action = document.createElement("div");
  action.className = "project-check-action";
  const label = document.createElement("span");
  label.className = "status-label error";
  label.textContent = "FEHLER !";
  action.append(label);

  row.append(text, action);
  return row;
}

function projectDocsRow(project) {
  const docs = project.docs || project.Docs;
  if (!docs || docs.ok || docs.OK) {
    return null;
  }

  const projectPath = project.path || project.Path;
  const command = docs.command || docs.Command || "/k-code2docs";
  return commandActionRow({
    title: "Dokumentation fehlt",
    detail: `${docs.message || docs.Message || "docs-Verzeichnis ist leer."} Im Projekt ${command} ausfuehren.`,
    command,
    helpTitle: "Projekt-Dokumentation erzeugen",
    helpContextLabel: "Oeffne den Assistenten/OpenCode im Projekt",
    helpContext: projectPath,
    helpText: `${command} erzeugt bzw. aktualisiert projektlokale Dokumentation. Wenn zuerst die vorhandenen Projekt-Tools inventarisiert werden sollen, nutze vorher /k-tools-scan.`,
  });
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

function projectDevcontainerRow(project) {
  const environment = project.environment || project.Environment || "unknown";
  if (environment !== "devcontainer") {
    return null;
  }

  const projectPath = project.path || project.Path;
  const missing = devcontainerMissing(projectPath);
  const checked = missing !== null;
  const row = document.createElement("div");
  row.className = "project-check-row";

  const text = document.createElement("div");
  const title = document.createElement("span");
  title.className = "project-check-title";
  title.textContent = "k-playbook im Container erreichbar";
  if (!checked) {
    const detail = document.createElement("span");
    detail.textContent = "Status wird geprueft.";
    text.append(title, detail);
  } else if (missing.length > 0) {
    const detail = document.createElement("span");
    detail.textContent = `Fehlt: ${missing.join(", ")}`;
    text.append(title, detail);
  } else {
    text.append(title);
  }

  const action = document.createElement("div");
  action.className = "project-check-action";
  const stateLabel = document.createElement("span");
  stateLabel.className = "status-label " + (!checked ? "muted" : missing.length === 0 ? "ok" : "warn");
  stateLabel.textContent = !checked ? "Pruefen..." : missing.length === 0 ? "OK ✓" : "WARN !";
  action.append(stateLabel);

  if (checked && missing.length > 0) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "primary";
    button.textContent = "Eintrag setzen";
    button.addEventListener("click", () => installDevcontainer(projectPath, button));
    action.append(button);
  }

  row.append(text, action);
  return row;
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
  elements.opencodePill.textContent = status.ok ? "OK ✓" : "WARN !";
  elements.opencodePill.className = "status-label " + (status.ok ? "ok" : "warn");
  elements.opencodeInstall.disabled = status.ok;

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
    detail.textContent = path ? `${role} - ${version} - ${path}` : role;
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
