"use strict";

// Seite "Docs": die mitgelieferte Doku aus k-playbook/docs.
//
// Der Index steht links im Menü, der gerenderte Text rechts. Anders als auf
// den übrigen Seiten entsteht das Menü nicht aus den Karten daneben, sondern
// aus der geladenen Dateiliste — dieser Bereich hat genau einen Inhalt.
//
// Quelle ist /api/docs; gelesen wird nur.

const elements = {
  blockNav: document.getElementById("block-nav"),
  docCard: document.getElementById("doc-card"),
  docTitle: document.getElementById("doc-title"),
  docPath: document.getElementById("doc-path"),
  docViewer: document.getElementById("doc-viewer"),
  docsMessage: document.getElementById("docs-message"),
};

// Die offene Datei; Verweise darin werden relativ zu ihr aufgelöst.
let currentDocPath = "";

// Das Lebenszeichen dieses Fensters: es hält den Dienst aus dem Leerlauf und
// merkt, wenn er weg ist.
startSession((message) => {
  elements.docsMessage.textContent = message;
});
elements.docViewer.addEventListener("click", onDocViewerClick);
loadDocs();

async function loadDocs() {
  elements.docViewer.classList.add("empty");
  elements.docViewer.textContent = "Wird geladen...";

  try {
    const response = await fetch("/api/docs", { cache: "no-store" });
    renderDocs(await response.json());
  } catch {
    elements.docViewer.textContent = "";
    elements.docsMessage.textContent = "Doku konnte nicht geladen werden.";
  }
}

function renderDocs(data) {
  elements.blockNav.replaceChildren();
  elements.docsMessage.textContent = data.message || "";

  if (!data.available) {
    elements.docViewer.textContent = "";
    elements.docsMessage.textContent = "Keine Projektkonfiguration gefunden.";
    return;
  }

  // Fehlt das Verzeichnis, steht der Grund in der Meldung; eine leere Liste
  // wäre dafür die falsche Auskunft.
  if (data.message) {
    elements.docViewer.textContent = "";
    return;
  }

  const docs = data.docs || [];
  if (docs.length === 0) {
    elements.docViewer.textContent = "";
    elements.docsMessage.textContent = "Keine Markdown-Dateien vorhanden.";
    return;
  }

  for (const doc of docs) {
    elements.blockNav.append(docNavItem(doc));
  }

  // Ohne Auswahl steht die README da: sie ist der Einstieg und steht deshalb
  // auch in der Liste vorn.
  const start = docs.find((doc) => doc.path === "README.md") || docs[0];
  openDoc(start.path, start.title || start.path);
}

// Derselbe Eintrag wie im kartenbasierten Menü der übrigen Seiten — nur ohne
// Statuspunkt: eine Doku-Datei hat keinen Zustand, den er spiegeln könnte.
function docNavItem(doc) {
  const item = document.createElement("button");
  item.type = "button";
  item.className = "block-nav-item";
  item.dataset.path = doc.path;
  // Der Titel steht auf dem Eintrag, der Dateiname gehört trotzdem dazu.
  item.title = doc.path;

  const label = document.createElement("span");
  label.textContent = doc.title || doc.path;
  item.append(label);

  item.addEventListener("click", () => openDoc(doc.path, doc.title || doc.path));
  return item;
}

async function openDoc(path, title, anchor = "") {
  currentDocPath = path;
  elements.docTitle.textContent = title || path;
  elements.docPath.textContent = path;
  setActiveDocPath(path);
  elements.docViewer.classList.add("empty");
  elements.docViewer.textContent = "Wird geladen...";

  try {
    const response = await fetch(`/api/docs/file?path=${encodeURIComponent(path)}`, { cache: "no-store" });
    const data = await response.json();
    // Wurde inzwischen etwas anderes geöffnet, gehört diese Antwort nicht
    // mehr in die Karte.
    if (currentDocPath !== path) {
      return;
    }
    if (data.message) {
      elements.docViewer.textContent = data.message;
      return;
    }

    elements.docTitle.textContent = data.title || title || path;
    elements.docPath.textContent = data.path || path;
    elements.docViewer.classList.remove("empty");
    // Das HTML kommt aus dem eigenen Backend. Gerendert wird dort mit
    // abgeschaltetem Roh-HTML, es steht also nichts darin, was nicht aus der
    // Markdown-Struktur der Datei stammt.
    elements.docViewer.innerHTML = data.html || "";
    scrollToAnchor(anchor);
    renderMermaidDiagrams(elements.docViewer);
  } catch {
    elements.docViewer.textContent = "Datei konnte nicht geladen werden.";
  }
}

// Markiert den Eintrag der offenen Datei. Ein Verweis im Text kann in eine
// Datei führen, die nicht angeklickt wurde — das Menü zieht dann mit.
function setActiveDocPath(path) {
  for (const item of elements.blockNav.querySelectorAll(".block-nav-item")) {
    if (item.dataset.path === path) {
      markBlockNavItem(item);
      return;
    }
  }

  // Kein Eintrag passt: ein Verweis kann in eine Datei außerhalb des Index
  // führen. Bliebe die alte Markierung stehen, zeigte das Menü auf eine Datei,
  // die gar nicht mehr offen ist.
  clearBlockNavMarking();
}

// Verweise in der Doku zeigen überwiegend auf andere Dateien der Doku.
// Abgefangen werden sie, damit das Menü mitzieht und Anker innerhalb der
// Datei springen — ein roher Klick führte auf einen Pfad, den der Server
// nicht kennt, statt in die gerenderte Ansicht.
function onDocViewerClick(event) {
  const link = event.target.closest("a[href]");
  if (!link) {
    return;
  }

  const href = link.getAttribute("href");
  if (href.startsWith("#")) {
    event.preventDefault();
    scrollToAnchor(href.slice(1));
    return;
  }

  // Ein Ziel mit Schema führt aus der Doku heraus und gehört in ein eigenes
  // Fenster.
  if (/^[a-z][a-z0-9+.-]*:/i.test(href)) {
    link.target = "_blank";
    link.rel = "noopener";
    return;
  }

  event.preventDefault();
  const [target, anchor] = splitAnchor(href);

  // Ein reiner Anker ohne Dateiname ist bereits oben abgefangen; bleibt ein
  // Verweis auf eine Datei. Alles außer Markdown kann diese Ansicht nicht
  // zeigen, der Pfad steht aber im Text und lässt sich im Editor öffnen.
  if (target.toLowerCase().endsWith(".md")) {
    openDoc(resolveDocPath(currentDocPath, target), "", anchor);
  }
}

// Löst einen Verweis gegen das Verzeichnis der offenen Datei auf; die
// URL-Klasse erledigt dabei "./" und "../".
function resolveDocPath(base, href) {
  const resolved = new URL(href, `https://docs.invalid/${base}`);
  return decodeURIComponent(resolved.pathname).replace(/^\//, "");
}

function splitAnchor(href) {
  const index = href.indexOf("#");
  return index === -1 ? [href, ""] : [href.slice(0, index), href.slice(index + 1)];
}

// Springt zu einer Überschrift der offenen Datei. Ohne Anker beginnt die
// Datei oben — sonst bliebe die Ansicht dort stehen, wo die vorige endete.
//
// Anders als im Fenster über der Startseite ist der Text hier kein eigener
// Scroll-Container: er steht in einer Karte, gescrollt wird die Seite.
function scrollToAnchor(anchor) {
  const target = anchor ? elements.docViewer.querySelector(`#${CSS.escape(anchor)}`) : null;
  if (target) {
    target.scrollIntoView();
    return;
  }
  window.scrollTo({ top: 0 });
}

// Mermaid ist zu groß, um es mitzuliefern, und wird deshalb nur bei Bedarf vom
// CDN geholt. Ohne Netz bleibt der Quelltext des Diagramms als Codeblock
// stehen — die Datei ist dann immer noch lesbar.
const MERMAID_MODULE_URL = "https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs";

let mermaidLoader = null;
let mermaidDiagramCount = 0;

async function renderMermaidDiagrams(container) {
  const blocks = Array.from(container.querySelectorAll("pre > code.language-mermaid"));
  if (blocks.length === 0) {
    return;
  }

  let mermaid;
  try {
    mermaid = await loadMermaid();
  } catch (error) {
    for (const block of blocks) {
      const note = document.createElement("p");
      note.className = "mermaid-message";
      note.textContent = `Mermaid konnte nicht geladen werden (${error.message}); das Diagramm bleibt als Quelltext stehen.`;
      block.closest("pre").before(note);
    }
    return;
  }

  for (const block of blocks) {
    const pre = block.closest("pre");
    // Das Laden dauert; inzwischen kann eine andere Datei in der Karte stehen.
    if (!pre.isConnected) {
      continue;
    }
    const source = block.textContent.trim();

    const diagram = document.createElement("div");
    diagram.className = "mermaid-diagram";
    diagram.setAttribute("aria-label", "Mermaid-Diagramm");
    pre.replaceWith(diagram);

    try {
      const { svg } = await mermaid.render(`doc-mermaid-${++mermaidDiagramCount}`, source);
      diagram.innerHTML = svg;
    } catch (error) {
      // Ein fehlerhaftes Diagramm ersetzt sich selbst durch die Meldung und
      // seinen Quelltext, damit die Stelle im Text nicht einfach verschwindet.
      diagram.classList.add("mermaid-error");
      diagram.textContent = `Diagramm konnte nicht gezeichnet werden: ${error.message}`;
      diagram.append(pre);
    }
  }
}

function loadMermaid() {
  if (!mermaidLoader) {
    mermaidLoader = import(MERMAID_MODULE_URL).then((module) => {
      const mermaid = module.default;
      mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: "neutral" });
      return mermaid;
    });
  }
  return mermaidLoader;
}
