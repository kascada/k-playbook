"use strict";

// Der gemeinsame Betrachter für gerenderte Markdown-Dateien.
//
// Zwei Seiten zeigen dasselbe: /docs die ausgewählte Datei der mitgelieferten
// Doku, /knowledge die Wissensablage. Was beide brauchen — Anker, Querverweise
// und Mermaid — steht deshalb hier und nicht zweimal daneben. Was sie
// unterscheidet, bleibt bei ihnen: welche Datei geöffnet wird, und wohin ein
// Verweis führt.

// Setzt den gerenderten Text in die Karte und springt an die gewünschte
// Stelle. Das HTML kommt aus dem eigenen Backend, gerendert mit abgeschaltetem
// Roh-HTML — es steht also nichts darin, was nicht aus der Markdown-Struktur
// der Datei stammt.
function showDoc(viewer, html, anchor = "") {
  viewer.classList.remove("empty");
  viewer.innerHTML = html || "";
  scrollToAnchor(viewer, anchor);
  renderMermaidDiagrams(viewer);
}

// Fängt die Klicks im Text ab. Anker springen innerhalb der Datei, Ziele mit
// Schema gehen in einen neuen Tab, und ein Verweis auf eine `.md`-Datei geht an
// den Aufrufer: die Docs-Seite öffnet sie in ihrer eigenen Karte, andere Seiten
// schicken den Leser damit in den Bereich Docs.
//
// Ohne dieses Abfangen führte ein roher Klick auf einen Pfad, den der Server
// nicht kennt, statt in die gerenderte Ansicht.
function setUpDocLinks(viewer, openMarkdown) {
  viewer.addEventListener("click", (event) => {
    const link = event.target.closest("a[href]");
    if (!link) {
      return;
    }

    const href = link.getAttribute("href");
    if (href.startsWith("#")) {
      event.preventDefault();
      scrollToAnchor(viewer, href.slice(1));
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
      openMarkdown(target, anchor);
    }
  });
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

// Springt zu einer Überschrift der offenen Datei. Ohne Anker beginnt die Datei
// oben — sonst bliebe die Ansicht dort stehen, wo die vorige endete.
//
// Der Text ist kein eigener Scroll-Container: er steht in einer Karte,
// gescrollt wird die Seite.
function scrollToAnchor(viewer, anchor) {
  const target = anchor ? viewer.querySelector(`#${CSS.escape(anchor)}`) : null;
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
