"use strict";

// Seite "Branches": Umgebungen, Branches des Code-Repos, Worktrees.
//
// Eine Antwort von /api/branches füllt alle drei Karten. Ungefragt holt die
// Seite nichts vom Remote; „Aktualisieren“ ist ein eigener POST. Umgeschaltet
// wird in drei Schritten: „Umschalten prüfen“ zeigt jeden Prüfpunkt, nur ohne
// Blockierendes erscheint „Umschalten auf …“, und erst die Bestätigung im
// Dialog schickt den POST — der Server prüft dann noch einmal.

// Muss vor den Ladefunktionen laufen: die blenden Blöcke ein, und das Menü
// zieht das nur mit, wenn es die Karten schon beobachtet.
buildBlockNav();

startSession((message) => {
  document.getElementById("br-list-message").textContent = message;
});

const branchView = {
  envPill: document.getElementById("br-env-pill"),
  envList: document.getElementById("br-env-list"),
  envMessage: document.getElementById("br-env-message"),
  listPill: document.getElementById("br-list-pill"),
  facts: document.getElementById("br-facts"),
  policy: document.getElementById("br-switch-policy"),
  fetchButton: document.getElementById("br-fetch"),
  fetchProgress: document.getElementById("br-fetch-progress"),
  fetchMessage: document.getElementById("br-fetch-message"),
  groups: document.getElementById("br-groups"),
  listMessage: document.getElementById("br-list-message"),
  worktreePill: document.getElementById("br-worktree-pill"),
  worktrees: document.getElementById("br-worktrees"),
  dialog: document.getElementById("br-confirm"),
  dialogSource: document.getElementById("br-confirm-source"),
  dialogTarget: document.getElementById("br-confirm-target"),
  dialogCommand: document.getElementById("br-confirm-command"),
  dialogHints: document.getElementById("br-confirm-hints"),
  dialogCancel: document.getElementById("br-confirm-cancel"),
  dialogRun: document.getElementById("br-confirm-run"),
};

// Die Prüfung, deren Bestätigung gerade im Dialog steht.
let pendingSwitch = null;

// Ab dieser Zahl klappt eine Gruppe von Arbeitsbranches zu.
const BRANCH_FOLD_FROM = 6;

const CHECK_TONE = { ok: "ok", hinweis: "warn", blockiert: "error" };
const CHECK_LABEL = { ok: "ok", hinweis: "Hinweis", blockiert: "blockiert", "nicht-pruefbar": "nicht prüfbar" };
const SOURCE_LABEL = {
  config: "festgelegt",
  deployment: "Vorschlag aus Deployment",
  name: "Vorschlag aus Namen",
};

branchView.fetchButton.addEventListener("click", runFetch);
branchView.dialogCancel.addEventListener("click", () => branchView.dialog.close());
branchView.dialogRun.addEventListener("click", runSwitch);

loadBranches();

function branchNode(tag, className, text) {
  const node = document.createElement(tag);
  if (className) {
    node.className = className;
  }
  if (text !== undefined) {
    node.textContent = text;
  }
  return node;
}

function branchPill(tone, text) {
  return branchNode("span", `pill ${tone}`, text);
}

function setBranchPill(pill, tone, text) {
  pill.className = `pill ${tone}`;
  pill.textContent = text;
}

function branchFact(term, detail) {
  const row = document.createElement("div");
  const dt = branchNode("dt", "", term);
  const dd = document.createElement("dd");
  if (typeof detail === "string") {
    dd.textContent = detail;
  } else {
    dd.append(detail);
  }
  row.append(dt, dd);
  branchView.facts.append(row);
}

function shortSha(sha) {
  return (sha || "").slice(0, 7);
}

function fetchAge(fetch) {
  if (!fetch || fetch.state === "never") {
    return (fetch && fetch.message) || "noch nie";
  }
  if (fetch.state !== "ok") {
    return fetch.message || "unbekannt";
  }
  const seconds = fetch.ageSeconds || 0;
  let age;
  if (seconds < 90) {
    age = "gerade eben";
  } else if (seconds < 5400) {
    age = `vor ${Math.round(seconds / 60)} Minuten`;
  } else if (seconds < 129600) {
    age = `vor ${Math.round(seconds / 3600)} Stunden`;
  } else {
    age = `vor ${Math.round(seconds / 86400)} Tagen`;
  }
  return `${age} (${new Date(fetch.at).toLocaleString("de-DE")})`;
}

async function loadBranches() {
  let data;
  try {
    const response = await fetch("/api/branches", { cache: "no-store" });
    data = await response.json();
  } catch {
    for (const pill of [branchView.envPill, branchView.listPill, branchView.worktreePill]) {
      setBranchPill(pill, "error", "Fehler");
    }
    branchView.listMessage.textContent = "Der Stand konnte nicht geladen werden.";
    return;
  }
  if (data.state !== "ok") {
    const tone = data.state === "no-project" || data.state === "no-vcs" ? "muted" : "error";
    for (const pill of [branchView.envPill, branchView.listPill, branchView.worktreePill]) {
      setBranchPill(pill, tone, "Nicht verfügbar");
    }
    branchView.groups.replaceChildren();
    branchView.envList.replaceChildren();
    branchView.worktrees.replaceChildren();
    branchView.listMessage.textContent = data.message || "Unbekannter Zustand.";
    return;
  }
  branchView.listMessage.textContent = "";
  renderEnvironments(data);
  renderFacts(data);
  renderGroups(data);
  renderWorktrees(data);
  branchView.fetchButton.disabled = false;
}

function renderEnvironments(data) {
  const environments = data.environments || [];
  branchView.envList.replaceChildren();
  branchView.envMessage.textContent = "";
  const gh = data.github || {};
  if (!gh.enabled) {
    branchView.envMessage.textContent = "Ohne gh: nur Festlegungen und Vorschläge aus Namen. " + (gh.message || "");
  } else if (gh.state !== "ok") {
    branchView.envMessage.textContent = "GitHub-Daten fehlen, Liste und Umgebungen zeigen nur den lokalen Stand: " + (gh.message || "");
  } else {
    // Eine Teilabfrage kann allein scheitern, etwa an der Zeitgrenze; dann
    // fehlen nur ihre Daten, und der Satz sagt, welche.
    const notes = gh.notes || {};
    const labels = { pulls: "Pull Requests", environments: "Environments", deployments: "Deployments" };
    const failed = Object.keys(labels).filter((key) => notes[key] && notes[key].state && notes[key].state !== "ok");
    if (failed.length > 0) {
      branchView.envMessage.textContent = "GitHub-Daten fehlen: " + failed.map((key) => `${labels[key]}: ${notes[key].message}`).join(" ");
    }
  }
  if (environments.length === 0) {
    branchView.envList.classList.add("empty");
    branchView.envList.textContent = "Keine Umgebung festgelegt, und kein Branch-Name legt eine nahe.";
    setBranchPill(branchView.envPill, "muted", "Keine");
    return;
  }
  branchView.envList.classList.remove("empty");
  let findings = 0;
  let suggested = 0;
  for (const env of environments) {
    const item = branchNode("div", "branch-item");
    const head = branchNode("div", "branch-head");
    head.append(branchNode("span", "branch-name", env.name));
    if (env.branch) {
      head.append(branchNode("span", "branch-meta", "←"), branchNode("span", "branch-name", env.branch));
      head.append(branchPill(env.suggested ? "warn" : "ok", SOURCE_LABEL[env.source] || env.source));
      if (env.suggested) {
        suggested += 1;
      }
    } else {
      head.append(branchPill("muted", "keine Zuordnung"));
    }
    if (env.onGitHub) {
      head.append(branchNode("span", "branch-meta", "GitHub-Environment"));
    }
    item.append(head);
    if (env.message) {
      item.append(branchNode("p", "branch-meta", env.message));
    }
    const hints = [];
    if (env.nameSuggestion && env.nameSuggestion !== env.branch) {
      hints.push(`Namensvorschlag: ${env.nameSuggestion}`);
    }
    if (env.deploymentSuggestion && env.deploymentSuggestion !== env.branch) {
      hints.push(`Deployment spricht für: ${env.deploymentSuggestion}`);
    }
    if (hints.length > 0) {
      item.append(branchNode("p", "branch-meta", hints.join(" · ")));
    }
    if (env.deployment) {
      const deployment = env.deployment;
      const parts = [`Letztes Deployment: ${deployment.ref || shortSha(deployment.sha)} (${shortSha(deployment.sha)})`];
      if (deployment.createdAt) {
        parts.push(new Date(deployment.createdAt).toLocaleString("de-DE"));
      }
      if (deployment.subject) {
        parts.push(deployment.subject);
      }
      item.append(branchNode("p", "branch-meta", parts.join(" · ")));
      if (deployment.message) {
        item.append(branchNode("p", "branch-meta", deployment.message));
      }
    }
    for (const finding of env.findings || []) {
      findings += 1;
      item.append(branchNode("p", "hint warn branch-note", "Befund: " + finding.message));
    }
    branchView.envList.append(item);
  }
  if (findings > 0) {
    setBranchPill(branchView.envPill, "warn", findings === 1 ? "1 Abweichung" : `${findings} Abweichungen`);
  } else if (suggested > 0) {
    setBranchPill(branchView.envPill, "warn", "Vorschlag");
  } else {
    setBranchPill(branchView.envPill, "ok", "Festgelegt");
  }
}

function renderFacts(data) {
  branchView.facts.replaceChildren();
  branchFact("Code-Repo", data.repoDir || "");
  const head = data.head || {};
  branchFact("Ausgecheckt", head.detached ? `losgelöst auf ${shortSha(head.sha)}` : head.branch || "unbekannt");
  const def = data.defaultBranch || {};
  if (def.state === "ok") {
    const source = def.source === "github" ? "von GitHub" : `aus refs/remotes/${data.remote}/HEAD`;
    branchFact("Default-Branch", `${def.name} (${source})`);
  } else {
    branchFact("Default-Branch", "unbekannt: " + (def.reason || ""));
  }
  branchFact("Letzter Fetch", fetchAge(data.fetch));
  if (data.git && !data.git.aheadBehind && data.git.reason) {
    branchFact(`git ${data.git.version}`, data.git.reason);
  }
  const policy = data.switch || {};
  branchView.policy.textContent = policy.message || "";
  branchView.policy.classList.toggle("warn", !policy.offered);
  const current = head.branch || "";
  setBranchPill(branchView.listPill, "ok", current || "losgelöst");
}

function renderGroups(data) {
  branchView.groups.replaceChildren();
  const groups = data.groups || [];
  if (groups.length === 0) {
    branchView.groups.textContent = "Keine Branches.";
    return;
  }
  for (const group of groups) {
    const list = branchNode("div", "branch-list");
    for (const branch of group.branches) {
      list.append(renderBranch(branch, data));
    }
    if (group.key === "work" && group.branches.length >= BRANCH_FOLD_FROM) {
      const fold = branchNode("details", "gh-fold branch-group");
      const summary = branchNode("summary", "disclosure gh-fold-head");
      summary.append(branchNode("span", "disclosure-marker", "▸"), branchNode("span", "", `${group.title} (${group.branches.length})`));
      summary.firstChild.setAttribute("aria-hidden", "true");
      fold.append(summary, list);
      branchView.groups.append(fold);
    } else {
      const section = branchNode("div", "branch-group");
      section.append(branchNode("h3", "branch-group-title", `${group.title} (${group.branches.length})`), list);
      branchView.groups.append(section);
    }
  }
}

function renderBranch(branch, data) {
  const item = branchNode("div", "branch-item");
  const head = branchNode("div", "branch-head");
  head.append(branchNode("span", "branch-name", branch.name));
  if (branch.current) {
    head.append(branchPill("ok", "ausgecheckt"));
  }
  if (branch.default) {
    head.append(branchPill("muted", "Default"));
  }
  for (const env of branch.environments || []) {
    head.append(branchPill(env.suggested ? "warn" : "ok", env.suggested ? `${env.name} (Vorschlag)` : env.name));
  }
  if (branch.remoteOnly) {
    head.append(branchPill("muted", `nur ${branch.remote}`));
  }
  if (branch.merged && branch.merged.state === "ok" && branch.merged.merged) {
    head.append(branchPill("muted", "gemergt"));
  }
  if (branch.worktree) {
    head.append(branchPill(branch.worktree.prunable ? "warn" : "muted", branch.worktree.prunable ? "Worktree (prunable)" : "anderer Worktree"));
  }
  if (branch.pull) {
    const link = branchNode("a", "branch-meta", `PR #${branch.pull.number}`);
    link.href = branch.pull.url || "#";
    link.rel = "noreferrer";
    link.title = branch.pull.title || "";
    head.append(link);
  }
  item.append(head);

  const commit = branch.commit || {};
  const meta = [];
  if (commit.date) {
    meta.push(new Date(commit.date).toLocaleString("de-DE"));
  }
  if (commit.author) {
    meta.push(commit.author);
  }
  if (commit.subject) {
    meta.push(commit.subject);
  }
  item.append(branchNode("p", "branch-meta", meta.join(" · ")));

  const distance = [];
  if (branch.upstream) {
    if (branch.upstream.gone) {
      distance.push(`Upstream ${branch.upstream.name}: gone`);
    } else {
      distance.push(`zu ${branch.upstream.name}: ${branch.upstream.ahead} vor, ${branch.upstream.behind} hinter`);
    }
  } else if (!branch.remoteOnly) {
    distance.push("kein Upstream");
  }
  const def = branch.toDefault || {};
  if (def.state === "ok") {
    distance.push(`zum Default-Branch: ${def.ahead} vor, ${def.behind} hinter`);
  } else if (def.state === "unknown") {
    distance.push("zum Default-Branch: unbekannt");
  }
  if (branch.merged && branch.merged.state === "unknown") {
    distance.push("gemergt: unbekannt");
  }
  item.append(branchNode("p", "branch-meta", distance.join(" · ")));
  if (def.state === "unknown" && def.reason) {
    item.append(branchNode("p", "branch-meta", def.reason));
  }

  const policy = data.switch || {};
  if (policy.offered && branch.allowed && !branch.current) {
    const actions = branchNode("div", "branch-actions");
    const button = branchNode("button", "secondary", "Umschalten prüfen");
    button.type = "button";
    const panel = branchNode("div", "branch-check hidden");
    button.addEventListener("click", () => checkSwitch(branch, button, panel));
    actions.append(button);
    item.append(actions, panel);
  }
  return item;
}

function renderWorktrees(data) {
  const worktrees = data.worktrees || [];
  branchView.worktrees.replaceChildren();
  branchView.worktrees.classList.toggle("empty", worktrees.length === 0);
  let prunable = 0;
  for (const worktree of worktrees) {
    const item = branchNode("div", "branch-item");
    const head = branchNode("div", "branch-head");
    head.append(branchNode("span", "branch-name", worktree.path));
    if (worktree.current) {
      head.append(branchPill("ok", "dieser"));
    }
    if (worktree.prunable) {
      prunable += 1;
      head.append(branchPill("warn", "prunable"));
    }
    if (worktree.locked) {
      head.append(branchPill("muted", "gesperrt"));
    }
    item.append(head);
    const what = worktree.detached ? `losgelöst auf ${shortSha(worktree.head)}` : worktree.bare ? "bare" : worktree.branch;
    item.append(branchNode("p", "branch-meta", what || ""));
    for (const reason of [worktree.prunableReason, worktree.lockedReason]) {
      if (reason) {
        item.append(branchNode("p", "branch-meta", reason));
      }
    }
    branchView.worktrees.append(item);
  }
  if (prunable > 0) {
    setBranchPill(branchView.worktreePill, "warn", `${prunable} prunable`);
  } else {
    setBranchPill(branchView.worktreePill, "ok", worktrees.length === 1 ? "1 Worktree" : `${worktrees.length} Worktrees`);
  }
}

async function runFetch() {
  branchView.fetchButton.disabled = true;
  branchView.fetchProgress.classList.remove("hidden");
  branchView.fetchMessage.textContent = "";
  try {
    const response = await fetch("/api/branches/fetch", { method: "POST" });
    const result = await response.json();
    branchView.fetchMessage.textContent = [result.message, result.output].filter(Boolean).join("\n");
  } catch {
    branchView.fetchMessage.textContent = "Der Fetch konnte nicht ausgeführt werden.";
  } finally {
    branchView.fetchProgress.classList.add("hidden");
  }
  await loadBranches();
}

async function checkSwitch(branch, button, panel) {
  button.disabled = true;
  panel.classList.remove("hidden");
  panel.replaceChildren(branchNode("p", "branch-meta", "Prüfen..."));
  const query = new URLSearchParams({ target: branch.name });
  if (branch.remoteOnly && branch.remote) {
    query.set("remote", branch.remote);
  }
  let result;
  try {
    const response = await fetch(`/api/branches/switch-check?${query}`, { cache: "no-store" });
    result = await response.json();
  } catch {
    panel.replaceChildren(branchNode("p", "message", "Die Prüfung konnte nicht ausgeführt werden."));
    button.disabled = false;
    return;
  }
  button.disabled = false;
  renderCheck(result, branch, panel);
}

function renderCheck(result, branch, panel) {
  panel.replaceChildren();
  if (result.state !== "ok") {
    panel.append(branchNode("p", "message", result.message || "Die Prüfung ist gescheitert."));
    return;
  }
  const list = branchNode("div", "check-list");
  for (const check of result.checks || []) {
    const row = branchNode("div", "check-item");
    const head = branchNode("div", "branch-head");
    let tone = CHECK_TONE[check.result] || "muted";
    if (check.result === "nicht-pruefbar") {
      tone = check.blocking ? "error" : "muted";
    }
    head.append(branchPill(tone, CHECK_LABEL[check.result] || check.result), branchNode("span", "check-title", check.title));
    row.append(head, branchNode("p", "branch-meta", check.reason));
    for (const detail of check.details || []) {
      row.append(branchNode("p", "branch-meta branch-detail", detail));
    }
    for (const process of check.processes || []) {
      row.append(branchNode("p", "branch-meta branch-detail", `${process.kind} · PID ${process.pid} · ${process.cwd}`));
    }
    list.append(row);
  }
  panel.append(list);
  if (!result.offered) {
    panel.append(branchNode("p", "hint warn branch-note", "Nicht angeboten: mindestens ein Prüfpunkt blockiert oder ist nicht prüfbar."));
    return;
  }
  const actions = branchNode("div", "branch-actions");
  const run = branchNode("button", "primary", `Umschalten auf ${branch.name}`);
  run.type = "button";
  run.addEventListener("click", () => openConfirm(result, branch, panel));
  actions.append(run);
  panel.append(actions);
}

function openConfirm(result, branch, panel) {
  pendingSwitch = { result, branch, panel };
  const source = result.source || {};
  branchView.dialogSource.textContent = source.detached ? `losgelöst auf ${shortSha(source.sha)}` : `${source.branch} (${shortSha(source.sha)})`;
  branchView.dialogTarget.textContent = `${branch.name} (${shortSha(result.targetSha)})`;
  branchView.dialogCommand.textContent = result.command;
  branchView.dialogHints.replaceChildren();
  const hints = (result.checks || []).filter((check) => check.result === "hinweis" || (check.result === "nicht-pruefbar" && !check.blocking));
  if (hints.length > 0) {
    const list = branchNode("ul", "branch-hints");
    for (const check of hints) {
      list.append(branchNode("li", "", `${check.title}: ${check.reason}`));
    }
    branchView.dialogHints.append(branchNode("p", "hint", "Hinweise der Prüfung:"), list);
  }
  branchView.dialogRun.textContent = `Umschalten auf ${branch.name}`;
  branchView.dialogRun.disabled = false;
  branchView.dialog.showModal();
}

async function runSwitch() {
  if (!pendingSwitch) {
    return;
  }
  const { result, branch, panel } = pendingSwitch;
  branchView.dialogRun.disabled = true;
  let answer;
  try {
    const response = await fetch("/api/branches/switch", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ target: branch.name, remote: branch.remoteOnly ? branch.remote : "", stamp: result.stamp }),
    });
    answer = await response.json();
  } catch {
    branchView.dialog.close();
    panel.append(branchNode("p", "message", "Das Umschalten konnte nicht angefragt werden."));
    return;
  }
  branchView.dialog.close();
  pendingSwitch = null;
  if (answer.executed && answer.ok) {
    await loadBranches();
    branchView.listMessage.textContent = `${answer.message}\n${answer.output || ""}`.trim();
    return;
  }
  if (!answer.executed && answer.check && answer.check.state === "ok") {
    renderCheck(answer.check, branch, panel);
  }
  if (answer.executed) {
    // git hat verweigert: der Stand danach gehört in die Liste, die Ausgabe
    // bleibt unter dem Branch stehen.
    await loadBranches();
    branchView.listMessage.textContent = answer.message || "";
    return;
  }
  panel.append(branchNode("p", "hint warn branch-note", answer.message || "Nicht umgeschaltet."));
  if (answer.output) {
    const output = branchNode("div", "command-row");
    output.append(branchNode("code", "", answer.output));
    panel.append(output);
  }
}
