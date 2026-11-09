// The docgen playground page: an editor of the sample repository in the memfs, and buttons that run
// docgen check, docgen update and shell commands with the real Go toolchain, one Web Worker per process.
import { createSession, loadBundle, describe } from "./session.mjs";
import * as cm from "./editor-bundle.js";
import { createExplorer, icon, iconOf, previewable, under, baseName, longPress } from "./explorer.mjs";

const $ = id => document.getElementById(id);
const enc = new TextEncoder();
const q = new URLSearchParams(location.search);
const WORK = "/work";
const REPO = "/work/dg";
// The page at /playground/ and the page at /main/playground/ keep their own caches and workspaces.
const CACHE = "docgen-playground-v1:" + location.pathname;
const env = { GOFLAGS: "-p=" + (q.get("p") || "4") };
const pg = (window.__pg = { ready: false, busy: false, steps: [], errors: [], load: {}, log: [], notes: [], saves: 0, renders: 0 });
addEventListener("error", e => pg.errors.push(String(e.message || e)));
addEventListener("unhandledrejection", e => pg.errors.push("unhandled rejection: " + String(e.reason && e.reason.stack || e.reason)));

// Theme: auto uses the system setting; light and dark set color-scheme on the root.
const themes = ["auto", "light", "dark"];
let theme = "auto";
try { theme = localStorage.getItem("pg-theme") || "auto"; } catch {}
function applyTheme() {
  if (theme === "auto") delete document.documentElement.dataset.theme; else document.documentElement.dataset.theme = theme;
  $("theme").textContent = "Theme: " + theme;
}
$("theme").addEventListener("click", () => {
  theme = themes[(themes.indexOf(theme) + 1) % themes.length];
  try { localStorage.setItem("pg-theme", theme); } catch {}
  applyTheme();
});
applyTheme();
pg.setTheme = t => { theme = t; applyTheme(); };

// Loading of the tools and bundles: from the Cache API when there, else fetched (zstd) and decoded in Workers.
const fmtMB = n => (n / 2 ** 20).toFixed(1) + " MiB";
let decoders = [], nextDecoder = 0, decodeId = 0, decoderURL = "";
const decodeWaits = new Map();
function decoder() {
  if (!decoders.length) {
    const n = Math.min(3, Math.max(1, (navigator.hardwareConcurrency || 2) - 1));
    for (let i = 0; i < n; i++) {
      const w = new Worker("unzstd-worker.js?wasm=" + encodeURIComponent(decoderURL));
      w.onmessage = e => { const f = decodeWaits.get(e.data.id); decodeWaits.delete(e.data.id); if (f) f(e.data); else if (e.data.error) pg.errors.push(e.data.error); };
      decoders.push(w);
    }
  }
  return decoders[nextDecoder++ % decoders.length];
}
const decode = (data, size) => new Promise((res, rej) => {
  const id = ++decodeId;
  decodeWaits.set(id, r => (r.error ? rej(new Error(r.error)) : res(r.out)));
  decoder().postMessage({ id, data, size }, [data.buffer]);
});

async function openCache() {
  try { return await caches.open(CACHE); } catch (e) { pg.log.push("no Cache API: " + e); return null; }
}

async function fetchProgress(url, onBytes) {
  const r = await fetch(url);
  if (!r.ok) throw new Error(`${url}: HTTP ${r.status}`);
  const len = +r.headers.get("content-length") || 0;
  const reader = r.body.getReader();
  const parts = []; let n = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    parts.push(value); n += value.length; onBytes(value.length);
  }
  const out = new Uint8Array(n); let o = 0;
  for (const p of parts) { out.set(p, o); o += p.length; }
  if (len && n !== len) throw new Error(`${url}: ${n} of ${len} bytes`);
  return out;
}

// showVersion shows the docgen version and commit of the page, with a link to the commit.
function showVersion(m) {
  const d = m.docgen || {};
  const short = String(d.commit || "").slice(0, 7);
  const a = $("ver");
  a.textContent = `docgen ${d.version || "dev"}${short && short !== d.version ? " (" + short + ")" : ""}, ${m.go}`;
  if (/^[0-9a-f]{40}$/.test(d.commit || "")) a.href = "https://github.com/USA-RedDragon/docgen/commit/" + d.commit;
  pg.version = { docgen: d.version, commit: d.commit, go: m.go };
}

async function loadAssets() {
  const t0 = performance.now();
  const manifest = await (await fetch("manifest.json", { cache: "no-cache" })).json();
  decoderURL = manifest.decoder;
  showVersion(manifest);
  const cache = await openCache();
  const keyOf = f => new URL(`cache/${f.name}.${f.sha256.slice(0, 16)}`, location.href).href;
  if (cache) {
    const want = new Set(manifest.files.map(keyOf));
    for (const k of await cache.keys()) if (!want.has(k.url)) await cache.delete(k);
  }
  const total = manifest.files.reduce((a, f) => a + f.zsize, 0);
  let got = 0, hits = 0, ready = 0;
  const rows = new Map();
  for (const f of manifest.files) {
    const li = document.createElement("li");
    li.innerHTML = `<span>${f.name}</span><span>${fmtMB(f.size)}</span>`;
    $("lfiles").append(li); rows.set(f.name, li);
  }
  const bar = $("lbar");
  const tick = () => { bar.value = (got + ready * 1e-9) / total; $("ltext").textContent = `${fmtMB(got)} of ${fmtMB(total)} fetched (zstd), ${ready} of ${manifest.files.length} files ready`; };
  const done = (f, how) => { ready++; rows.get(f.name).lastChild.textContent = `${fmtMB(f.size)} ${how}`; tick(); };
  const out = { tools: {}, bundles: {}, perFile: {}, sha: Object.fromEntries(manifest.files.map(f => [f.name, f.sha256])) };
  await Promise.all(manifest.files.map(async f => {
    const ft = performance.now();
    const key = keyOf(f);
    let hit = cache && await cache.match(key);
    if (hit) {
      hits++; got += f.zsize;
      if (f.kind === "tool") out.tools[f.name] = WebAssembly.compileStreaming(hit);
      else out.bundles[f.name] = new Uint8Array(await hit.arrayBuffer());
      if (f.kind === "tool") await out.tools[f.name];
      out.perFile[f.name] = Math.round(performance.now() - ft);
      return done(f, "cached");
    }
    const z = await fetchProgress(f.url, n => { got += n; tick(); });
    const raw = await decode(z, f.size);
    if (raw.length !== f.size) throw new Error(`${f.name}: decoded ${raw.length} bytes, want ${f.size}`);
    if (cache) {
      try { await cache.put(key, new Response(raw, { headers: { "content-type": f.kind === "tool" ? "application/wasm" : "application/octet-stream" } })); }
      catch (e) { pg.log.push(`cache put ${f.name}: ${e}`); }
    }
    if (f.kind === "tool") { out.tools[f.name] = WebAssembly.compile(raw); await out.tools[f.name]; }
    else out.bundles[f.name] = raw;
    out.perFile[f.name] = Math.round(performance.now() - ft);
    done(f, "fetched");
  }));
  for (const w of decoders) w.terminate();
  decoders = [];
  out.ms = Math.round(performance.now() - t0);
  out.hits = hits; out.files = manifest.files.length; out.zbytes = total;
  out.bytes = manifest.files.reduce((a, f) => a + f.size, 0);
  return out;
}

// Kernel session with a small pool of booted Workers.
const POOL = 2;
const idle = [];
const newWorker = () => new Worker("proc.js");
let S;
function makeSession(tools) {
  for (let i = 0; i < POOL; i++) idle.push(newWorker());
  const limit = +(q.get("limit") || 0);
  S = createSession({
    loadTool: n => tools[n],
    limitMs: limit * 1000,
    makeWorker: () => {
      const w = idle.length ? idle.shift() : newWorker();
      while (idle.length < POOL) idle.push(newWorker());
      const o = { post: m => w.postMessage(m), terminate: () => w.terminate(), onmessage: null };
      w.onmessage = e => o.onmessage && o.onmessage(e.data);
      w.onerror = e => pg.errors.push("worker error " + e.message);
      return o;
    },
  });
  S.kernel.toolEnv = { compile: { GOGC: "50" }, link: { GOGC: "50" } };
  S.kernel.onSpawn = p => { lastTool = p.tool; spawns++; };
  pg.session = S;
}

// Files of the memfs.
const dec = new TextDecoder();
const readText = p => dec.decode(S.fsys.readFile(p));
const exists = p => { const e = S.fsys.lookup(p); return e && !e.dir; };
function listFiles(root) {
  const out = [];
  const walk = (e, p) => { for (const [n, c] of [...e.children].sort((a, b) => a[0].localeCompare(b[0]))) { const q = p + "/" + n; if (c.dir) walk(c, q); else out.push(q); } };
  const e = S.fsys.lookup(root);
  if (e && e.dir) walk(e, root);
  return out;
}
const isBinary = p => { const d = S.fsys.readFile(p); for (let i = 0; i < Math.min(d.length, 8000); i++) if (d[i] === 0) return true; return false; };
let base = new Map();
const snapshot = () => new Map(listFiles(WORK).map(p => [p, S.fsys.readFile(p).slice()]));
const same = (a, b) => a.length === b.length && a.every((x, i) => x === b[i]);

// Editor.
const languages = {
  go: { name: "Go", support: () => cm.go() }, yaml: { name: "YAML", support: () => cm.yaml() }, yml: { name: "YAML", support: () => cm.yaml() },
  md: { name: "Markdown", support: () => cm.markdownLanguage.extension }, c: { name: "C", support: () => cm.cpp() }, h: { name: "C", support: () => cm.cpp() },
  sh: { name: "Shell Script" }, svg: { name: "SVG", support: () => (cm.xml ? cm.xml() : []) }, xml: { name: "XML", support: () => (cm.xml ? cm.xml() : []) },
  json: { name: "JSON", support: () => (cm.json ? cm.json() : []) }, toml: { name: "TOML" }, markdown: { name: "Markdown", support: () => cm.markdownLanguage.extension },
};
const fileNames = { "go.mod": { name: "Go Module" }, "go.sum": { name: "Go Checksum File" } };
function languageOf(path) {
  const name = path.split("/").pop();
  if (fileNames[name]) return fileNames[name];
  const dot = name.lastIndexOf(".");
  return (dot > 0 && languages[name.slice(dot + 1).toLowerCase()]) || { name: "Plain Text" };
}
const color = n => `var(--vs-${n})`;
const t = cm.tags;
const look = [
  cm.EditorView.theme({
    "&": { height: "100%", color: color("fg"), backgroundColor: color("bg"), fontSize: "13.5px" },
    "&.cm-focused": { outline: "none" },
    ".cm-scroller": { fontFamily: "var(--font-mono)", lineHeight: "1.5", overflow: "auto" },
    ".cm-gutters": { color: color("line"), backgroundColor: color("bg"), border: "none" },
    ".cm-lineNumbers .cm-gutterElement": { padding: "0 14px 0 12px" },
    ".cm-activeLine": { backgroundColor: "light-dark(#f5f8fc, #232629)" },
    ".cm-activeLineGutter": { backgroundColor: "transparent", color: color("fg") },
    ".cm-cursor": { borderLeftColor: color("fg") },
    "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection": { backgroundColor: color("sel") + " !important" },
    ".cm-selectionMatch": { backgroundColor: "light-dark(#e2ecf8, #343a40)" },
    ".cm-panels": { backgroundColor: color("side"), color: color("fg") },
  }),
  cm.syntaxHighlighting(cm.HighlightStyle.define([
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
    { tag: t.heading, fontWeight: "bold" }, { tag: t.strong, fontWeight: "bold" }, { tag: t.emphasis, fontStyle: "italic" },
  ])),
];
const narrow = matchMedia("(max-width: 640px)");
const wrap = new cm.Compartment();
const fileConf = new cm.Compartment();
let view;
const states = new Map();
const refs = new Map();
let tabs = [], current = null, side = null;

// fileExt holds the settings of an editor that depend on the path of its file, so that a rename keeps the undo history.
const fileExt = path => {
  const info = languageOf(path);
  return [cm.EditorView.contentAttributes.of({ "aria-label": rel(path) }), cm.indentUnit.of(path.endsWith(".go") ? "\t" : "  "), info.support ? info.support() : []];
};
function makeState(path) {
  const ref = { path };
  refs.set(path, ref);
  return cm.EditorState.create({
    doc: readText(path),
    extensions: [
      cm.lineNumbers(), cm.highlightActiveLineGutter(), cm.highlightSpecialChars(), cm.history(), cm.drawSelection(), cm.dropCursor(),
      cm.EditorState.allowMultipleSelections.of(true), cm.indentOnInput(), cm.bracketMatching(), cm.closeBrackets(), cm.rectangularSelection(),
      cm.highlightActiveLine(), cm.highlightSelectionMatches(), cm.EditorState.tabSize.of(4),
      cm.keymap.of([...cm.closeBracketsKeymap, ...cm.defaultKeymap, ...cm.searchKeymap, ...cm.historyKeymap, cm.indentWithTab]),
      fileConf.of(fileExt(path)),
      wrap.of(narrow.matches ? cm.EditorView.lineWrapping : []),
      look,
      cm.EditorView.updateListener.of(u => { if (u.docChanged) { S.fsys.writeFile(ref.path, enc.encode(u.state.doc.toString())); markTree(); livePreview(); dirty(); } }),
    ],
  });
}

const rel = p => p.slice(WORK.length + 1);
const tabTitle = tab => (tab.kind === "preview" ? "Preview " : "") + tab.path.split("/").pop();

function renderTabs() {
  const strip = $("tabs");
  strip.replaceChildren(...tabs.map(tab => {
    const d = document.createElement("div");
    d.className = "vs-tab"; d.setAttribute("role", "tab"); d.setAttribute("aria-selected", String(tab === current));
    d.title = (tab.kind === "preview" ? "Preview " : "") + rel(tab.path);
    d.dataset.path = tab.path; d.dataset.kind = tab.kind;
    const b = document.createElement("button"); b.type = "button"; b.textContent = tabTitle(tab);
    b.addEventListener("click", () => { saveState(); show(tab); });
    const x = document.createElement("button"); x.type = "button"; x.className = "vs-x"; x.textContent = "×"; x.setAttribute("aria-label", "Close " + tabTitle(tab));
    x.addEventListener("click", () => { saveState(); closeTab(tab); });
    d.append(icon(tab.kind === "preview" ? "i-pv" : "i-" + iconOf(baseName(tab.path), false)), b, x);
    return d;
  }));
  updateActs();
}

function openTab(path, kind = "file", focus = true) {
  let tab = tabs.find(x => x.path === path && x.kind === kind);
  if (!tab) { tab = { path, kind }; tabs.push(tab); dirty(); }
  if (focus) show(tab); else renderTabs();
  return tab;
}

function closeTab(tab) {
  const i = tabs.indexOf(tab);
  if (i < 0) return;
  tabs.splice(i, 1);
  dirty();
  if (current === tab) current = null;
  if (!current && tabs.length) return show(tabs[Math.min(i, tabs.length - 1)]);
  if (!tabs.length) { $("code").hidden = true; $("preview").hidden = true; $("empty").hidden = false; $("crumbs").textContent = ""; $("lang").textContent = ""; }
  renderTabs(); markTree();
}

// The editor actions: Open Preview and Open Preview to the Side on a Markdown or SVG source, Show Source on a preview.
function updateActs() {
  const src = !!current && current.kind === "file" && previewable(current.path);
  $("openpv").hidden = !src; $("openside").hidden = !src;
  $("showsrc").hidden = !(current && current.kind === "preview");
}
$("openpv").addEventListener("click", () => { saveState(); openTab(current.path, "preview"); });
$("openside").addEventListener("click", () => { saveState(); openSide(current.path); });
$("showsrc").addEventListener("click", () => openTab(current.path, "file"));
$("sidesrc").addEventListener("click", () => { if (side) { saveState(); openTab(side, "file"); } });
$("sideclose").addEventListener("click", () => closeSide());

function openSide(path) {
  side = path;
  $("sidebox").hidden = false; $("pane").classList.add("split");
  $("sidetitle").textContent = "Preview " + baseName(path);
  $("sidebox").title = "Preview " + rel(path);
  renderPreview(path, $("sidepv"));
  if (!current || current.path !== path || current.kind !== "file") openTab(path, "file");
  dirty();
}
function closeSide() {
  if (!side) return;
  side = null;
  $("sidebox").hidden = true; $("pane").classList.remove("split");
  freeBlobs($("sidepv")); $("sidepv").replaceChildren();
  dirty();
}

// livePreview renders the visible previews again a short time after an edit.
let liveTimer = 0;
function livePreview() {
  clearTimeout(liveTimer);
  liveTimer = setTimeout(() => {
    if (side) renderPreview(side, $("sidepv"));
    if (current && current.kind === "preview" && !$("preview").hidden) renderPreview(current.path, $("preview"));
  }, 250);
}

const blobUrls = new Map();
const freeBlobs = box => { for (const u of blobUrls.get(box) || []) URL.revokeObjectURL(u); blobUrls.set(box, []); };
const md = new cm.MarkdownIt({ html: true, linkify: false, typographer: false });

// sanitize keeps only the elements and attributes of an allow list in the parsed Markdown, so that
// no HTML in a page can run script: no script elements, no event attributes, and no URL with a
// scheme other than http, https and mailto. It removes an element of DROP with its content, and
// replaces another element that is not in KEEP with its content.
const KEEP = new Set(("a abbr b blockquote br caption code col colgroup dd del details div dl dt em figcaption figure h1 h2 h3 h4 h5 h6 " +
  "hr i img ins kbd li mark ol p pre q s samp small span strong sub summary sup table tbody td tfoot th thead tr tt u ul var").split(" "));
const DROP = new Set("script style iframe frame frameset object embed applet svg math template noscript form input button select textarea base meta link title head".split(" "));
const ATTRS = new Set("href src alt title width height align colspan rowspan start type open class lang dir".split(" "));
const safeURL = v => { const s = v.trim().replace(/[\u0000-\u0020]/g, ""); const m = s.match(/^([a-z][a-z0-9+.-]*):/i); return !m || /^(https?|mailto)$/i.test(m[1]); };
function sanitize(root) {
  for (const el of [...root.querySelectorAll("*")]) {
    const name = el.localName;
    if (DROP.has(name) || el.namespaceURI !== "http://www.w3.org/1999/xhtml") { el.remove(); continue; }
    if (!KEEP.has(name)) { el.replaceWith(...el.childNodes); continue; }
    for (const a of [...el.attributes]) {
      if (!ATTRS.has(a.name) || ((a.name === "href" || a.name === "src") && !safeURL(a.value))) el.removeAttribute(a.name);
    }
  }
}

function renderPreview(path, box = $("preview")) {
  pg.renders++;
  const top = box.scrollTop;
  freeBlobs(box);
  const urls = blobUrls.get(box);
  box.classList.remove("vs-media");
  if (!exists(path)) { box.textContent = "This file no longer exists."; return; }
  if (/\.svg$/i.test(path)) {
    box.classList.add("vs-media");
    const u = URL.createObjectURL(new Blob([S.fsys.readFile(path)], { type: "image/svg+xml" }));
    urls.push(u);
    const img = document.createElement("img"); img.src = u; img.alt = rel(path); img.dataset.path = rel(path);
    img.addEventListener("error", () => { const p = document.createElement("p"); p.className = "vs-empty"; p.textContent = "This SVG does not render. Check its source."; img.replaceWith(p); });
    box.replaceChildren(img);
    return;
  }
  const html = md.render(readText(path));
  const tpl = document.createElement("template");
  tpl.innerHTML = html;
  sanitize(tpl.content);
  const dir = path.slice(0, path.lastIndexOf("/"));
  for (const img of tpl.content.querySelectorAll("img")) {
    const src = img.getAttribute("src") || "";
    if (/^[a-z]+:/i.test(src)) continue;
    const target = src.startsWith("/") ? REPO + src : normalize(dir + "/" + src.split("#")[0]);
    if (exists(target)) {
      const u = URL.createObjectURL(new Blob([S.fsys.readFile(target)], { type: target.endsWith(".svg") ? "image/svg+xml" : "application/octet-stream" }));
      urls.push(u); img.src = u; img.dataset.path = rel(target);
    }
  }
  for (const a of tpl.content.querySelectorAll("a[href]")) {
    if (/^https?:/.test(a.getAttribute("href"))) { a.target = "_blank"; a.rel = "noopener noreferrer"; }
  }
  box.replaceChildren(tpl.content);
  box.scrollTop = top;
}
function normalize(p) {
  const out = [];
  for (const s of p.split("/")) { if (!s || s === ".") continue; if (s === "..") out.pop(); else out.push(s); }
  return "/" + out.join("/");
}

function show(tab) {
  current = tab;
  const path = tab.path;
  $("empty").hidden = true;
  const picture = tab.kind === "preview";
  $("crumbs").textContent = rel(path).split("/").join(" › ");
  $("lang").textContent = picture ? languageOf(path).name + " Preview" : languageOf(path).name;
  $("pick").value = tab.kind === "preview" ? "preview:" + path : path;
  if (picture) {
    $("code").hidden = true; $("preview").hidden = false;
    renderPreview(path, $("preview"));
  } else if (!exists(path) || isBinary(path)) {
    $("code").hidden = true; $("preview").hidden = false; $("preview").classList.remove("vs-media");
    $("preview").textContent = exists(path) ? "This file is binary." : "This file no longer exists.";
  } else {
    $("preview").hidden = true; $("code").hidden = false;
    if (!states.has(path)) states.set(path, makeState(path));
    view.setState(states.get(path));
  }
  renderTabs(); markTree();
  if (explorer) explorer.reveal(path);
  dirty();
}

// saveState keeps the state of the open editor, so a tab switch keeps its undo history.
const saveState = () => { if (current && current.kind === "file" && states.has(current.path) && !$("code").hidden) states.set(current.path, view.state); };

// reloadFiles replaces the text of each editor state whose file changed in the memfs, as an undoable change.
function reloadFiles() {
  saveState();
  const changed = [];
  for (const [path, st] of states) {
    if (!exists(path)) continue;
    const text = readText(path);
    if (text === st.doc.toString()) continue;
    const tr = st.update({ changes: { from: 0, to: st.doc.length, insert: text } });
    states.set(path, tr.state);
    changed.push(rel(path));
  }
  if (current) show(current);
  if (side) renderPreview(side, $("sidepv"));
  return changed;
}

// changes returns the files of /work that differ from the starter: M for changed, U for new.
function changes() {
  const m = new Map();
  for (const p of listFiles(WORK)) { const old = base.get(p); if (!old) m.set(p, "U"); else if (!same(old, S.fsys.readFile(p))) m.set(p, "M"); }
  return m;
}
const edited = p => { if (!exists(p)) return false; const old = base.get(p), d = S.fsys.readFile(p); return old ? !same(old, d) : d.length > 0; };

// moved updates the tabs, the editor states and the side preview after a rename or a move.
function moved(pairs) {
  saveState();
  const map = p => { for (const [a, b] of pairs) if (under(p, a)) return b + p.slice(a.length); return p; };
  for (const tab of tabs) tab.path = map(tab.path);
  for (const [p, st] of [...states]) {
    const q = map(p);
    if (q === p) continue;
    states.delete(p);
    const ref = refs.get(p); refs.delete(p); ref.path = q; refs.set(q, ref);
    states.set(q, st.update({ effects: fileConf.reconfigure(fileExt(q)) }).state);
  }
  if (side) { side = map(side); $("sidetitle").textContent = "Preview " + baseName(side); }
  if (current) show(current); else renderTabs();
  dirty();
}
// removed closes the tabs of deleted files, with a note when a closed file had edits.
function removed(paths, lost) {
  saveState();
  const gone = p => paths.some(a => under(p, a));
  const closed = tabs.filter(t => gone(t.path));
  const lostOpen = [...new Set(closed.filter(t => lost.includes(t.path)).map(t => rel(t.path)))];
  if (side && gone(side)) closeSide();
  for (const t of closed) closeTab(t);
  for (const p of [...states.keys()]) if (gone(p)) { states.delete(p); refs.delete(p); }
  if (lostOpen.length) note(`Closed ${lostOpen.join(", ")}: the deleted ${lostOpen.length === 1 ? "file had edits that are" : "files had edits that are"} now lost.`);
  else if (closed.length) note(`Closed ${closed.length === 1 ? "the tab" : closed.length + " tabs"} of the deleted ${paths.length === 1 ? "item" : "items"}.`);
}

function renderPick() {
  const files = listFiles(WORK);
  $("pick").replaceChildren(...files.flatMap(p => {
    const o = [new Option(rel(p), p)];
    if (previewable(p)) o.push(new Option("Preview " + rel(p), "preview:" + p));
    return o;
  }));
  if (current) $("pick").value = current.kind === "preview" ? "preview:" + current.path : current.path;
}
function renderTree() {
  explorer.render();
  renderPick();
}

// markTree marks each file that differs from the starter: M for changed, U for new.
function markTree() { if (explorer) explorer.decorate(); }

// Messages and the confirm dialog, inside the page.
let toastTimer = 0;
function note(text) {
  pg.notes.push(text);
  const t = $("toast");
  t.textContent = text; t.hidden = false;
  clearTimeout(toastTimer); toastTimer = setTimeout(() => (t.hidden = true), 5000);
}
function ask({ title, text = "", list = [], ok = "OK", danger = false }) {
  const d = $("dlg");
  $("dlgt").textContent = title; $("dlgm").textContent = text;
  $("dlgl").replaceChildren(...list.map(x => { const li = document.createElement("li"); li.textContent = x; return li; }));
  $("dlgok").textContent = ok; $("dlgok").classList.toggle("danger", danger);
  d.returnValue = "";
  const back = document.activeElement;
  d.showModal();
  $("dlgok").focus();
  return new Promise(res => d.addEventListener("close", () => { res(d.returnValue === "ok"); if (back && back.isConnected) back.focus({ preventScroll: true }); }, { once: true }));
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
    fsys: S.fsys, root: WORK, tree: $("tree"), side: $("side"), menu: $("menu"),
    open: openFromTree, changes, edited, moved, removed, note, ask,
    changed: () => { renderPick(); dirty(); if (side) livePreview(); },
    busy: () => pg.busy,
    canSide: () => !narrow.matches,
    reset: () => resetSample(),
    showTree: () => { if (narrow.matches) setDrawer(true); },
    copied: t => (pg.copied = t),
    downloaded: (name, blob) => (pg.download = { name, size: blob.size, blob }),
  });
  pg.explorer = explorer;
}

// At 640 px and less the explorer is a drawer, with the dropdown and an actions button beside it.
function setDrawer(open) {
  $("vs").classList.toggle("ex-open", open);
  $("files").setAttribute("aria-expanded", String(open));
  if (open) { if (current) explorer.reveal(current.path); $("tree").focus({ preventScroll: true }); }
}
$("files").addEventListener("click", () => setDrawer(!$("vs").classList.contains("ex-open")));
$("scrim").addEventListener("click", () => setDrawer(false));
$("side").addEventListener("keydown", e => { if (e.key === "Escape" && narrow.matches && $("vs").classList.contains("ex-open") && $("menu").hidden) setDrawer(false); });
const actionsMenu = (x, y, path) => explorer.openMenu(x, y, [path || WORK], false);
$("more").addEventListener("click", () => { const b = $("more").getBoundingClientRect(); actionsMenu(b.right - 240, b.bottom, current && current.path); });
longPress($("tabs"), (x, y, e) => { const t = e.target.closest(".vs-tab"); if (t) actionsMenu(x, y, t.dataset.path); });

// Storage of the workspace: the files and folders of /work and the open tabs, in IndexedDB, else in the Cache API.
// Without storage the page works the same, and changes last until a reload.
const DB = "docgen-playground", STORE = "workspace", KEY = "work:" + location.pathname;
let sampleKey = "", sampleBundle = null, saveTimer = 0, store = null;
function serialize() {
  const dirs = [], files = [], parts = [];
  let off = 0;
  const walk = (e, p) => {
    if (!e.children.size) dirs.push(p);
    for (const [n, c] of e.children) {
      const q = p + "/" + n;
      if (c.dir) walk(c, q);
      else { files.push([q, off, c.size, c.mtime, c.mode]); parts.push(c.data.subarray(0, c.size)); off += c.size; }
    }
  };
  walk(S.fsys.lookup(WORK), WORK);
  const t = current ? tabs.indexOf(current) : -1;
  const head = enc.encode(JSON.stringify({ v: 1, sample: sampleKey, dirs, files, tabs: tabs.map(x => [x.path, x.kind]), current: t, side }));
  const out = new Uint8Array(4 + head.length + off);
  new DataView(out.buffer).setUint32(0, head.length, true);
  out.set(head, 4);
  let o = 4 + head.length;
  for (const d of parts) { out.set(d, o); o += d.length; }
  return out;
}
function parse(bytes) {
  const hl = new DataView(bytes.buffer, bytes.byteOffset).getUint32(0, true);
  const head = JSON.parse(dec.decode(bytes.subarray(4, 4 + hl)));
  head.data = bytes.subarray(4 + hl);
  return head;
}
const idbReq = r => new Promise((res, rej) => { r.onsuccess = () => res(r.result); r.onerror = () => rej(r.error); });
async function openStore() {
  try {
    const open = indexedDB.open(DB, 1);
    open.onupgradeneeded = () => open.result.createObjectStore(STORE);
    const db = await idbReq(open);
    const tx = m => db.transaction(STORE, m).objectStore(STORE);
    await idbReq(tx("readonly").get(KEY));
    return { kind: "indexeddb", get: async () => (await idbReq(tx("readonly").get(KEY))) || null, put: v => idbReq(tx("readwrite").put(v, KEY)), clear: () => idbReq(tx("readwrite").delete(KEY)) };
  } catch (e) { pg.log.push("no IndexedDB: " + e); }
  try {
    const c = await caches.open("docgen-playground-workspace");
    const url = new URL("workspace.bin", location.href).href;
    return { kind: "cache", get: async () => { const r = await c.match(url); return r ? new Uint8Array(await r.arrayBuffer()) : null; }, put: v => c.put(url, new Response(v)), clear: () => c.delete(url) };
  } catch (e) { pg.log.push("no Cache API for the workspace: " + e); }
  return null;
}
function dirty() {
  if (!pg.ready || !store) return;
  clearTimeout(saveTimer);
  saveTimer = setTimeout(saveNow, 300);
}
async function saveNow() {
  clearTimeout(saveTimer);
  if (!store || !pg.ready) return;
  try { saveState(); await store.put(serialize()); pg.saves++; }
  catch (e) { pg.log.push("save: " + e); if (!pg.saveFailed) { pg.saveFailed = true; note("The browser did not save the workspace. Your changes last until you reload the page."); } }
}
addEventListener("pagehide", () => { if (saveTimer) saveNow(); });
document.addEventListener("visibilitychange", () => { if (document.hidden && saveTimer) saveNow(); });
pg.saveNow = saveNow;
function restore(rec) {
  const w = S.fsys.lookup(WORK);
  w.children.clear();
  for (const d of rec.dirs) if (d.startsWith(WORK + "/")) S.fsys.mkdirAll(d);
  for (const [p, off, len, mtime, mode] of rec.files) if (p.startsWith(WORK + "/")) S.fsys.writeFile(p, rec.data.slice(off, off + len), mtime).mode = mode;
}
function defaultTabs() {
  openTab(REPO + "/machine/machine.go", "file", false);
  openTab(REPO + "/docs/lamp.md", "file");
}
async function resetSample() {
  if (pg.busy) return note("Wait for the run to end before you reset the sample.");
  clearTimeout(saveTimer);
  closeSide();
  S.fsys.lookup(WORK).children.clear();
  loadBundle(S.fsys, sampleBundle);
  states.clear(); refs.clear(); tabs = []; current = null;
  try { if (store) await store.clear(); } catch (e) { pg.log.push("clear: " + e); }
  renderTree();
  defaultTabs();
  note("The sample has its starter files again.");
}
pg.resetSample = resetSample;

// Output pane.
const out = $("out");
function term(text, cls) {
  if (!text) return;
  const stick = out.scrollTop + out.clientHeight >= out.scrollHeight - 4;
  const last = out.lastChild;
  if (last && last.nodeType === 1 && last.className === (cls || "") && !cls?.startsWith("cmdl")) last.textContent += text;
  else { const s = document.createElement("span"); s.className = cls || ""; s.textContent = text; out.append(s); }
  while (out.childNodes.length > 2000) out.firstChild.remove();
  if (stick) out.scrollTop = out.scrollHeight;
}
$("clear").addEventListener("click", () => out.replaceChildren());
pg.term = () => out.textContent;

// Runs, status line, busy state and Stop.
let running = null, lastTool = "", spawns = 0, ticker = 0;
const buttons = ["check", "update", "run"];
function setBusy(b, label) {
  pg.busy = b;
  for (const id of buttons) $(id).disabled = b;
  $("stop").disabled = !b;
  $("status").classList.toggle("busy", b);
  clearInterval(ticker);
  if (b) {
    const t0 = performance.now();
    const tick = () => {
      $("step").textContent = `${label}${lastTool ? " · " + lastTool : ""}`;
      $("elapsed").textContent = ((performance.now() - t0) / 1000).toFixed(1) + " s";
      const live = S.kernel.stats.live;
      $("info").textContent = `${live} ${live === 1 ? "process" : "processes"}, ${spawns} started`;
    };
    tick(); ticker = setInterval(tick, 100);
  }
}

async function run(label, argv0, argv, cwd = REPO) {
  if (pg.busy) return null;
  saveState();
  lastTool = ""; spawns = 0;
  setBusy(true, label);
  term(`$ ${label}\n`, "cmdl");
  const d1 = new TextDecoder(), d2 = new TextDecoder();
  const before = snapshot();
  const w0 = Date.now();
  const { proc, done } = S.start(argv0, argv, cwd, env, b => term(d1.decode(b, { stream: true }), ""), b => term(d2.decode(b, { stream: true }), "err"));
  running = proc;
  const r = await done;
  running = null;
  const status = describe(r.code);
  term(`[${status}${r.reason === "limit" ? ", process limit" : ""} in ${(r.ms / 1000).toFixed(2)} s]\n`, r.code === 0 ? "ok" : "bad");
  const now = snapshot();
  const written = [...now].filter(([p, d]) => !before.has(p) || !same(before.get(p), d)).map(([p]) => rel(p));
  const reloaded = reloadFiles();
  renderTree();
  if (current) show(current);
  const step = { label, code: r.code, status, ms: Math.round(r.ms), wall: [w0, Date.now()], spawns, changed: written, reloaded };
  pg.steps.push(step);
  setBusy(false);
  $("step").textContent = `${label}: ${status}`;
  $("elapsed").textContent = (r.ms / 1000).toFixed(2) + " s";
  $("info").textContent = written.length ? "changed " + written.join(", ") : `${spawns} ${spawns === 1 ? "process" : "processes"}`;
  return step;
}
pg.run = run;
const docgen = sub => run(`docgen ${sub}`, "/usr/local/bin/docgen", ["docgen", sub]);
$("check").addEventListener("click", () => docgen("check"));
$("update").addEventListener("click", () => docgen("update"));
$("cmdform").addEventListener("submit", e => {
  e.preventDefault();
  const c = $("cmd").value.trim();
  if (c) run(c, "/bin/sh", ["sh", "-c", c]);
});
$("stop").addEventListener("click", () => {
  if (!running) return;
  const n = S.kernel.killTree(running.pid);
  term(`[Stop: killed ${n} processes]\n`, "bad");
});
$("pick").addEventListener("change", () => {
  saveState();
  const v = $("pick").value;
  if (v.startsWith("preview:")) openTab(v.slice(8), "preview"); else openTab(v);
});
narrow.addEventListener("change", () => { setDrawer(false); explorer.closeMenu(); saveState(); for (const [p, s] of states) states.set(p, s.update({ effects: wrap.reconfigure(narrow.matches ? cm.EditorView.lineWrapping : []) }).state); if (current) show(current); });

// A file dropped outside the explorer and the editor does nothing, so that the browser does not leave the page to show it.
for (const n of ["dragover", "drop"]) addEventListener(n, e => { if (!e.defaultPrevented && e.dataTransfer && [...e.dataTransfer.types].includes("Files")) e.preventDefault(); });

// Hooks for the browser test.
pg.open = (path, kind = "file") => { saveState(); return openTab(REPO + "/" + path, kind); };
pg.replace = (path, from, to) => {
  pg.open(path);
  const doc = view.state.doc.toString(), i = doc.indexOf(from);
  if (i < 0) throw new Error(`${path}: no ${from}`);
  view.dispatch({ changes: { from: i, to: i + from.length, insert: to } });
  return true;
};
pg.text = path => readText(REPO + "/" + path);
pg.editorText = () => view.state.doc.toString();
pg.cursorEnd = () => { view.dispatch({ selection: { anchor: view.state.doc.length } }); view.focus(); };
pg.previewText = () => $("preview").textContent;
const imgs = box => [...box.querySelectorAll("img")].map(i => ({ path: i.dataset.path, w: i.naturalWidth, h: i.naturalHeight, complete: i.complete, src: i.src }));
pg.previewImages = () => imgs($("preview"));
pg.sideImages = () => imgs($("sidepv"));
pg.sideText = () => (side ? $("sidepv").textContent : null);
pg.tabs = () => tabs.map(t => ({ path: rel(t.path), kind: t.kind, current: t === current }));
pg.current = () => current && { path: rel(current.path), kind: current.kind, code: !$("code").hidden, lang: $("lang").textContent };
pg.side = () => side && rel(side);
pg.exists = path => exists(REPO + "/" + path);
pg.isDir = path => { const e = S.fsys.lookup(REPO + "/" + path); return !!e && e.dir; };
pg.marks = () => Object.fromEntries([...changes()].map(([p, m]) => [rel(p), m]));

// Start.
try {
  const T0 = performance.now();
  store = await openStore();
  pg.storage = store ? store.kind : "none";
  if (q.has("clearcache")) { try { await caches.delete(CACHE); } catch {} try { if (store) await store.clear(); } catch {} try { localStorage.removeItem("pg-explorer-open"); } catch {} }
  const a = await loadAssets();
  pg.load = { ms: a.ms, hits: a.hits, files: a.files, zbytes: a.zbytes, bytes: a.bytes, perFile: a.perFile, navToAssets: Math.round(T0) };
  makeSession(a.tools);
  const tb = performance.now();
  for (const b of ["root", "gocache", "sample"]) loadBundle(S.fsys, a.bundles[b], b === "gocache" ? Date.now() : 0);
  pg.load.bundlesMs = Math.round(performance.now() - tb);
  base = snapshot();
  sampleBundle = a.bundles.sample; sampleKey = a.sha.sample;
  let rec = null;
  try { const b = store && await store.get(); rec = b && parse(b); } catch (e) { pg.log.push("restore: " + e); }
  if (rec && rec.sample !== sampleKey) { rec = null; pg.restoreSkipped = "sample changed"; }
  if (rec) restore(rec);
  pg.restored = !!rec;
  view = new cm.EditorView({ parent: $("code") });
  makeExplorer();
  renderTree();
  if (rec) {
    for (const [p, kind] of rec.tabs) if (exists(p) && (kind === "file" || previewable(p))) openTab(p, kind, false);
    const t = rec.current >= 0 && rec.tabs[rec.current];
    const cur = t && tabs.find(x => x.path === t[0] && x.kind === t[1]);
    if (cur) show(cur); else if (tabs.length) show(tabs[0]); else { $("code").hidden = true; $("empty").hidden = false; renderTabs(); }
    if (rec.side && exists(rec.side)) openSide(rec.side);
  } else defaultTabs();
  $("loader").hidden = true;
  for (const id of buttons) $(id).disabled = false;
  pg.load.total = Math.round(performance.now());
  $("step").textContent = "Ready";
  $("elapsed").textContent = `loaded in ${(pg.load.total / 1000).toFixed(1)} s`;
  $("info").textContent = (a.hits === a.files ? "tools from the browser cache" : `${a.files - a.hits} files fetched`) + (rec ? ", workspace restored" : "");
  pg.ready = true;
  if (!store) note("This browser has no storage for the page. Your changes last until you reload.");
} catch (e) {
  pg.errors.push("load: " + String(e && e.stack || e));
  const li = document.createElement("li"); li.className = "lerr"; li.textContent = "Load failed: " + String(e && e.message || e);
  $("lfiles").append(li);
}
