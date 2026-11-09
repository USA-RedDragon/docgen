// The docgen playground page: an editor of the sample repository in the memfs, and buttons that run
// docgen check, docgen update and shell commands with the real Go toolchain, one Web Worker per process.
import { createSession, loadBundle, describe } from "./session.mjs";
import { encodeBundle, decodeBundle } from "./bundle.mjs";
import * as cm from "./editor-bundle.js";
import { createExplorer, iconOf, previewable } from "./explorer.mjs";
import { element, icon, longPress } from "./dom.mjs";
import { baseName, isUnder, movePath, normalizePath } from "./paths.mjs";

const $ = id => document.getElementById(id);
const textEncoder = new TextEncoder();
const textDecoder = new TextDecoder();
const query = new URLSearchParams(location.search);
const WORK = "/work";
const REPO = "/work/dg";
// The page at /playground/ and the page at /main/playground/ keep their own caches and workspaces.
const CACHE = "docgen-playground-v1:" + location.pathname;
const RUN_ENV = { GOFLAGS: "-p=" + (query.get("p") || "4") };
const WORKER_POOL_SIZE = 2;
const OUTPUT_MAX_NODES = 2000;
const PREVIEW_DELAY_MS = 250;
const SAVE_DELAY_MS = 300;
const NOTE_MS = 5000;
/** The first bytes of a file that isBinary reads. */
const BINARY_CHECK_BYTES = 8000;
const PREVIEW_PREFIX = "preview:";
const THEMES = ["auto", "light", "dark"];
const THEME_KEY = "pg-theme";

/** The state of the page for the browser test, as window.__pg. */
const pg = (window.__pg = {
  ready: false,
  busy: false,
  steps: [],
  errors: [],
  load: {},
  log: [],
  notes: [],
  saves: 0,
  renders: 0,
});
addEventListener("error", event => pg.errors.push(String(event.message || event)));
addEventListener("unhandledrejection", event => {
  pg.errors.push("unhandled rejection: " + String((event.reason && event.reason.stack) || event.reason));
});

const seconds = (ms, digits) => (ms / 1000).toFixed(digits) + " s";
const processCount = count => `${count} ${count === 1 ? "process" : "processes"}`;

// Theme: auto uses the system setting; light and dark set color-scheme on the root.

let theme = "auto";
try {
  theme = localStorage.getItem(THEME_KEY) || "auto";
} catch {
  // No storage: the theme is auto.
}

function applyTheme() {
  if (theme === "auto") delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = theme;
  $("theme").textContent = "Theme: " + theme;
}

$("theme").addEventListener("click", () => {
  theme = THEMES[(THEMES.indexOf(theme) + 1) % THEMES.length];
  try {
    localStorage.setItem(THEME_KEY, theme);
  } catch {
    // The theme then lasts until a reload.
  }
  applyTheme();
});
applyTheme();
pg.setTheme = name => {
  theme = name;
  applyTheme();
};

// Loading of the tools and bundles: from the Cache API when there, else fetched. An asset with the
// encoding zstd has Content-Encoding: zstd, so the browser decodes it, and the progress counts the
// bytes of the decoded files.

const formatMiB = bytes => (bytes / 2 ** 20).toFixed(1) + " MiB";

async function openCache() {
  try {
    return await caches.open(CACHE);
  } catch (error) {
    pg.log.push("no Cache API: " + error);
    return null;
  }
}

async function sha256Hex(bytes) {
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", bytes));
  return [...digest].map(byte => byte.toString(16).padStart(2, "0")).join("");
}

/**
 * fetchAsset fetches the asset of the manifest entry file, calls onBytes with each count of decoded
 * bytes that arrives, and checks the size and the SHA-256 of the decoded bytes. A response with a
 * Content-Encoding has the Content-Length of the encoded body, so only the size of the manifest
 * counts.
 */
async function fetchAsset(file, onBytes) {
  const response = await fetch(file.url);
  if (!response.ok) throw new Error(`${file.url}: HTTP ${response.status}`);
  const bytes = new Uint8Array(file.size);
  const reader = response.body.getReader();
  let received = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    if (received + value.length > file.size) throw new Error(`${file.url}: more than ${file.size} bytes`);
    bytes.set(value, received);
    received += value.length;
    onBytes(value.length);
  }
  if (received !== file.size) throw new Error(`${file.url}: ${received} of ${file.size} bytes`);
  const sum = await sha256Hex(bytes);
  if (sum !== file.sha256) throw new Error(`${file.name}: the SHA-256 is ${sum}, not ${file.sha256}`);
  return bytes;
}

/** showVersion shows the docgen version and commit of the page, with a link to the commit. */
function showVersion(manifest) {
  const docgen = manifest.docgen || {};
  const short = String(docgen.commit || "").slice(0, 7);
  const link = $("ver");
  const commit = short && short !== docgen.version ? " (" + short + ")" : "";
  link.textContent = `docgen ${docgen.version || "dev"}${commit}, ${manifest.go}`;
  if (/^[0-9a-f]{40}$/.test(docgen.commit || "")) {
    link.href = "https://github.com/USA-RedDragon/docgen/commit/" + docgen.commit;
  }
  pg.version = { docgen: docgen.version, commit: docgen.commit, go: manifest.go };
}

/**
 * loadAssets loads each file of the manifest and shows the progress. It returns {tools, bundles,
 * perFile, sha, ms, hits, files, bytes}: tools maps a name to a Promise of its WebAssembly.Module,
 * and bundles a name to its bytes.
 */
async function loadAssets() {
  const startedAt = performance.now();
  const manifest = await (await fetch("manifest.json", { cache: "no-cache" })).json();
  showVersion(manifest);
  const cache = await openCache();
  const cacheKey = file => new URL(`cache/${file.name}.${file.sha256.slice(0, 16)}`, location.href).href;
  if (cache) {
    const current = new Set(manifest.files.map(cacheKey));
    for (const request of await cache.keys()) if (!current.has(request.url)) await cache.delete(request);
  }
  const total = manifest.files.reduce((sum, file) => sum + file.size, 0);
  let received = 0;
  let hits = 0;
  let ready = 0;
  const rows = new Map();
  for (const file of manifest.files) {
    const row = element("li", {}, element("span", { textContent: file.name }), element("span", { textContent: formatMiB(file.size) }));
    $("lfiles").append(row);
    rows.set(file.name, row);
  }
  const showProgress = () => {
    // A small part per ready file, so the bar moves when all bytes came from the cache.
    $("lbar").value = (received + ready * 1e-9) / total;
    const fileCount = manifest.files.length;
    $("ltext").textContent = `${formatMiB(received)} of ${formatMiB(total)} fetched, ${ready} of ${fileCount} files ready`;
  };
  const fileReady = (file, how) => {
    ready++;
    rows.get(file.name).lastChild.textContent = `${formatMiB(file.size)} ${how}`;
    showProgress();
  };
  const assets = { tools: {}, bundles: {}, perFile: {}, sha: Object.fromEntries(manifest.files.map(file => [file.name, file.sha256])) };
  await Promise.all(
    manifest.files.map(async file => {
      const fileStartedAt = performance.now();
      const key = cacheKey(file);
      const cached = cache && (await cache.match(key));
      if (cached) {
        hits++;
        received += file.size;
        if (file.kind === "tool") {
          assets.tools[file.name] = WebAssembly.compileStreaming(cached);
          await assets.tools[file.name];
        } else {
          assets.bundles[file.name] = new Uint8Array(await cached.arrayBuffer());
        }
        assets.perFile[file.name] = Math.round(performance.now() - fileStartedAt);
        return fileReady(file, "cached");
      }
      const bytes = await fetchAsset(file, count => {
        received += count;
        showProgress();
      });
      if (cache) {
        const type = file.kind === "tool" ? "application/wasm" : "application/octet-stream";
        try {
          await cache.put(key, new Response(bytes, { headers: { "content-type": type } }));
        } catch (error) {
          pg.log.push(`cache put ${file.name}: ${error}`);
        }
      }
      if (file.kind === "tool") {
        assets.tools[file.name] = WebAssembly.compile(bytes);
        await assets.tools[file.name];
      } else {
        assets.bundles[file.name] = bytes;
      }
      assets.perFile[file.name] = Math.round(performance.now() - fileStartedAt);
      fileReady(file, "fetched");
    }),
  );
  assets.ms = Math.round(performance.now() - startedAt);
  assets.hits = hits;
  assets.files = manifest.files.length;
  assets.bytes = total;
  return assets;
}

// The kernel session, with a small pool of Workers that are ready before a process needs one.

const idleWorkers = [];
const newWorker = () => new Worker("proc.mjs", { type: "module" });
let session;

function makeSession(tools) {
  for (let i = 0; i < WORKER_POOL_SIZE; i++) idleWorkers.push(newWorker());
  const limitSeconds = +(query.get("limit") || 0);
  session = createSession({
    loadTool: name => tools[name],
    limitMs: limitSeconds * 1000,
    makeWorker: () => {
      const worker = idleWorkers.length ? idleWorkers.shift() : newWorker();
      while (idleWorkers.length < WORKER_POOL_SIZE) idleWorkers.push(newWorker());
      const handle = { post: message => worker.postMessage(message), terminate: () => worker.terminate(), onmessage: null };
      worker.onmessage = event => handle.onmessage && handle.onmessage(event.data);
      worker.onerror = event => pg.errors.push("worker error " + event.message);
      return handle;
    },
  });
  session.kernel.onSpawn = process => {
    lastTool = process.tool;
    spawnCount++;
  };
  pg.session = session;
}

// The files of the memfs.

const readText = path => textDecoder.decode(session.fsys.readFile(path));

function exists(path) {
  const entry = session.fsys.lookup(path);
  return entry && !entry.dir;
}

/** listFiles returns the paths of the files under root, sorted by name in each folder. */
function listFiles(root) {
  const files = [];
  const addFiles = (entry, path) => {
    const children = [...entry.children].sort((a, b) => a[0].localeCompare(b[0]));
    for (const [name, child] of children) {
      if (child.dir) addFiles(child, path + "/" + name);
      else files.push(path + "/" + name);
    }
  };
  const entry = session.fsys.lookup(root);
  if (entry && entry.dir) addFiles(entry, root);
  return files;
}

/** isBinary reports whether the file has a zero byte near its start. */
function isBinary(path) {
  const data = session.fsys.readFile(path);
  for (let i = 0; i < Math.min(data.length, BINARY_CHECK_BYTES); i++) if (data[i] === 0) return true;
  return false;
}

/** The content of each file of /work in the starter sample. */
let starter = new Map();
const snapshot = () => new Map(listFiles(WORK).map(path => [path, session.fsys.readFile(path).slice()]));
const sameBytes = (a, b) => a.length === b.length && a.every((byte, i) => byte === b[i]);

// The editor.

const LANGUAGES = {
  go: { name: "Go", support: () => cm.go() },
  yaml: { name: "YAML", support: () => cm.yaml() },
  yml: { name: "YAML", support: () => cm.yaml() },
  md: { name: "Markdown", support: () => cm.markdownLanguage.extension },
  markdown: { name: "Markdown", support: () => cm.markdownLanguage.extension },
  c: { name: "C", support: () => cm.cpp() },
  h: { name: "C", support: () => cm.cpp() },
  sh: { name: "Shell Script" },
  svg: { name: "SVG", support: () => cm.xml() },
  xml: { name: "XML", support: () => cm.xml() },
  json: { name: "JSON", support: () => cm.json() },
  toml: { name: "TOML" },
};
const FILE_NAME_LANGUAGES = { "go.mod": { name: "Go Module" }, "go.sum": { name: "Go Checksum File" } };

function languageOf(path) {
  const name = baseName(path);
  if (FILE_NAME_LANGUAGES[name]) return FILE_NAME_LANGUAGES[name];
  const dot = name.lastIndexOf(".");
  return (dot > 0 && LANGUAGES[name.slice(dot + 1).toLowerCase()]) || { name: "Plain Text" };
}

const color = name => `var(--vs-${name})`;
const tags = cm.tags;
const editorLook = [
  cm.EditorView.theme({
    "&": { height: "100%", color: color("fg"), backgroundColor: color("bg"), fontSize: "13.5px" },
    "&.cm-focused": { outline: "none" },
    ".cm-scroller": { fontFamily: "var(--font-mono)", lineHeight: "1.5", overflow: "auto" },
    ".cm-gutters": { color: color("line"), backgroundColor: color("bg"), border: "none" },
    ".cm-lineNumbers .cm-gutterElement": { padding: "0 14px 0 12px" },
    ".cm-activeLine": { backgroundColor: "light-dark(#f5f8fc, #232629)" },
    ".cm-activeLineGutter": { backgroundColor: "transparent", color: color("fg") },
    ".cm-cursor": { borderLeftColor: color("fg") },
    "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection": {
      backgroundColor: color("sel") + " !important",
    },
    ".cm-selectionMatch": { backgroundColor: "light-dark(#e2ecf8, #343a40)" },
    ".cm-panels": { backgroundColor: color("side"), color: color("fg") },
  }),
  cm.syntaxHighlighting(
    cm.HighlightStyle.define([
      { tag: [tags.keyword, tags.bool, tags.null, tags.atom], color: color("keyword") },
      { tag: [tags.controlKeyword, tags.moduleKeyword, tags.processingInstruction], color: color("control") },
      { tag: [tags.typeName, tags.className, tags.namespace], color: color("type") },
      { tag: [tags.string, tags.special(tags.string), tags.monospace], color: color("string") },
      { tag: [tags.number, tags.integer, tags.float], color: color("number") },
      { tag: [tags.comment, tags.lineComment, tags.blockComment], color: color("comment") },
      { tag: [tags.function(tags.variableName), tags.function(tags.propertyName)], color: color("function") },
      { tag: [tags.variableName, tags.propertyName], color: color("variable") },
      { tag: tags.definition(tags.propertyName), color: color("keyword") },
      { tag: [tags.heading, tags.link, tags.url], color: color("keyword") },
      { tag: tags.heading, fontWeight: "bold" },
      { tag: tags.strong, fontWeight: "bold" },
      { tag: tags.emphasis, fontStyle: "italic" },
    ]),
  ),
];
const narrow = matchMedia("(max-width: 640px)");
const lineWrapping = new cm.Compartment();
const fileSettings = new cm.Compartment();
let view;
/** The editor state of each file, so a tab switch keeps the undo history. */
const states = new Map();
/** The {path} object of each editor state, which the update listener of the state reads. */
const stateFiles = new Map();
/** The open tabs, each {path, kind: "file" or "preview"}. */
let tabs = [];
let current = null;
/** The path of the file of the preview to the side, or null. */
let side = null;

const relative = path => path.slice(WORK.length + 1);
const previewLabel = kind => (kind === "preview" ? "Preview " : "");
const tabTitle = tab => previewLabel(tab.kind) + baseName(tab.path);
const pickValue = tab => (tab.kind === "preview" ? PREVIEW_PREFIX + tab.path : tab.path);
const wrapping = () => (narrow.matches ? cm.EditorView.lineWrapping : []);

/** fileExtensions returns the settings of an editor that depend on the path of its file, so that a rename keeps the undo history. */
function fileExtensions(path) {
  const language = languageOf(path);
  return [
    cm.EditorView.contentAttributes.of({ "aria-label": relative(path) }),
    cm.indentUnit.of(path.endsWith(".go") ? "\t" : "  "),
    language.support ? language.support() : [],
  ];
}

function makeState(path) {
  const file = { path };
  stateFiles.set(path, file);
  return cm.EditorState.create({
    doc: readText(path),
    extensions: [
      cm.lineNumbers(),
      cm.highlightActiveLineGutter(),
      cm.highlightSpecialChars(),
      cm.history(),
      cm.drawSelection(),
      cm.dropCursor(),
      cm.EditorState.allowMultipleSelections.of(true),
      cm.indentOnInput(),
      cm.bracketMatching(),
      cm.closeBrackets(),
      cm.rectangularSelection(),
      cm.highlightActiveLine(),
      cm.highlightSelectionMatches(),
      cm.EditorState.tabSize.of(4),
      cm.keymap.of([...cm.closeBracketsKeymap, ...cm.defaultKeymap, ...cm.searchKeymap, ...cm.historyKeymap, cm.indentWithTab]),
      fileSettings.of(fileExtensions(path)),
      lineWrapping.of(wrapping()),
      editorLook,
      cm.EditorView.updateListener.of(update => {
        if (!update.docChanged) return;
        session.fsys.writeFile(file.path, textEncoder.encode(update.state.doc.toString()));
        markTree();
        livePreview();
        scheduleSave();
      }),
    ],
  });
}

function renderTabs() {
  const tabElements = tabs.map(tab => {
    const select = element("button", { type: "button", textContent: tabTitle(tab) });
    select.addEventListener("click", () => {
      saveState();
      show(tab);
    });
    const close = element("button", {
      type: "button",
      className: "vs-x",
      textContent: "×",
      attributes: { "aria-label": "Close " + tabTitle(tab) },
    });
    close.addEventListener("click", () => {
      saveState();
      closeTab(tab);
    });
    const tabIcon = icon(tab.kind === "preview" ? "i-pv" : "i-" + iconOf(baseName(tab.path), false));
    const tabElement = element(
      "div",
      {
        className: "vs-tab",
        title: previewLabel(tab.kind) + relative(tab.path),
        attributes: { role: "tab", "aria-selected": String(tab === current) },
      },
      tabIcon,
      select,
      close,
    );
    tabElement.dataset.path = tab.path;
    tabElement.dataset.kind = tab.kind;
    return tabElement;
  });
  $("tabs").replaceChildren(...tabElements);
  updateActions();
}

function openTab(path, kind = "file", focus = true) {
  let tab = tabs.find(each => each.path === path && each.kind === kind);
  if (!tab) {
    tab = { path, kind };
    tabs.push(tab);
    scheduleSave();
  }
  if (focus) show(tab);
  else renderTabs();
  return tab;
}

function showNoFile() {
  $("code").hidden = true;
  $("preview").hidden = true;
  $("empty").hidden = false;
  $("crumbs").textContent = "";
  $("lang").textContent = "";
}

function closeTab(tab) {
  const index = tabs.indexOf(tab);
  if (index < 0) return;
  tabs.splice(index, 1);
  scheduleSave();
  if (current === tab) current = null;
  if (!current && tabs.length) return show(tabs[Math.min(index, tabs.length - 1)]);
  if (!tabs.length) showNoFile();
  renderTabs();
  markTree();
}

/** updateActions shows Open Preview and Open Preview to the Side on a Markdown or SVG source, and Show Source on a preview. */
function updateActions() {
  const source = !!current && current.kind === "file" && previewable(current.path);
  $("openpv").hidden = !source;
  $("openside").hidden = !source;
  $("showsrc").hidden = !(current && current.kind === "preview");
}

$("openpv").addEventListener("click", () => {
  saveState();
  openTab(current.path, "preview");
});
$("openside").addEventListener("click", () => {
  saveState();
  openSide(current.path);
});
$("showsrc").addEventListener("click", () => openTab(current.path, "file"));
$("sidesrc").addEventListener("click", () => {
  if (!side) return;
  saveState();
  openTab(side, "file");
});
$("sideclose").addEventListener("click", () => closeSide());

function openSide(path) {
  side = path;
  $("sidebox").hidden = false;
  $("pane").classList.add("split");
  $("sidetitle").textContent = "Preview " + baseName(path);
  $("sidebox").title = "Preview " + relative(path);
  renderPreview(path, $("sidepv"));
  if (!current || current.path !== path || current.kind !== "file") openTab(path, "file");
  scheduleSave();
}

function closeSide() {
  if (!side) return;
  side = null;
  $("sidebox").hidden = true;
  $("pane").classList.remove("split");
  freeBlobs($("sidepv"));
  $("sidepv").replaceChildren();
  scheduleSave();
}

/** livePreview renders the visible previews again a short time after an edit. */
let previewTimer = 0;
function livePreview() {
  clearTimeout(previewTimer);
  previewTimer = setTimeout(() => {
    if (side) renderPreview(side, $("sidepv"));
    if (current && current.kind === "preview" && !$("preview").hidden) renderPreview(current.path, $("preview"));
  }, PREVIEW_DELAY_MS);
}

/** The blob URLs of the images of each preview box. */
const blobUrls = new Map();
function freeBlobs(box) {
  for (const url of blobUrls.get(box) || []) URL.revokeObjectURL(url);
  blobUrls.set(box, []);
}
const markdown = new cm.MarkdownIt({ html: true, linkify: false, typographer: false });

// sanitize keeps only the elements and attributes of an allow list in the parsed Markdown, so that
// no HTML in a page can run script: no script elements, no event attributes, and no URL with a
// scheme other than http, https and mailto. It removes an element of DROP with its content, and
// replaces another element that is not in KEEP with its content.
const KEEP = new Set(
  ("a abbr b blockquote br caption code col colgroup dd del details div dl dt em figcaption figure h1 h2 h3 h4 h5 h6 " +
    "hr i img ins kbd li mark ol p pre q s samp small span strong sub summary sup table tbody td tfoot th thead tr tt u ul var").split(" "),
);
const DROP = new Set(
  ("script style iframe frame frameset object embed applet svg math template noscript form input button select textarea " +
    "base meta link title head").split(" "),
);
const ATTRIBUTES = new Set("href src alt title width height align colspan rowspan start type open class lang dir".split(" "));
const HTML_NAMESPACE = "http://www.w3.org/1999/xhtml";

function isSafeUrl(value) {
  const compact = value.trim().replace(/[\u0000- ]/g, "");
  const scheme = compact.match(/^([a-z][a-z0-9+.-]*):/i);
  return !scheme || /^(https?|mailto)$/i.test(scheme[1]);
}

function sanitize(root) {
  for (const node of [...root.querySelectorAll("*")]) {
    const name = node.localName;
    if (DROP.has(name) || node.namespaceURI !== HTML_NAMESPACE) {
      node.remove();
      continue;
    }
    if (!KEEP.has(name)) {
      node.replaceWith(...node.childNodes);
      continue;
    }
    for (const attribute of [...node.attributes]) {
      const isUrl = attribute.name === "href" || attribute.name === "src";
      if (!ATTRIBUTES.has(attribute.name) || (isUrl && !isSafeUrl(attribute.value))) node.removeAttribute(attribute.name);
    }
  }
}

function blobUrl(box, bytes, type) {
  const url = URL.createObjectURL(new Blob([bytes], { type }));
  blobUrls.get(box).push(url);
  return url;
}

function renderPreview(path, box = $("preview")) {
  pg.renders++;
  const scrollTop = box.scrollTop;
  freeBlobs(box);
  box.classList.remove("vs-media");
  if (!exists(path)) {
    box.textContent = "This file no longer exists.";
    return;
  }
  if (/\.svg$/i.test(path)) {
    box.classList.add("vs-media");
    const image = element("img", { src: blobUrl(box, session.fsys.readFile(path), "image/svg+xml"), alt: relative(path) });
    image.dataset.path = relative(path);
    image.addEventListener("error", () => {
      image.replaceWith(element("p", { className: "vs-empty", textContent: "This SVG does not render. Check its source." }));
    });
    box.replaceChildren(image);
    return;
  }
  const template = document.createElement("template");
  template.innerHTML = markdown.render(readText(path));
  sanitize(template.content);
  const dir = path.slice(0, path.lastIndexOf("/"));
  for (const image of template.content.querySelectorAll("img")) {
    const src = image.getAttribute("src") || "";
    if (/^[a-z]+:/i.test(src)) continue;
    // A path that starts with a slash is relative to the root of the repository.
    const target = src.startsWith("/") ? REPO + src : normalizePath(dir + "/" + src.split("#")[0]);
    if (!exists(target)) continue;
    const type = target.endsWith(".svg") ? "image/svg+xml" : "application/octet-stream";
    image.src = blobUrl(box, session.fsys.readFile(target), type);
    image.dataset.path = relative(target);
  }
  for (const link of template.content.querySelectorAll("a[href]")) {
    if (!/^https?:/.test(link.getAttribute("href"))) continue;
    link.target = "_blank";
    link.rel = "noopener noreferrer";
  }
  box.replaceChildren(template.content);
  box.scrollTop = scrollTop;
}

function showMessage(text) {
  $("code").hidden = true;
  $("preview").hidden = false;
  $("preview").classList.remove("vs-media");
  $("preview").textContent = text;
}

function show(tab) {
  current = tab;
  const path = tab.path;
  const isPreview = tab.kind === "preview";
  $("empty").hidden = true;
  $("crumbs").textContent = relative(path).split("/").join(" › ");
  $("lang").textContent = languageOf(path).name + (isPreview ? " Preview" : "");
  $("pick").value = pickValue(tab);
  if (isPreview) {
    $("code").hidden = true;
    $("preview").hidden = false;
    renderPreview(path, $("preview"));
  } else if (!exists(path)) {
    showMessage("This file no longer exists.");
  } else if (isBinary(path)) {
    showMessage("This file is binary.");
  } else {
    $("preview").hidden = true;
    $("code").hidden = false;
    if (!states.has(path)) states.set(path, makeState(path));
    view.setState(states.get(path));
  }
  renderTabs();
  markTree();
  if (explorer) explorer.reveal(path);
  scheduleSave();
}

/** saveState keeps the state of the open editor, so a tab switch keeps its undo history. */
function saveState() {
  if (current && current.kind === "file" && states.has(current.path) && !$("code").hidden) {
    states.set(current.path, view.state);
  }
}

/** reloadFiles replaces the text of each editor state whose file changed in the memfs, as an undoable change. */
function reloadFiles() {
  saveState();
  const changed = [];
  for (const [path, state] of states) {
    if (!exists(path)) continue;
    const text = readText(path);
    if (text === state.doc.toString()) continue;
    states.set(path, state.update({ changes: { from: 0, to: state.doc.length, insert: text } }).state);
    changed.push(relative(path));
  }
  if (current) show(current);
  if (side) renderPreview(side, $("sidepv"));
  return changed;
}

/** changes returns the files of /work that differ from the starter: M for changed, U for new. */
function changes() {
  const marks = new Map();
  for (const path of listFiles(WORK)) {
    const old = starter.get(path);
    if (!old) marks.set(path, "U");
    else if (!sameBytes(old, session.fsys.readFile(path))) marks.set(path, "M");
  }
  return marks;
}

/** edited reports whether the file differs from the starter, or is a new file that is not empty. */
function edited(path) {
  if (!exists(path)) return false;
  const old = starter.get(path);
  const data = session.fsys.readFile(path);
  return old ? !sameBytes(old, data) : data.length > 0;
}

/** moved updates the tabs, the editor states and the side preview after the moves of pairs. */
function moved(pairs) {
  saveState();
  for (const tab of tabs) tab.path = movePath(tab.path, pairs);
  for (const [path, state] of [...states]) {
    const newPath = movePath(path, pairs);
    if (newPath === path) continue;
    states.delete(path);
    const file = stateFiles.get(path);
    stateFiles.delete(path);
    file.path = newPath;
    stateFiles.set(newPath, file);
    states.set(newPath, state.update({ effects: fileSettings.reconfigure(fileExtensions(newPath)) }).state);
  }
  if (side) {
    side = movePath(side, pairs);
    $("sidetitle").textContent = "Preview " + baseName(side);
  }
  if (current) show(current);
  else renderTabs();
  scheduleSave();
}

/** removed closes the tabs of deleted files, with a note when a closed file had edits. */
function removed(paths, lost) {
  saveState();
  const isRemoved = path => paths.some(removedPath => isUnder(path, removedPath));
  const closed = tabs.filter(tab => isRemoved(tab.path));
  const lostOpen = [...new Set(closed.filter(tab => lost.includes(tab.path)).map(tab => relative(tab.path)))];
  if (side && isRemoved(side)) closeSide();
  for (const tab of closed) closeTab(tab);
  for (const path of [...states.keys()]) {
    if (!isRemoved(path)) continue;
    states.delete(path);
    stateFiles.delete(path);
  }
  if (lostOpen.length) {
    const what = lostOpen.length === 1 ? "file had edits that are" : "files had edits that are";
    note(`Closed ${lostOpen.join(", ")}: the deleted ${what} now lost.`);
  } else if (closed.length) {
    const tabsText = closed.length === 1 ? "the tab" : closed.length + " tabs";
    note(`Closed ${tabsText} of the deleted ${paths.length === 1 ? "item" : "items"}.`);
  }
}

/** renderPick fills the file dropdown of the narrow layout. */
function renderPick() {
  const options = listFiles(WORK).flatMap(path => {
    const pathOptions = [new Option(relative(path), path)];
    if (previewable(path)) pathOptions.push(new Option("Preview " + relative(path), PREVIEW_PREFIX + path));
    return pathOptions;
  });
  $("pick").replaceChildren(...options);
  if (current) $("pick").value = pickValue(current);
}

function renderTree() {
  explorer.render();
  renderPick();
}

/** markTree marks each file that differs from the starter: M for changed, U for new. */
function markTree() {
  if (explorer) explorer.decorate();
}

// Messages and the confirm dialog, inside the page.

let noteTimer = 0;
function note(text) {
  pg.notes.push(text);
  const toast = $("toast");
  toast.textContent = text;
  toast.hidden = false;
  clearTimeout(noteTimer);
  noteTimer = setTimeout(() => (toast.hidden = true), NOTE_MS);
}

/** ask opens the confirm dialog and resolves to true on OK. */
function ask({ title, text = "", list = [], ok = "OK", danger = false }) {
  const dialog = $("dlg");
  $("dlgt").textContent = title;
  $("dlgm").textContent = text;
  $("dlgl").replaceChildren(...list.map(item => element("li", { textContent: item })));
  $("dlgok").textContent = ok;
  $("dlgok").classList.toggle("danger", danger);
  dialog.returnValue = "";
  const returnFocus = document.activeElement;
  dialog.showModal();
  $("dlgok").focus();
  return new Promise(resolve => {
    const onClose = () => {
      resolve(dialog.returnValue === "ok");
      if (returnFocus && returnFocus.isConnected) returnFocus.focus({ preventScroll: true });
    };
    dialog.addEventListener("close", onClose, { once: true });
  });
}
pg.ask = ask;

// The explorer.

let explorer = null;

function openFromTree(path, how, focusEditor) {
  saveState();
  if (how === "side") openSide(path);
  else openTab(path, how === "preview" ? "preview" : "file");
  if (narrow.matches) setDrawer(false);
  if (focusEditor && how !== "preview") view.focus();
}

function makeExplorer() {
  explorer = createExplorer({
    fsys: session.fsys,
    root: WORK,
    tree: $("tree"),
    side: $("side"),
    menu: $("menu"),
    open: openFromTree,
    changes,
    edited,
    moved,
    removed,
    note,
    ask,
    changed: () => {
      renderPick();
      scheduleSave();
      if (side) livePreview();
    },
    busy: () => pg.busy,
    canSide: () => !narrow.matches,
    reset: () => resetSample(),
    showTree: () => {
      if (narrow.matches) setDrawer(true);
    },
    copied: text => (pg.copied = text),
    downloaded: (name, blob) => (pg.download = { name, size: blob.size, blob }),
  });
  pg.explorer = explorer;
}

/** setDrawer opens or closes the explorer drawer of the layout of 640 px and less. */
function setDrawer(open) {
  $("vs").classList.toggle("ex-open", open);
  $("files").setAttribute("aria-expanded", String(open));
  if (!open) return;
  if (current) explorer.reveal(current.path);
  $("tree").focus({ preventScroll: true });
}

const drawerOpen = () => $("vs").classList.contains("ex-open");
$("files").addEventListener("click", () => setDrawer(!drawerOpen()));
$("scrim").addEventListener("click", () => setDrawer(false));
$("side").addEventListener("keydown", event => {
  if (event.key === "Escape" && narrow.matches && drawerOpen() && $("menu").hidden) setDrawer(false);
});
const actionsMenu = (x, y, path) => explorer.openMenu(x, y, [path || WORK], false);
$("more").addEventListener("click", () => {
  const box = $("more").getBoundingClientRect();
  actionsMenu(box.right - 240, box.bottom, current && current.path);
});
longPress($("tabs"), (x, y, event) => {
  const tab = event.target.closest(".vs-tab");
  if (tab) actionsMenu(x, y, tab.dataset.path);
});

// Storage of the workspace: the files and folders of /work and the open tabs, in IndexedDB, else in the
// Cache API. Without storage the page works the same, and changes last until a reload.

const DATABASE = "docgen-playground";
const STORE = "workspace";
const STORE_KEY = "work:" + location.pathname;
let sampleKey = "";
let sampleBundle = null;
let saveTimer = 0;
let store = null;

/** serializeWorkspace returns the bundle of /work, with the empty folders and the tabs in its header. */
function serializeWorkspace() {
  const dirs = [];
  const files = [];
  session.fsys.walk(WORK, (path, entry) => {
    if (entry.dir && !entry.children.size) dirs.push(path);
    if (!entry.dir) files.push([path, entry.data.subarray(0, entry.size), entry.mtime, entry.mode]);
  });
  const header = {
    v: 1,
    sample: sampleKey,
    dirs,
    tabs: tabs.map(tab => [tab.path, tab.kind]),
    current: current ? tabs.indexOf(current) : -1,
    side,
  };
  return encodeBundle(header, files);
}

const idbRequest = request =>
  new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });

/** openStore returns the storage of the workspace: {kind, get(), put(bytes), clear()}, or null. */
async function openStore() {
  try {
    const opening = indexedDB.open(DATABASE, 1);
    opening.onupgradeneeded = () => opening.result.createObjectStore(STORE);
    const database = await idbRequest(opening);
    const objectStore = mode => database.transaction(STORE, mode).objectStore(STORE);
    await idbRequest(objectStore("readonly").get(STORE_KEY));
    return {
      kind: "indexeddb",
      get: async () => (await idbRequest(objectStore("readonly").get(STORE_KEY))) || null,
      put: bytes => idbRequest(objectStore("readwrite").put(bytes, STORE_KEY)),
      clear: () => idbRequest(objectStore("readwrite").delete(STORE_KEY)),
    };
  } catch (error) {
    pg.log.push("no IndexedDB: " + error);
  }
  try {
    const cache = await caches.open("docgen-playground-workspace");
    const url = new URL("workspace.bin", location.href).href;
    return {
      kind: "cache",
      get: async () => {
        const response = await cache.match(url);
        return response ? new Uint8Array(await response.arrayBuffer()) : null;
      },
      put: bytes => cache.put(url, new Response(bytes)),
      clear: () => cache.delete(url),
    };
  } catch (error) {
    pg.log.push("no Cache API for the workspace: " + error);
  }
  return null;
}

function scheduleSave() {
  if (!pg.ready || !store) return;
  clearTimeout(saveTimer);
  saveTimer = setTimeout(saveNow, SAVE_DELAY_MS);
}

async function saveNow() {
  clearTimeout(saveTimer);
  if (!store || !pg.ready) return;
  try {
    saveState();
    await store.put(serializeWorkspace());
    pg.saves++;
  } catch (error) {
    pg.log.push("save: " + error);
    if (!pg.saveFailed) {
      pg.saveFailed = true;
      note("The browser did not save the workspace. Your changes last until you reload the page.");
    }
  }
}
addEventListener("pagehide", () => {
  if (saveTimer) saveNow();
});
document.addEventListener("visibilitychange", () => {
  if (document.hidden && saveTimer) saveNow();
});
pg.saveNow = saveNow;

/** restoreWorkspace replaces /work with the files and folders of a saved workspace. */
function restoreWorkspace({ header, content }) {
  session.fsys.lookup(WORK).children.clear();
  for (const dir of header.dirs) if (dir.startsWith(WORK + "/")) session.fsys.mkdirAll(dir);
  for (const [path, offset, length, mtime, mode] of header.files) {
    if (!path.startsWith(WORK + "/")) continue;
    session.fsys.writeFile(path, content.slice(offset, offset + length), mtime).mode = mode;
  }
}

function openDefaultTabs() {
  openTab(REPO + "/machine/machine.go", "file", false);
  openTab(REPO + "/docs/lamp.md", "file");
}

async function resetSample() {
  if (pg.busy) return note("Wait for the run to end before you reset the sample.");
  clearTimeout(saveTimer);
  closeSide();
  session.fsys.lookup(WORK).children.clear();
  loadBundle(session.fsys, sampleBundle);
  states.clear();
  stateFiles.clear();
  tabs = [];
  current = null;
  try {
    if (store) await store.clear();
  } catch (error) {
    pg.log.push("clear: " + error);
  }
  renderTree();
  openDefaultTabs();
  note("The sample has its starter files again.");
}
pg.resetSample = resetSample;

// The output pane.

const output = $("out");

/** term appends text to the output with the class className. A command line always gets its own span. */
function term(text, className = "") {
  if (!text) return;
  const atBottom = output.scrollTop + output.clientHeight >= output.scrollHeight - 4;
  const last = output.lastChild;
  const joins = last && last.nodeType === Node.ELEMENT_NODE && last.className === className && !className.startsWith("cmdl");
  if (joins) last.textContent += text;
  else output.append(element("span", { className, textContent: text }));
  while (output.childNodes.length > OUTPUT_MAX_NODES) output.firstChild.remove();
  if (atBottom) output.scrollTop = output.scrollHeight;
}
$("clear").addEventListener("click", () => output.replaceChildren());
pg.term = () => output.textContent;

// Runs, the status line, the busy state and Stop.

let running = null;
let lastTool = "";
let spawnCount = 0;
let statusTimer = 0;
const RUN_BUTTONS = ["check", "update", "run"];

function setBusy(busy, label) {
  pg.busy = busy;
  for (const id of RUN_BUTTONS) $(id).disabled = busy;
  $("stop").disabled = !busy;
  $("status").classList.toggle("busy", busy);
  clearInterval(statusTimer);
  if (!busy) return;
  const startedAt = performance.now();
  const showStatus = () => {
    $("step").textContent = `${label}${lastTool ? " · " + lastTool : ""}`;
    $("elapsed").textContent = seconds(performance.now() - startedAt, 1);
    $("info").textContent = `${processCount(session.kernel.stats.live)}, ${spawnCount} started`;
  };
  showStatus();
  statusTimer = setInterval(showStatus, 100);
}

/** run runs a command of the page and resolves with its step for the browser test, or null during another run. */
async function run(label, argv0, argv, cwd = REPO) {
  if (pg.busy) return null;
  saveState();
  lastTool = "";
  spawnCount = 0;
  setBusy(true, label);
  term(`$ ${label}\n`, "cmdl");
  const stdoutDecoder = new TextDecoder();
  const stderrDecoder = new TextDecoder();
  const before = snapshot();
  const startedAt = Date.now();
  const onOut = bytes => term(stdoutDecoder.decode(bytes, { stream: true }));
  const onErr = bytes => term(stderrDecoder.decode(bytes, { stream: true }), "err");
  const { proc, done } = session.start(argv0, argv, cwd, RUN_ENV, onOut, onErr);
  running = proc;
  const result = await done;
  running = null;
  const status = describe(result.code);
  const limit = result.reason === "limit" ? ", process limit" : "";
  term(`[${status}${limit} in ${(result.ms / 1000).toFixed(2)} s]\n`, result.code === 0 ? "ok" : "bad");
  const after = snapshot();
  const written = [...after].filter(([path, data]) => !before.has(path) || !sameBytes(before.get(path), data)).map(([path]) => relative(path));
  const reloaded = reloadFiles();
  renderTree();
  if (current) show(current);
  const step = {
    label,
    code: result.code,
    status,
    ms: Math.round(result.ms),
    wall: [startedAt, Date.now()],
    spawns: spawnCount,
    changed: written,
    reloaded,
  };
  pg.steps.push(step);
  setBusy(false);
  $("step").textContent = `${label}: ${status}`;
  $("elapsed").textContent = seconds(result.ms, 2);
  $("info").textContent = written.length ? "changed " + written.join(", ") : processCount(spawnCount);
  return step;
}
pg.run = run;

const docgen = command => run(`docgen ${command}`, "/usr/local/bin/docgen", ["docgen", command]);
$("check").addEventListener("click", () => docgen("check"));
$("update").addEventListener("click", () => docgen("update"));
$("cmdform").addEventListener("submit", event => {
  event.preventDefault();
  const command = $("cmd").value.trim();
  if (command) run(command, "/bin/sh", ["sh", "-c", command]);
});
$("stop").addEventListener("click", () => {
  if (!running) return;
  const killed = session.kernel.killTree(running.pid);
  term(`[Stop: killed ${killed} processes]\n`, "bad");
});
$("pick").addEventListener("change", () => {
  saveState();
  const value = $("pick").value;
  if (value.startsWith(PREVIEW_PREFIX)) openTab(value.slice(PREVIEW_PREFIX.length), "preview");
  else openTab(value);
});
narrow.addEventListener("change", () => {
  setDrawer(false);
  explorer.closeMenu();
  saveState();
  for (const [path, state] of states) states.set(path, state.update({ effects: lineWrapping.reconfigure(wrapping()) }).state);
  if (current) show(current);
});

// A file dropped outside the explorer and the editor does nothing, so that the browser does not leave the page to show it.
for (const type of ["dragover", "drop"]) {
  addEventListener(type, event => {
    if (event.defaultPrevented || !event.dataTransfer) return;
    if ([...event.dataTransfer.types].includes("Files")) event.preventDefault();
  });
}

// Hooks for the browser test.

pg.open = (path, kind = "file") => {
  saveState();
  return openTab(REPO + "/" + path, kind);
};
pg.replace = (path, from, to) => {
  pg.open(path);
  const text = view.state.doc.toString();
  const index = text.indexOf(from);
  if (index < 0) throw new Error(`${path}: no ${from}`);
  view.dispatch({ changes: { from: index, to: index + from.length, insert: to } });
  return true;
};
pg.text = path => readText(REPO + "/" + path);
pg.editorText = () => view.state.doc.toString();
pg.cursorEnd = () => {
  view.dispatch({ selection: { anchor: view.state.doc.length } });
  view.focus();
};
pg.previewText = () => $("preview").textContent;
const imagesOf = box =>
  [...box.querySelectorAll("img")].map(image => ({
    path: image.dataset.path,
    w: image.naturalWidth,
    h: image.naturalHeight,
    complete: image.complete,
    src: image.src,
  }));
pg.previewImages = () => imagesOf($("preview"));
pg.sideImages = () => imagesOf($("sidepv"));
pg.sideText = () => (side ? $("sidepv").textContent : null);
pg.tabs = () => tabs.map(tab => ({ path: relative(tab.path), kind: tab.kind, current: tab === current }));
pg.current = () =>
  current && { path: relative(current.path), kind: current.kind, code: !$("code").hidden, lang: $("lang").textContent };
pg.side = () => side && relative(side);
pg.exists = path => exists(REPO + "/" + path);
pg.isDir = path => {
  const entry = session.fsys.lookup(REPO + "/" + path);
  return !!entry && entry.dir;
};
pg.marks = () => Object.fromEntries([...changes()].map(([path, mark]) => [relative(path), mark]));

// Start.

/** clearStorage removes the asset cache, the workspace and the open folders, for ?clearcache. */
async function clearStorage() {
  try {
    await caches.delete(CACHE);
  } catch {
    // No Cache API.
  }
  try {
    if (store) await store.clear();
  } catch {
    // No workspace to clear.
  }
  try {
    localStorage.removeItem("pg-explorer-open");
  } catch {
    // No localStorage.
  }
}

/** readWorkspace returns the saved workspace of the current sample, or null. */
async function readWorkspace() {
  let workspace = null;
  try {
    const bytes = store && (await store.get());
    workspace = bytes && decodeBundle(bytes);
  } catch (error) {
    pg.log.push("restore: " + error);
  }
  if (workspace && workspace.header.sample !== sampleKey) {
    pg.restoreSkipped = "sample changed";
    return null;
  }
  return workspace;
}

function restoreTabs(header) {
  for (const [path, kind] of header.tabs) {
    if (exists(path) && (kind === "file" || previewable(path))) openTab(path, kind, false);
  }
  const saved = header.current >= 0 && header.tabs[header.current];
  const savedTab = saved && tabs.find(tab => tab.path === saved[0] && tab.kind === saved[1]);
  if (savedTab) show(savedTab);
  else if (tabs.length) show(tabs[0]);
  else {
    $("code").hidden = true;
    $("empty").hidden = false;
    renderTabs();
  }
  if (header.side && exists(header.side)) openSide(header.side);
}

try {
  const navigationToAssets = performance.now();
  store = await openStore();
  pg.storage = store ? store.kind : "none";
  if (query.has("clearcache")) await clearStorage();
  const assets = await loadAssets();
  pg.load = {
    ms: assets.ms,
    hits: assets.hits,
    files: assets.files,
    bytes: assets.bytes,
    perFile: assets.perFile,
    navToAssets: Math.round(navigationToAssets),
  };
  makeSession(assets.tools);
  const bundlesStartedAt = performance.now();
  for (const name of ["root", "gocache", "sample"]) {
    loadBundle(session.fsys, assets.bundles[name], name === "gocache" ? Date.now() : 0);
  }
  pg.load.bundlesMs = Math.round(performance.now() - bundlesStartedAt);
  starter = snapshot();
  sampleBundle = assets.bundles.sample;
  sampleKey = assets.sha.sample;
  const workspace = await readWorkspace();
  if (workspace) restoreWorkspace(workspace);
  pg.restored = !!workspace;
  view = new cm.EditorView({ parent: $("code") });
  makeExplorer();
  renderTree();
  if (workspace) restoreTabs(workspace.header);
  else openDefaultTabs();
  $("loader").hidden = true;
  for (const id of RUN_BUTTONS) $(id).disabled = false;
  pg.load.total = Math.round(performance.now());
  $("step").textContent = "Ready";
  $("elapsed").textContent = `loaded in ${(pg.load.total / 1000).toFixed(1)} s`;
  const source = assets.hits === assets.files ? "tools from the browser cache" : `${assets.files - assets.hits} files fetched`;
  $("info").textContent = source + (workspace ? ", workspace restored" : "");
  pg.ready = true;
  if (!store) note("This browser has no storage for the page. Your changes last until you reload.");
} catch (error) {
  pg.errors.push("load: " + String((error && error.stack) || error));
  $("lfiles").append(element("li", { className: "lerr", textContent: "Load failed: " + String((error && error.message) || error) }));
}
