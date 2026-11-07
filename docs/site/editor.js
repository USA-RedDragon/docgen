// editor.js upgrades each .editor block of a page to a read-only code editor.
// With no JavaScript, or when codemirror.js does not load, the block stays as
// plain headings and code.

const languages = {
  go: { name: "Go", support: (cm) => cm.go() },
  yaml: { name: "YAML", support: (cm) => cm.yaml() },
  yml: { name: "YAML", support: (cm) => cm.yaml() },
  md: { name: "Markdown", support: (cm) => cm.markdownLanguage.extension },
  c: { name: "C", support: (cm) => cm.cpp() },
  h: { name: "C", support: (cm) => cm.cpp() },
  cc: { name: "C++", support: (cm) => cm.cpp() },
  cpp: { name: "C++", support: (cm) => cm.cpp() },
  hpp: { name: "C++", support: (cm) => cm.cpp() },
  sh: { name: "Shell Script" },
  svg: { name: "SVG" },
};
const fileNames = { "go.mod": { name: "Go Module" } };
const plainText = { name: "Plain Text" };

const gapLabel = "⋮";
const lineHeight = 21;
const charWidth = 8.4;
const treeRow = 23;
const treeTop = 30;
const tabsHeight = 46;
const barHeight = 30;
const pickHeight = 36;
const statusHeight = 24;
const explorerIcon = '<svg viewBox="0 0 16 16" aria-hidden="true"><rect x="1.5" y="2.5" width="13" height="11"/><path d="M6 2.5v11"/></svg>';
const theatreIcon = '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M2 6V2h4M10 2h4v4M14 10v4h-4M6 14H2v-4"/></svg>';
const narrow = window.matchMedia("(max-width: 640px)");
const explorerViews = new Set();
let explorerClosed = readExplorer();
let mounted = 0;

function readExplorer() {
  try {
    return localStorage.getItem("explorer") === "closed";
  } catch {
    return false;
  }
}

// toggleExplorer shows or hides the explorer of each editor of the page.
function toggleExplorer() {
  explorerClosed = !explorerClosed;
  try {
    localStorage.setItem("explorer", explorerClosed ? "closed" : "open");
  } catch {
    // The state then lasts for this page only.
  }
  explorerViews.forEach((sync) => sync());
}

function languageOf(path) {
  const name = path.split("/").pop();
  if (fileNames[name]) {
    return fileNames[name];
  }
  const dot = name.lastIndexOf(".");
  return (dot > 0 && languages[name.slice(dot + 1).toLowerCase()]) || plainText;
}

// gutterLabel returns the formatNumber function of the labels lines. A number
// past the last line sets the width of the gutter, so it gets the longest label.
function gutterLabel(lines) {
  const widest = lines.reduce((a, b) => (b.length > a.length ? b : a), "");
  return (n) => lines[n - 1] ?? widest;
}

// label sets the accessible name and the tooltip of button.
function label(button, text) {
  button.setAttribute("aria-label", text);
  button.title = text;
}

function toolButton(className, icon) {
  const button = el("button", { type: "button", class: `vs-tool ${className}` });
  button.innerHTML = icon;
  return button;
}

/**
 * theatreButton returns the button that moves the editor box out of host to a
 * modal dialog that fills the window, and back.
 *
 * @param {HTMLElement} host The element that holds box.
 * @param {() => HTMLElement} box The editor.
 * @param {() => void} measure Called after each move.
 */
function theatreButton(host, box, measure) {
  const button = toolButton("vs-theatre-button", theatreIcon);
  const dialog = el("dialog", { class: "editor vs-theatre", "aria-label": "Editor" });
  label(button, "Open theatre mode");
  dialog.addEventListener("close", () => {
    host.append(box());
    host.style.removeProperty("height");
    dialog.remove();
    document.documentElement.classList.remove("vs-locked");
    label(button, "Open theatre mode");
    measure();
    button.focus();
  });
  button.addEventListener("click", () => {
    if (dialog.open) {
      dialog.close();
      return;
    }
    host.style.height = `${host.offsetHeight}px`;
    document.documentElement.classList.add("vs-locked");
    document.body.append(dialog);
    dialog.append(box());
    dialog.showModal();
    label(button, "Close theatre mode");
    measure();
    button.focus();
  });
  return button;
}

function el(tag, props, ...children) {
  const node = document.createElement(tag);
  for (const [key, value] of Object.entries(props || {})) {
    if (key === "class") {
      node.className = value;
    } else {
      node.setAttribute(key, value);
    }
  }
  node.append(...children);
  return node;
}

// styles returns the theme and the highlight style. The colors are the --vs-*
// properties of the template, so they follow the site theme with no script.
function styles(cm) {
  const t = cm.tags;
  const color = (name) => `var(--vs-${name})`;
  return [
    cm.EditorView.theme({
      "&": { height: "100%", color: color("fg"), backgroundColor: color("bg"), fontSize: "var(--vs-code-size, 14px)" },
      "&.cm-focused": { outline: "none" },
      ".cm-scroller": { fontFamily: "var(--font-mono)", lineHeight: "1.5", overflow: "auto" },
      ".cm-gutters": { color: color("line"), backgroundColor: color("bg"), border: "none" },
      ".cm-lineNumbers .cm-gutterElement": { padding: "0 14px 0 12px" },
      ".cm-content:focus-visible": { outline: `1px solid ${color("accent")}`, outlineOffset: "-1px" },
    }),
    cm.syntaxHighlighting(
      cm.HighlightStyle.define([
        { tag: [t.keyword, t.bool, t.null, t.atom], color: color("keyword") },
        { tag: [t.controlKeyword, t.moduleKeyword, t.processingInstruction], color: color("control") },
        { tag: [t.typeName, t.className, t.namespace], color: color("type") },
        { tag: [t.string, t.special(t.string), t.monospace], color: color("string") },
        { tag: [t.number, t.integer, t.float], color: color("number") },
        { tag: [t.comment, t.lineComment, t.blockComment], color: color("comment") },
        { tag: [t.function(t.variableName), t.function(t.propertyName)], color: color("function") },
        { tag: [t.variableName, t.propertyName], color: color("variable") },
        { tag: t.definition(t.propertyName), color: color("keyword") },
        { tag: [t.heading, t.link, t.url], color: color("keyword") },
        { tag: t.heading, fontWeight: "bold" },
        { tag: t.strong, fontWeight: "bold" },
        { tag: t.emphasis, fontStyle: "italic" },
      ]),
    ),
  ];
}

// explorer returns the file tree of the paths of files, and calls open with the
// index of the file of a clicked item. It sets the items of buttons by index. The
// item of a generated file gets a badge.
function explorer(files, root, open, buttons) {
  const top = { dirs: new Map(), files: [] };
  files.forEach((file, index) => {
    const parts = file.path.split("/");
    const name = parts.pop();
    let node = top;
    for (const part of parts) {
      if (!node.dirs.has(part)) {
        node.dirs.set(part, { dirs: new Map(), files: [] });
      }
      node = node.dirs.get(part);
    }
    node.files.push({ name, index });
  });
  function list(node, depth) {
    const items = el("ul");
    for (const name of [...node.dirs.keys()].sort()) {
      const summary = el("summary", { style: `--depth: ${depth}` }, name);
      items.append(el("li", null, el("details", { open: "" }, summary, list(node.dirs.get(name), depth + 1))));
    }
    for (const { name, index } of [...node.files].sort((a, b) => a.name.localeCompare(b.name))) {
      const button = el("button", { type: "button", style: `--depth: ${depth + 1}`, title: files[index].path }, name);
      if (files[index].image) {
        button.classList.add("vs-gen");
        button.append(el("span", { class: "vs-badge", "aria-hidden": "true" }, "U"));
      }
      button.addEventListener("click", () => open(index));
      buttons[index] = button;
      items.append(el("li", null, button));
    }
    return items;
  }
  if (!root) {
    return el("div", { class: "vs-tree" }, list(top, 0));
  }
  const summary = el("summary", { style: "--depth: 0" }, root);
  return el("div", { class: "vs-tree" }, el("details", { class: "vs-root", open: "" }, summary, list(top, 1)));
}

/**
 * mountEditor replaces the content of host with a read-only editor. The promise
 * rejects, with host as it was, when codemirror.js does not load. The height comes
 * from the --vs-fit property of host, and with none it is the limit of the style.
 *
 * @param {HTMLElement} host The element to fill.
 * @param {{path: string, versions: {name?: string, text: string, lines?: string[]}[], preview?: Node, image?: {src: string, owner: number, version: number}}[]} files
 *   The files. path is the path to show. lines is the label of each line of the
 *   text, and with none the editor shows no line numbers. A file with two versions
 *   or more shows a switch. preview is the rendered file. image makes the file an
 *   image that a version of another file generates: src is its URL, owner is the
 *   index of that file, and version is the index of that version. The file exists
 *   only in that version, and it has no versions of its own.
 * @param {string} [root] The name of the top folder of the explorer.
 */
export async function mountEditor(host, files, root) {
  const cm = await import("./codemirror.js");
  const look = styles(cm);
  const id = `vs-${++mounted}`;
  const chosen = files.map(() => 0);
  const views = files.map(() => "image");
  const sources = new Map();
  const entries = [];
  files.forEach((file, index) => {
    entries.push({ index, title: file.path.split("/").pop() });
    if (file.preview) {
      entries.push({ index, title: `Preview ${file.path.split("/").pop()}`, preview: true });
    }
  });
  let current = entries[0];

  const pane = el("div", { class: "vs-pane", id: `${id}-pane`, role: "tabpanel" });
  const code = el("div", { class: "vs-code" });
  const preview = el("div", { class: "vs-preview", tabindex: "0" });
  pane.append(code, preview);
  const view = new cm.EditorView({ parent: code });

  const tree = [];
  const pick = el("select", { class: "vs-pick", "aria-label": "File" });
  files.forEach((file, index) => pick.append(el("option", { value: index }, file.path)));
  pick.addEventListener("change", () => show(entries.find((e) => e.index === Number(pick.value) && !e.preview)));

  const strip = el("div", { class: "vs-tabs", role: "tablist", "aria-label": "Open files" });
  const tabs = entries.map((entry) => {
    const tab = el("button", { type: "button", role: "tab", "aria-controls": pane.id }, entry.title);
    tab.addEventListener("click", () => show(entry));
    strip.append(tab);
    return tab;
  });
  strip.addEventListener("keydown", (event) => {
    const step = { ArrowRight: 1, ArrowLeft: -1 }[event.key];
    if (step) {
      event.preventDefault();
      const next = (entries.indexOf(current) + step + entries.length) % entries.length;
      show(entries[next]);
      tabs[next].focus();
    }
  });

  const crumbs = el("div", { class: "vs-crumbs" });
  const toggle = el("div", { class: "vs-switch", role: "group", "aria-label": "Version" });
  const viewSwitch = el("div", { class: "vs-switch", role: "group", "aria-label": "View" });
  for (const [value, name] of [
    ["image", "Image"],
    ["source", "Source"],
  ]) {
    const button = el("button", { type: "button", value }, name);
    button.addEventListener("click", () => {
      views[current.index] = value;
      show(current);
    });
    viewSwitch.append(button);
  }
  const language = el("span");
  const place = el("span");

  // exists reports whether the file of index is in the chosen version.
  function exists(index) {
    const { image } = files[index];
    return !image || chosen[image.owner] === image.version;
  }

  // source returns the text of the image file of index, from the site.
  function source(index) {
    if (!sources.has(index)) {
      sources.set(
        index,
        fetch(files[index].image.src)
          .then((response) => (response.ok ? response.text() : Promise.reject(new Error(`HTTP ${response.status}`))))
          .then((text) => text.replace(/\n$/, ""))
          .catch((error) => `The source of ${files[index].path} did not load: ${error.message}`),
      );
    }
    return sources.get(index);
  }

  // textOf returns the text and line labels that the editor shows for entry.
  async function textOf(entry) {
    const file = files[entry.index];
    if (!file.image) {
      return file.versions[chosen[entry.index]];
    }
    const text = await source(entry.index);
    return { text, lines: text.split("\n").map((_, n) => String(n + 1)) };
  }

  async function render() {
    const entry = current;
    const file = files[entry.index];
    const info = languageOf(file.path);
    const version = await textOf(entry);
    if (entry !== current || code.hidden) {
      return;
    }
    view.setState(
      cm.EditorState.create({
        doc: version.text,
        extensions: [
          cm.EditorState.readOnly.of(true),
          cm.EditorState.tabSize.of(4),
          cm.EditorView.editable.of(false),
          cm.EditorView.contentAttributes.of({ tabindex: "0", "aria-label": file.path }),
          version.lines ? cm.lineNumbers({ formatNumber: gutterLabel(version.lines) }) : [],
          narrow.matches ? cm.EditorView.lineWrapping : [],
          info.support ? info.support(cm) : [],
          look,
        ],
      }),
    );
  }

  function show(entry) {
    current = entry;
    const file = files[entry.index];
    tabs.forEach((tab, n) => {
      tab.setAttribute("aria-selected", String(entries[n] === entry));
      tab.tabIndex = entries[n] === entry ? 0 : -1;
    });
    tree.forEach((button, n) => button.setAttribute("aria-current", String(n === entry.index)));
    markGenerated();
    pick.value = String(entry.index);
    crumbs.textContent = file.path.split("/").join(" › ");
    place.textContent = file.path;
    const owner = file.image ? file.image.owner : entry.index;
    toggle.hidden = entry.preview || files[owner].versions.length < 2;
    toggle.replaceChildren(
      ...files[owner].versions.map((version, n) => {
        const button = el("button", { type: "button", "aria-pressed": String(n === chosen[owner]) }, version.name || "");
        button.addEventListener("click", () => {
          chosen[owner] = n;
          show(entry);
        });
        return button;
      }),
    );
    const present = exists(entry.index);
    const picture = file.image && present && views[entry.index] === "image";
    viewSwitch.hidden = !file.image || !present;
    [...viewSwitch.children].forEach((button) => button.setAttribute("aria-pressed", String(button.value === views[entry.index])));
    language.textContent = entry.preview || picture ? `${languageOf(file.path).name} Preview` : languageOf(file.path).name;
    const rendered = entry.preview || picture || !present;
    code.hidden = rendered;
    preview.hidden = !rendered;
    preview.classList.toggle("vs-media", Boolean(file.image));
    if (entry.preview) {
      preview.replaceChildren(file.preview);
    } else if (!present) {
      const after = files[owner].versions[file.image.version].name;
      preview.replaceChildren(
        el("p", { class: "vs-absent" }, `${file.path} does not exist yet. docgen update writes it. Select ${after} to see it.`),
      );
    } else if (picture) {
      preview.replaceChildren(el("img", { src: file.image.src, alt: file.path }));
    } else {
      render();
    }
  }

  // markGenerated sets the state of the item of each generated file of the
  // explorer and of the file list.
  function markGenerated() {
    files.forEach((file, index) => {
      if (!file.image) {
        return;
      }
      const name = file.path.split("/").pop();
      const after = files[file.image.owner].versions[file.image.version].name;
      const present = exists(index);
      const note = present
        ? `Generated by docgen update in ${after}; not in the page before`
        : `Not created yet; docgen update writes this file. Select ${after} to see it`;
      const button = tree[index];
      if (button) {
        button.classList.toggle("vs-not-yet", !present);
        button.title = `${file.path}\n${note}`;
        button.setAttribute("aria-label", `${name}, ${present ? "generated" : "not created yet"}`);
      }
      pick.options[index].textContent = `${file.path} (${present ? "generated" : "not created yet"})`;
    });
  }

  const side = el("div", { class: "vs-side", id: `${id}-side` }, el("p", { class: "vs-title" }, "EXPLORER"));
  side.append(explorer(files, root, (index) => show(entries.find((e) => e.index === index && !e.preview)), tree));
  const bar = el("div", { class: "vs-bar" }, crumbs, el("div", { class: "vs-switches" }, viewSwitch, toggle));
  const explorerButton = toolButton("vs-explorer", explorerIcon);
  explorerButton.setAttribute("aria-controls", side.id);
  explorerButton.addEventListener("click", toggleExplorer);
  const theatre = theatreButton(host, () => box, () => view.requestMeasure());
  const head = el("div", { class: "vs-head" }, explorerButton, strip, theatre);
  const main = el("div", { class: "vs-main" }, pick, head, bar, pane);
  const status = el("div", { class: "vs-status" }, place, language);
  const box = el("div", { class: "vs" }, side, main, status);
  const syncExplorer = () => {
    box.classList.toggle("vs-closed", explorerClosed);
    explorerButton.setAttribute("aria-expanded", String(!explorerClosed));
    label(explorerButton, explorerClosed ? "Show explorer" : "Hide explorer");
    view.requestMeasure();
  };
  explorerViews.add(syncExplorer);
  syncExplorer();
  narrow.addEventListener("change", () => code.hidden || render());
  show(current);
  host.replaceChildren(box);
}

// joinParts returns the version of the excerpts parts of one file. A blank line
// with a gap label stands for the lines between two excerpts.
function joinParts(parts) {
  const text = [];
  const lines = [];
  let next = 0;
  parts.forEach((part, index) => {
    if (index > 0 && (!part.line || part.line !== next)) {
      text.push("");
      lines.push(gapLabel);
    }
    const rows = part.text.split("\n");
    rows.forEach((row, n) => {
      text.push(row);
      lines.push(part.line ? String(part.line + n) : "");
    });
    next = part.line ? part.line + rows.length : 0;
  });
  return { text: text.join("\n"), lines: parts.some((part) => part.line) ? lines : undefined };
}

// readFiles returns the file list of the plain .editor block host. The files that
// a page generates follow that page.
function readFiles(host) {
  return [...host.querySelectorAll(".editor-file")].flatMap((node) => {
    const preview = node.querySelector(".editor-preview")?.cloneNode(true);
    preview?.querySelector("h5")?.remove();
    const file = {
      path: node.dataset.path,
      versions: [...node.querySelectorAll(".editor-version")].map((version) => ({
        name: version.dataset.name,
        ...joinParts(
          [...version.querySelectorAll("pre")].map((pre) => ({
            text: pre.textContent.replace(/\n$/, ""),
            line: Number(pre.closest("[data-line]")?.dataset.line) || 0,
          })),
        ),
      })),
      preview,
    };
    const generated = [...node.querySelectorAll(".editor-generated")].map((image) => ({
      path: image.dataset.path,
      versions: [],
      image: { src: image.dataset.src, version: file.versions.findIndex((version) => version.name === image.dataset.version) },
    }));
    return [file, ...generated.filter((image) => image.image.version >= 0)];
  });
}

// rowsOf returns the rows of text. Long lines wrap in a narrow editor.
function rowsOf(text, width) {
  const lines = text.split("\n");
  if (!narrow.matches) {
    return lines.length;
  }
  const columns = Math.max(1, Math.floor((width - 60) / charWidth));
  return lines.reduce((rows, line) => rows + Math.max(1, Math.ceil(line.length / columns)), 0);
}

// treeRows returns the rows of the explorer for files.
function treeRows(files, root) {
  const rows = new Set();
  for (const { path } of files) {
    const parts = path.split("/");
    parts.forEach((_, n) => rows.add(parts.slice(0, n + 1).join("/")));
  }
  return rows.size + (root ? 1 : 0);
}

// previewHeight returns the height in pixels of the previews of the plain block
// host. A preview with an image gets more than any limit of the style, as the
// height of the image is not known yet.
function previewHeight(host) {
  const nodes = [...host.querySelectorAll(".editor-preview")];
  const images = host.querySelector(".editor-generated") ? 9999 : 0;
  return Math.max(images, ...nodes.map((node) => (node.querySelector("img") ? 9999 : node.offsetHeight)));
}

// fitHeight returns the height in pixels that shows the longest file of block
// with no scroll.
function fitHeight({ host, files, preview }) {
  const width = host.clientWidth;
  const stacked = width <= 560;
  const versions = files.flatMap((file) => file.versions);
  const pane = Math.max(preview, 8 + lineHeight * Math.max(...versions.map((version) => rowsOf(version.text, width))));
  const hasSwitch = files.some((file) => file.versions.length > 1);
  const main = pane + tabsHeight + (stacked ? pickHeight + (hasSwitch ? barHeight : 0) : barHeight);
  const tree = stacked ? 0 : treeTop + treeRow * treeRows(files, host.dataset.root);
  return Math.max(main, tree) + statusHeight;
}

// ownersOf sets the owner of each generated file of files to the index of the
// file before it that is not generated.
function ownersOf(files) {
  let owner = 0;
  files.forEach((file, index) => {
    if (file.image) {
      file.image.owner = owner;
    } else {
      owner = index;
    }
  });
  return files;
}

const blocks = [...document.querySelectorAll(".editor")].map((host) => ({
  host,
  files: ownersOf(readFiles(host)),
  preview: previewHeight(host),
}));

function reserve() {
  const fits = blocks.map(fitHeight);
  blocks.forEach(({ host }, index) => {
    if (!host.classList.contains("large")) {
      host.style.setProperty("--vs-fit", `${fits[index]}px`);
    }
  });
}

// release leaves each block that has no editor as plain headings and code.
function release(observer) {
  observer.disconnect();
  for (const { host } of blocks) {
    delete host.dataset.pending;
    host.style.removeProperty("--vs-fit");
  }
}

const observer = new IntersectionObserver(
  (seen) => {
    for (const { isIntersecting, target } of seen) {
      if (isIntersecting) {
        observer.unobserve(target);
        const { files } = blocks.find(({ host }) => host === target);
        mountEditor(target, files, target.dataset.root).then(
          () => delete target.dataset.pending,
          () => release(observer),
        );
      }
    }
  },
  { rootMargin: "600px 0px" },
);

reserve();
narrow.addEventListener("change", reserve);
for (const { host } of blocks) {
  host.dataset.pending = "";
  observer.observe(host);
}
document.getElementById(decodeURIComponent(location.hash.slice(1)))?.scrollIntoView();
