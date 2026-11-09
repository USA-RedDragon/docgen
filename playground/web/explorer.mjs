// The explorer of the playground page: a tree of one memfs folder, after the Explorer of VS Code.
// It does the file operations (new, rename, move, duplicate, delete, upload, download, copy path) on the memfs
// and gives the changed paths to the page, which then updates the tabs, the badges and the storage.
const NS = "http://www.w3.org/2000/svg";
const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });
const byName = (a, b) => collator.compare(a, b) || (a < b ? -1 : a > b ? 1 : 0);
export const parentOf = p => p.slice(0, p.lastIndexOf("/"));
export const baseName = p => p.slice(p.lastIndexOf("/") + 1);
export const under = (p, dir) => p === dir || p.startsWith(dir + "/");
export const previewable = p => /\.(md|markdown|svg)$/i.test(p);

const icons = { go: "go", md: "md", markdown: "md", yaml: "yaml", yml: "yaml", json: "json", toml: "toml", svg: "svg" };
/** iconOf returns the id of the icon symbol of a file or folder name. */
export function iconOf(name, dir, open) {
  if (dir) return open ? "fo" : "fc";
  if (name === "go.mod" || name === "go.sum" || name === "go.work") return "mod";
  const dot = name.lastIndexOf(".");
  return (dot > 0 && icons[name.slice(dot + 1).toLowerCase()]) || "plain";
}
/** icon returns an svg element that shows the symbol #id. */
export function icon(id, cls = "ex-icon") {
  const s = document.createElementNS(NS, "svg");
  s.setAttribute("class", cls); s.setAttribute("aria-hidden", "true");
  const u = document.createElementNS(NS, "use"); u.setAttribute("href", "#" + id);
  s.append(u);
  return s;
}

/** longPress calls fn(x, y, event) when a touch or pen holds still on el for 500 ms. */
export function longPress(el, fn) {
  let t = 0, x = 0, y = 0, at = 0;
  const stop = () => clearTimeout(t);
  el.addEventListener("pointerdown", e => {
    if (e.pointerType === "mouse") return;
    stop(); x = e.clientX; y = e.clientY;
    t = setTimeout(() => { at = Date.now(); fn(x, y, e); }, 500);
  });
  el.addEventListener("pointermove", e => { if (Math.hypot(e.clientX - x, e.clientY - y) > 10) stop(); });
  for (const n of ["pointerup", "pointercancel", "pointerleave"]) el.addEventListener(n, stop);
  const recent = () => Date.now() - at < 800;
  el.addEventListener("contextmenu", e => { if (recent()) { e.preventDefault(); e.stopImmediatePropagation(); } }, true);
  el.addEventListener("click", e => { if (recent()) { e.preventDefault(); e.stopImmediatePropagation(); } }, true);
}

const crcTable = (() => { const t = new Uint32Array(256); for (let n = 0; n < 256; n++) { let c = n; for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1; t[n] = c >>> 0; } return t; })();
const crc32 = d => { let c = 0xffffffff; for (let i = 0; i < d.length; i++) c = crcTable[(c ^ d[i]) & 255] ^ (c >>> 8); return (c ^ 0xffffffff) >>> 0; };
/** zip returns a zip archive (stored, no compression) of entries [[name, bytes or null for a folder]]. */
function zip(entries) {
  const enc = new TextEncoder(), parts = [], central = [];
  const d = new Date(), time = (d.getHours() << 11) | (d.getMinutes() << 5) | (d.getSeconds() >> 1);
  const date = ((d.getFullYear() - 1980) << 9) | ((d.getMonth() + 1) << 5) | d.getDate();
  let off = 0;
  for (const [n, data] of entries) {
    const name = enc.encode(data ? n : n + "/"), body = data || new Uint8Array(0), crc = crc32(body);
    const h = new DataView(new ArrayBuffer(30));
    h.setUint32(0, 0x04034b50, true); h.setUint16(4, 20, true); h.setUint16(6, 0x800, true); h.setUint16(10, time, true); h.setUint16(12, date, true);
    h.setUint32(14, crc, true); h.setUint32(18, body.length, true); h.setUint32(22, body.length, true); h.setUint16(26, name.length, true);
    const c = new DataView(new ArrayBuffer(46));
    c.setUint32(0, 0x02014b50, true); c.setUint16(4, 0x31e, true); c.setUint16(6, 20, true); c.setUint16(8, 0x800, true); c.setUint16(12, time, true); c.setUint16(14, date, true);
    c.setUint32(16, crc, true); c.setUint32(20, body.length, true); c.setUint32(24, body.length, true); c.setUint16(28, name.length, true);
    c.setUint32(38, data ? 0o100644 << 16 : (0o40755 << 16) | 0x10, true); c.setUint32(42, off, true);
    parts.push(new Uint8Array(h.buffer), name, body); central.push(new Uint8Array(c.buffer), name);
    off += 30 + name.length + body.length;
  }
  const size = central.reduce((a, b) => a + b.length, 0);
  const e = new DataView(new ArrayBuffer(22));
  e.setUint32(0, 0x06054b50, true); e.setUint16(8, entries.length, true); e.setUint16(10, entries.length, true); e.setUint32(12, size, true); e.setUint32(16, off, true);
  return new Blob([...parts, ...central, new Uint8Array(e.buffer)], { type: "application/zip" });
}

/**
 * createExplorer shows the folder o.root of o.fsys in o.tree. The page gives these functions:
 * open(path, how) opens a file ("file", "preview" or "side"); changes() returns a Map of path to "M" or "U";
 * edited(path) returns true when a file differs from the starter; moved(pairs) and removed(paths, edited) run after file operations;
 * changed() is called after each change of the memfs; busy() returns true during a run; note(text) shows a message;
 * ask({title, text, list, ok}) resolves to true on OK; reset() restores the sample; showTree() makes the tree visible.
 */
export function createExplorer(o) {
  const { fsys, root, tree } = o;
  const menu = o.menu;
  let expanded = new Set(), selected = new Set(), focus = null, anchor = null, edit = null, rows = [], rowOf = new Map(), uid = 0;
  let drag = null, dropDir = null, hoverDir = null, hoverTimer = 0, typed = "", typedAt = 0;
  const KEY = "pg-explorer-open";
  try { const v = JSON.parse(localStorage.getItem(KEY)); if (Array.isArray(v)) expanded = new Set(v); else expanded = null; } catch { expanded = null; }
  if (!expanded) { expanded = new Set(); const walk = p => { const e = fsys.lookup(p); for (const [n, c] of e.children) if (c.dir) { expanded.add(p + "/" + n); walk(p + "/" + n); } }; walk(root); }
  const saveOpen = () => { try { localStorage.setItem(KEY, JSON.stringify([...expanded])); } catch {} };

  const ent = p => fsys.lookup(p);
  const isDir = p => { const e = ent(p); return !!e && e.dir; };
  const rel = p => p.slice(root.length + 1);
  const topLevel = ps => ps.filter(p => !ps.some(q => q !== p && under(p, q)));
  const filesUnder = p => { const out = []; const walk = (q, e) => { if (!e.dir) return out.push(q); for (const [n, c] of e.children) walk(q + "/" + n, c); }; const e = ent(p); if (e) walk(p, e); return out; };
  const guard = () => { if (o.busy()) { o.note("Wait for the run to end before you change files."); return false; } return true; };

  function nameError(raw, dir, self) {
    const n = raw.trim();
    if (!n) return "A file or folder name must be provided.";
    if (n.includes("/")) return "A name cannot contain a slash (/).";
    if (n === "." || n === "..") return `The name ${n} is not valid as a file or folder name.`;
    if (n.includes("\0")) return "A name cannot contain a null character.";
    const d = ent(dir);
    if (n !== self && d && d.dir && d.children.has(n)) return `A file or folder ${n} already exists in this folder. Choose a different name.`;
    return "";
  }

  // Rendering.
  function render() {
    const changes = o.changes();
    const dirty = new Set();
    for (const p of changes.keys()) for (let d = parentOf(p); d.startsWith(root + "/"); d = parentOf(d)) dirty.add(d);
    rows = []; rowOf = new Map();
    const frag = document.createDocumentFragment();
    let editInput = null;
    const walk = (dir, depth) => {
      const e = ent(dir);
      if (!e || !e.dir) return;
      if (edit && edit.kind !== "rename" && edit.parent === dir) { const r = newRow(depth); frag.append(r.el); editInput = r.inp; }
      const kids = [...e.children].sort(([a, x], [b, y]) => (y.dir - x.dir) || byName(a, b));
      for (const [name, c] of kids) {
        const p = dir + "/" + name;
        const r = { path: p, name, dir: c.dir, depth };
        r.el = rowEl(r);
        if (edit && edit.kind === "rename" && edit.path === p) editInput = renameInput(r);
        rows.push(r); rowOf.set(p, r); frag.append(r.el);
        if (c.dir && expanded.has(p)) walk(p, depth + 1);
      }
    };
    walk(root, 0);
    tree.replaceChildren(frag);
    for (const p of [...selected]) if (!ent(p)) selected.delete(p);
    if (focus && !ent(focus)) focus = null;
    decorate(changes, dirty);
    paint();
    if (dropDir) setDrop(dropDir);
    if (editInput) { editInput.focus(); editInput.dispatchEvent(new Event("select-name")); }
  }

  function rowEl(r) {
    const el = document.createElement("div");
    el.className = "ex-row"; el.id = "ex-" + ++uid;
    el.setAttribute("role", "treeitem"); el.setAttribute("aria-level", r.depth + 1);
    el.dataset.path = r.path; el.draggable = true; el.title = rel(r.path);
    el.style.setProperty("--d", r.depth);
    const tw = document.createElement("span"); tw.className = "ex-tw";
    if (r.dir) { el.setAttribute("aria-expanded", String(expanded.has(r.path))); tw.append(icon("t-chev", "")); }
    const ind = document.createElement("span"); ind.className = "ex-ind";
    const name = document.createElement("span"); name.className = "ex-name"; name.textContent = r.name;
    const badge = document.createElement("span"); badge.className = "ex-badge"; badge.setAttribute("aria-hidden", "true");
    el.append(ind, tw, icon("i-" + iconOf(r.name, r.dir, expanded.has(r.path))), name, badge);
    return el;
  }

  function inputBox(value, check, done) {
    const wrap = document.createElement("span"); wrap.className = "ex-edit";
    const inp = document.createElement("input"); inp.className = "ex-input"; inp.value = value; inp.spellcheck = false; inp.autocomplete = "off";
    inp.setAttribute("aria-label", "Name");
    const msg = document.createElement("div"); msg.className = "ex-msg"; msg.setAttribute("role", "alert"); msg.hidden = true;
    const upd = () => { const m = check(inp.value); msg.textContent = m; msg.hidden = !m; inp.classList.toggle("bad", !!m); inp.setAttribute("aria-invalid", String(!!m)); return m; };
    let ended = false;
    const end = ok => { if (ended) return; ended = true; done(ok ? inp.value.trim() : null); };
    inp.addEventListener("input", upd);
    inp.addEventListener("keydown", e => {
      e.stopPropagation();
      if (e.key === "Enter") { e.preventDefault(); if (!upd()) end(true); }
      else if (e.key === "Escape") { e.preventDefault(); end(false); }
    });
    inp.addEventListener("blur", () => setTimeout(() => { if (!ended && document.activeElement !== inp) end(!upd()); }, 0));
    for (const n of ["click", "dblclick", "pointerdown", "mousedown", "contextmenu", "dragstart"]) inp.addEventListener(n, e => e.stopPropagation());
    inp.addEventListener("select-name", () => { const dot = value.lastIndexOf("."); inp.setSelectionRange(0, dot > 0 && !edit?.dir ? dot : value.length); });
    wrap.append(inp, msg);
    return { wrap, inp };
  }

  function renameInput(r) {
    const { wrap, inp } = inputBox(r.name, v => nameError(v, parentOf(r.path), r.name), v => {
      const path = edit.path; edit = null;
      if (v && v !== r.name) rename(path, v); else render();
      tree.focus({ preventScroll: true });
    });
    r.el.querySelector(".ex-name").replaceWith(wrap);
    r.el.draggable = false;
    return inp;
  }

  function newRow(depth) {
    const el = document.createElement("div");
    el.className = "ex-row ex-new"; el.style.setProperty("--d", depth);
    const ind = document.createElement("span"); ind.className = "ex-ind";
    const tw = document.createElement("span"); tw.className = "ex-tw";
    const { kind, parent } = edit;
    const { wrap, inp } = inputBox("", v => nameError(v, parent, null), v => {
      edit = null;
      if (v) create(kind, parent + "/" + v); else render();
      tree.focus({ preventScroll: true });
    });
    el.append(ind, tw, icon(kind === "folder" ? "i-fc" : "i-plain"), wrap);
    return { el, inp };
  }

  /** decorate sets the M, U and folder marks without a new render. */
  function decorate(changes = o.changes(), dirty) {
    if (!dirty) { dirty = new Set(); for (const p of changes.keys()) for (let d = parentOf(p); d.startsWith(root + "/"); d = parentOf(d)) dirty.add(d); }
    for (const r of rows) {
      const m = r.dir ? "" : changes.get(r.path) || "";
      const dd = r.dir && dirty.has(r.path);
      r.el.classList.toggle("ex-M", m === "M"); r.el.classList.toggle("ex-U", m === "U"); r.el.classList.toggle("ex-dirty", dd);
      r.el.lastChild.textContent = m || (dd ? "●" : "");
      r.el.lastChild.title = m === "M" ? "Modified" : m === "U" ? "New" : dd ? "Contains changes" : "";
    }
  }

  function paint() {
    let fid = "";
    for (const r of rows) {
      const s = selected.has(r.path), f = focus === r.path;
      r.el.classList.toggle("sel", s); r.el.classList.toggle("foc", f);
      r.el.setAttribute("aria-selected", String(s));
      if (f) fid = r.el.id;
    }
    if (fid) tree.setAttribute("aria-activedescendant", fid); else tree.removeAttribute("aria-activedescendant");
  }
  const scrollTo = p => { const r = rowOf.get(p); if (r) r.el.scrollIntoView({ block: "nearest" }); };

  function setOpen(p, open) {
    if (open === expanded.has(p)) return;
    if (open) expanded.add(p); else expanded.delete(p);
    saveOpen(); render();
  }
  function expandTo(p) {
    let changed = false;
    for (let d = parentOf(p); d.startsWith(root + "/"); d = parentOf(d)) if (!expanded.has(d)) { expanded.add(d); changed = true; }
    if (changed) saveOpen();
    return changed;
  }

  /** reveal expands the folders above p, selects p and scrolls it into view. */
  function reveal(p) {
    if (!ent(p) || edit) return;
    const changed = expandTo(p);
    selected = new Set([p]); focus = anchor = p;
    if (changed) render(); else paint();
    scrollTo(p);
  }

  function collapseAll() { expanded.clear(); saveOpen(); render(); }

  // Selection.
  const indexOf = p => rows.findIndex(r => r.path === p);
  function rangeTo(p) {
    const a = indexOf(anchor), b = indexOf(p);
    if (a < 0 || b < 0) { selected = new Set([p]); anchor = p; return; }
    selected = new Set(rows.slice(Math.min(a, b), Math.max(a, b) + 1).map(r => r.path));
  }
  const targets = () => { const s = topLevel([...selected]); return s.length ? s : focus ? [focus] : [root]; };
  const folderFor = p => (!p || p === root ? root : isDir(p) ? p : parentOf(p));

  // Operations.
  function startNew(kind, dir = folderFor(focus)) {
    if (!guard()) return;
    o.showTree();
    if (dir !== root) { expandTo(dir + "/x"); expanded.add(dir); saveOpen(); }
    edit = { kind, parent: dir, dir: kind === "folder" };
    render();
  }
  function startRename(p) {
    if (!p || p === root || !ent(p) || !guard()) return;
    o.showTree();
    expandTo(p);
    edit = { kind: "rename", path: p, dir: isDir(p) };
    selected = new Set([p]); focus = anchor = p;
    render();
  }
  function mapPaths(pairs) {
    const map = p => { for (const [a, b] of pairs) if (under(p, a)) return b + p.slice(a.length); return p; };
    expanded = new Set([...expanded].map(map)); saveOpen();
    selected = new Set([...selected].map(map));
    if (focus) focus = map(focus);
    if (anchor) anchor = map(anchor);
  }
  function moveEntry(src, dst) {
    const a = ent(parentOf(src)), b = ent(parentOf(dst)), e = a.children.get(baseName(src));
    a.children.delete(baseName(src)); b.children.set(baseName(dst), e);
    e.mtime = Date.now();
  }
  function rename(p, name) {
    if (!guard()) return render();
    const err = nameError(name, parentOf(p), baseName(p));
    if (err) { o.note(err); return render(); }
    const q = parentOf(p) + "/" + name;
    moveEntry(p, q);
    mapPaths([[p, q]]);
    selected = new Set([q]); focus = anchor = q;
    o.moved([[p, q]]);
    render(); o.changed();
  }
  function create(kind, p) {
    if (!guard()) return render();
    const err = nameError(baseName(p), parentOf(p), null);
    if (err) { o.note(err); return render(); }
    if (kind === "folder") { fsys.mkdirAll(p); expanded.add(p); saveOpen(); } else fsys.writeFile(p, new Uint8Array(0));
    selected = new Set([p]); focus = anchor = p;
    render(); scrollTo(p); o.changed();
    if (kind !== "folder") o.open(p, "file");
  }
  function copyEntry(e) {
    const c = fsys.newEntry(e.dir);
    c.mode = e.mode;
    if (e.dir) for (const [n, k] of e.children) c.children.set(n, copyEntry(k));
    else { c.data = e.data.slice(0, e.size); c.size = e.size; }
    return c;
  }
  function copyName(dir, name, isFolder) {
    const d = ent(dir), dot = isFolder ? -1 : name.lastIndexOf(".");
    const stem = dot > 0 ? name.slice(0, dot) : name, ext = dot > 0 ? name.slice(dot) : "";
    for (let i = 1; ; i++) { const n = `${stem} copy${i > 1 ? " " + i : ""}${ext}`; if (!d.children.has(n)) return n; }
  }
  function duplicate(ps) {
    if (!guard()) return;
    const made = [];
    for (const p of topLevel(ps)) {
      if (p === root) continue;
      const dir = parentOf(p), e = ent(p), n = copyName(dir, baseName(p), e.dir);
      ent(dir).children.set(n, copyEntry(e));
      made.push(dir + "/" + n);
    }
    if (!made.length) return;
    selected = new Set(made); focus = anchor = made.at(-1);
    render(); scrollTo(focus); o.changed();
  }
  async function remove(ps) {
    ps = topLevel(ps).filter(p => p !== root && ent(p));
    if (!ps.length || !guard()) return;
    const lost = ps.flatMap(filesUnder).filter(o.edited);
    const folders = ps.filter(isDir).length;
    const ok = await o.ask({
      title: ps.length === 1 ? `Delete ${baseName(ps[0])}?` : `Delete ${ps.length} items?`,
      text: (folders ? "The contents of folders are deleted too. " : "") + (lost.length ? `These files have edits that will be lost: ${lost.map(rel).join(", ")}. ` : "") + "You cannot undo this.",
      list: ps.length > 1 ? ps.map(rel) : [], ok: "Delete", danger: true,
    });
    if (!ok || !guard()) return tree.focus({ preventScroll: true });
    const next = rows.slice(indexOf(ps.at(-1)) + 1).find(r => !ps.some(p => under(r.path, p))) || rows.slice(0, indexOf(ps[0])).reverse().find(r => !ps.some(p => under(r.path, p)));
    for (const p of ps) ent(parentOf(p)).children.delete(baseName(p));
    for (const q of [...expanded]) if (ps.some(p => under(q, p))) expanded.delete(q);
    saveOpen();
    selected = new Set(next ? [next.path] : []); focus = anchor = next ? next.path : null;
    o.removed(ps, lost);
    render(); o.changed();
    tree.focus({ preventScroll: true });
  }
  /** moveTo moves ps into the folder dir. It returns an error text, or "" when it moved the files. */
  function moveTo(ps, dir) {
    ps = topLevel(ps).filter(p => p !== root);
    if (!guard()) return "busy";
    for (const p of ps) if (under(dir, p)) return `Cannot move ${baseName(p)} into itself.`;
    ps = ps.filter(p => parentOf(p) !== dir);
    if (!ps.length) return "";
    const d = ent(dir);
    for (const p of ps) if (d.children.has(baseName(p))) return `A file or folder ${baseName(p)} already exists in ${dir === root ? "the root folder" : rel(dir)}.`;
    const pairs = ps.map(p => [p, dir + "/" + baseName(p)]);
    for (const [a, b] of pairs) moveEntry(a, b);
    mapPaths(pairs);
    if (dir !== root) { expandTo(dir + "/x"); expanded.add(dir); saveOpen(); }
    selected = new Set(pairs.map(x => x[1])); focus = anchor = pairs.at(-1)[1];
    o.moved(pairs);
    render(); scrollTo(focus); o.changed();
    return "";
  }
  /** upload writes [[relative path, bytes or null for a folder]] into dir. */
  async function upload(dir, list) {
    if (!list.length || !guard()) return;
    const d = ent(dir);
    const tops = [...new Set(list.map(([p]) => p.split("/")[0]))];
    const clash = tops.filter(n => d.children.has(n));
    if (clash.length) {
      const ok = await o.ask({ title: "Replace existing files?", text: `The folder ${dir === root ? "work" : rel(dir)} already has items with these names. Replace them?`, list: clash, ok: "Replace", danger: true });
      if (!ok || !guard()) return;
      const gone = clash.map(n => dir + "/" + n);
      for (const p of gone) d.children.delete(baseName(p));
      o.removed(gone, []);
    }
    for (const [p, data] of list) {
      const q = dir + "/" + p;
      if (data) fsys.writeFile(q, data); else fsys.mkdirAll(q);
    }
    if (dir !== root) { expandTo(dir + "/x"); expanded.add(dir); saveOpen(); }
    const made = tops.map(n => dir + "/" + n);
    selected = new Set(made); focus = anchor = made.at(-1);
    render(); scrollTo(focus); o.changed();
    o.note(`Added ${list.filter(x => x[1]).length} ${list.filter(x => x[1]).length === 1 ? "file" : "files"} to ${dir === root ? "work" : rel(dir)}.`);
  }
  async function copyText(text, what) {
    o.copied && o.copied(text);
    let ok = false;
    try { await navigator.clipboard.writeText(text); ok = true; } catch {}
    if (!ok) {
      const t = document.createElement("textarea"); t.value = text; t.style.position = "fixed"; t.style.opacity = "0";
      document.body.append(t); t.select();
      try { ok = document.execCommand("copy"); } catch {}
      t.remove(); tree.focus({ preventScroll: true });
    }
    o.note(ok ? `Copied the ${what}: ${text}` : `The browser did not allow a copy. The ${what} is ${text}`);
  }
  /** archive returns {name, blob} of a download of ps. */
  function archive(ps) {
    ps = topLevel(ps);
    if (ps.length === 1 && !isDir(ps[0])) return { name: baseName(ps[0]), blob: new Blob([fsys.readFile(ps[0])], { type: "application/octet-stream" }) };
    const entries = [];
    const add = (p, prefix) => { const e = ent(p); if (!e) return; if (!e.dir) return entries.push([prefix, e.data.slice(0, e.size)]); entries.push([prefix, null]); for (const n of [...e.children.keys()].sort(byName)) add(p + "/" + n, prefix + "/" + n); };
    for (const p of ps) {
      if (p === root) for (const n of [...ent(root).children.keys()].sort(byName)) add(root + "/" + n, n);
      else add(p, baseName(p));
    }
    const name = (ps.length === 1 ? (ps[0] === root ? "work" : baseName(ps[0])) : baseName(parentOf(ps[0])) || "work") + ".zip";
    return { name, blob: zip(entries) };
  }
  function download(ps) {
    const { name, blob } = archive(ps);
    o.downloaded && o.downloaded(name, blob);
    const u = URL.createObjectURL(blob), a = document.createElement("a");
    a.href = u; a.download = name; document.body.append(a); a.click(); a.remove();
    setTimeout(() => URL.revokeObjectURL(u), 30000);
  }
  async function resetSample() {
    if (!guard()) return;
    const ok = await o.ask({ title: "Reset the sample?", text: "This restores the starter files of the sample in /work. All your edits, new files and new folders in /work are lost. You cannot undo this.", ok: "Reset Sample", danger: true });
    if (ok) await o.reset();
    else tree.focus({ preventScroll: true });
  }

  // The context menu.
  const isMac = /Mac|iPhone|iPad/.test(navigator.platform || "");
  const k = s => (isMac ? s.replace("Ctrl+", "⌘").replace("Alt+", "⌥").replace("Shift+", "⇧") : s);
  function items(ps) {
    const isRoot = ps.length === 1 && ps[0] === root, one = ps.length === 1 ? ps[0] : null, dir = !!one && isDir(one);
    const busy = o.busy();
    const list = [];
    if (one && !dir && previewable(one)) list.push(["Open Preview", () => o.open(one, "preview")], ...(o.canSide() ? [["Open Preview to the Side", () => o.open(one, "side")]] : []), "-");
    if (one && !dir && !previewable(one)) list.push(["Open", () => o.open(one, "file")], "-");
    list.push(["New File...", () => startNew("file", folderFor(one || ps[0])), "", busy], ["New Folder...", () => startNew("folder", folderFor(one || ps[0])), "", busy], "-");
    if (!isRoot) list.push(["Duplicate", () => duplicate(ps), "", busy]);
    list.push(["Download", () => download(ps)], "-");
    list.push(["Copy Path", () => copyText(ps.join("\n"), ps.length > 1 ? "paths" : "path"), k("Shift+Alt+C")]);
    if (!isRoot) list.push(["Copy Relative Path", () => copyText(ps.map(rel).join("\n"), ps.length > 1 ? "relative paths" : "relative path"), k("Ctrl+Shift+Alt+C")]);
    if (!isRoot) list.push("-", ["Rename...", () => startRename(one), "F2", busy || !one], ["Delete", () => remove(ps), isMac ? "⌘⌫" : "Delete", busy]);
    list.push("-", ["Collapse All", collapseAll]);
    if (isRoot) list.push("-", ["Reset Sample...", resetSample, "", busy]);
    return list;
  }
  let menuReturn = null;
  function openMenu(x, y, ps, keyboard) {
    closeMenu();
    menuReturn = document.activeElement;
    const list = items(ps);
    menu.replaceChildren(...list.map(it => {
      if (it === "-") return document.createElement("hr");
      const [label, fn, key, off] = it;
      const b = document.createElement("button"); b.type = "button"; b.setAttribute("role", "menuitem"); b.disabled = !!off;
      const l = document.createElement("span"); l.textContent = label; b.append(l);
      if (key) { const kb = document.createElement("kbd"); kb.textContent = key; b.append(kb); }
      b.addEventListener("click", () => { closeMenu(true); fn(); });
      return b;
    }));
    menu.dataset.targets = JSON.stringify(ps);
    menu.hidden = false;
    const r = menu.getBoundingClientRect();
    menu.style.left = Math.max(4, Math.min(x, innerWidth - r.width - 4)) + "px";
    menu.style.top = Math.max(4, y + r.height > innerHeight - 4 ? y - r.height : y) + "px";
    if (r.height > innerHeight - 8) { menu.style.top = "4px"; menu.style.maxHeight = innerHeight - 8 + "px"; menu.style.overflow = "auto"; }
    const first = menu.querySelector("button:not(:disabled)");
    if (keyboard && first) first.focus(); else menu.focus({ preventScroll: true });
  }
  function closeMenu(keepFocus) {
    if (menu.hidden) return;
    menu.hidden = true;
    if (!keepFocus && menuReturn && menuReturn.isConnected) menuReturn.focus({ preventScroll: true });
    else if (keepFocus) tree.focus({ preventScroll: true });
  }
  menu.addEventListener("keydown", e => {
    const bs = [...menu.querySelectorAll("button:not(:disabled)")], i = bs.indexOf(document.activeElement);
    if (e.key === "ArrowDown") bs[(i + 1) % bs.length].focus();
    else if (e.key === "ArrowUp") bs[(i - 1 + bs.length) % bs.length].focus();
    else if (e.key === "Home") bs[0].focus();
    else if (e.key === "End") bs.at(-1).focus();
    else if (e.key === "Escape" || e.key === "Tab") closeMenu();
    else return;
    e.preventDefault(); e.stopPropagation();
  });
  document.addEventListener("pointerdown", e => { if (!menu.hidden && !menu.contains(e.target)) closeMenu(true); }, true);
  addEventListener("resize", () => closeMenu(true));
  addEventListener("blur", () => closeMenu(true));

  // Mouse, touch and keyboard.
  const rowAt = e => { const el = e.target.closest && e.target.closest(".ex-row[data-path]"); return el && tree.contains(el) ? rowOf.get(el.dataset.path) : null; };
  tree.addEventListener("click", e => {
    if (edit) return;
    const r = rowAt(e);
    tree.focus({ preventScroll: true });
    if (!r) { selected.clear(); focus = anchor = null; return paint(); }
    const p = r.path;
    if (e.ctrlKey || e.metaKey) { if (selected.has(p)) selected.delete(p); else selected.add(p); focus = anchor = p; return paint(); }
    if (e.shiftKey && anchor) { rangeTo(p); focus = p; return paint(); }
    selected = new Set([p]); focus = anchor = p;
    if (r.dir) setOpen(p, !expanded.has(p)); else { paint(); o.open(p, "file"); }
  });
  tree.addEventListener("mousedown", e => { if (e.shiftKey || e.detail > 1) e.preventDefault(); });
  const menuFor = (e, x, y) => {
    const r = rowAt(e);
    if (r) { if (!selected.has(r.path)) { selected = new Set([r.path]); anchor = r.path; } focus = r.path; paint(); }
    openMenu(x, y, r ? targets() : [root], false);
  };
  o.side.addEventListener("contextmenu", e => {
    if (e.target.closest(".ex-input")) return;
    e.preventDefault();
    if (!e.clientX && !e.clientY && focus && rowOf.get(focus)) { const b = rowOf.get(focus).el.getBoundingClientRect(); return openMenu(b.left + 24, b.bottom, targets(), true); }
    if (!tree.contains(e.target)) return openMenu(e.clientX, e.clientY, [root], false);
    menuFor(e, e.clientX, e.clientY);
  });
  longPress(tree, (x, y, e) => { if (!edit) menuFor(e, x, y); });

  tree.addEventListener("keydown", e => {
    if (edit || e.target !== tree) return;
    const mod = e.ctrlKey || e.metaKey;
    let i = indexOf(focus);
    const r = rows[i];
    const go = j => {
      if (!rows.length) return;
      j = Math.max(0, Math.min(rows.length - 1, j));
      const p = rows[j].path;
      focus = p;
      if (e.shiftKey) rangeTo(p); else if (!mod) { selected = new Set([p]); anchor = p; }
      paint(); scrollTo(p);
    };
    const page = Math.max(1, Math.floor(tree.clientHeight / 22) - 1);
    if (e.code === "KeyC" && e.altKey && e.shiftKey) { e.preventDefault(); const ps = targets(); return mod ? copyText(ps.map(rel).join("\n"), "relative path") : copyText(ps.join("\n"), "path"); }
    switch (e.key) {
      case "ArrowDown": go(i < 0 ? 0 : i + 1); break;
      case "ArrowUp": go(i < 0 ? 0 : i - 1); break;
      case "Home": go(0); break;
      case "End": go(rows.length - 1); break;
      case "PageDown": go(i + page); break;
      case "PageUp": go(i - page); break;
      case "ArrowRight":
        if (!r) go(0);
        else if (r.dir && !expanded.has(r.path)) setOpen(r.path, true);
        else if (r.dir && rows[i + 1] && rows[i + 1].depth > r.depth) go(i + 1);
        break;
      case "ArrowLeft":
        if (!r) break;
        if (r.dir && expanded.has(r.path)) setOpen(r.path, false);
        else if (r.depth > 0) go(indexOf(parentOf(r.path)));
        break;
      case "Enter": if (r) { if (r.dir) setOpen(r.path, !expanded.has(r.path)); else o.open(r.path, "file", true); } break;
      case " ":
        if (!r) break;
        if (mod) { if (selected.has(r.path)) selected.delete(r.path); else selected.add(r.path); anchor = r.path; paint(); }
        else if (r.dir) setOpen(r.path, !expanded.has(r.path)); else o.open(r.path, "file");
        break;
      case "F2": if (r) startRename(r.path); break;
      case "Delete": remove(targets()); break;
      case "Backspace": if (e.metaKey) remove(targets()); else return; break;
      case "Escape": selected = new Set(focus ? [focus] : []); paint(); break;
      default:
        if (mod && e.key.toLowerCase() === "a") { selected = new Set(rows.map(x => x.path)); paint(); break; }
        if (e.key.length === 1 && !mod && !e.altKey && rows.length) {
          const now = Date.now(); typed = now - typedAt < 700 ? typed + e.key.toLowerCase() : e.key.toLowerCase(); typedAt = now;
          const start = typed.length > 1 ? Math.max(i, 0) : i + 1;
          for (let n = 0; n < rows.length; n++) { const j = (start + n) % rows.length; if (rows[j].name.toLowerCase().startsWith(typed)) { go(j); break; } }
          break;
        }
        return;
    }
    e.preventDefault();
  });

  // Drag and drop.
  function setDrop(dir) {
    dropDir = dir;
    tree.classList.toggle("ex-drop-root", dir === root);
    for (const r of rows) r.el.classList.toggle("ex-drop", !!dir && dir !== root && under(r.path, dir));
  }
  const clearDrop = () => { setDrop(null); clearTimeout(hoverTimer); hoverDir = null; };
  tree.addEventListener("dragstart", e => {
    const r = rowAt(e);
    if (!r || edit) return e.preventDefault();
    if (!selected.has(r.path)) { selected = new Set([r.path]); anchor = r.path; }
    focus = r.path; paint();
    drag = topLevel([...selected]);
    e.dataTransfer.effectAllowed = "copyMove";
    e.dataTransfer.setData("text/plain", drag.map(rel).join("\n"));
    e.dataTransfer.setData("application/x-docgen-paths", JSON.stringify(drag));
    for (const p of drag) rowOf.get(p)?.el.classList.add("ex-cut");
  });
  tree.addEventListener("dragend", () => { drag = null; clearDrop(); for (const r of rows) r.el.classList.remove("ex-cut"); });
  const dirAt = e => { const r = rowAt(e); return r ? (r.dir ? r.path : parentOf(r.path)) : root; };
  const external = dt => !!dt && [...dt.types].includes("Files");
  tree.addEventListener("dragover", e => {
    const dir = dirAt(e);
    if (drag) {
      if (drag.some(p => under(dir, p)) || drag.every(p => parentOf(p) === dir)) { e.dataTransfer.dropEffect = "none"; return setDrop(null); }
      e.preventDefault(); e.dataTransfer.dropEffect = "move";
    } else if (external(e.dataTransfer)) { e.preventDefault(); e.dataTransfer.dropEffect = "copy"; }
    else return;
    if (dropDir !== dir) setDrop(dir);
    const r = rowAt(e);
    const hover = r && r.dir && !expanded.has(r.path) ? r.path : null;
    if (hover !== hoverDir) {
      clearTimeout(hoverTimer); hoverDir = hover;
      if (hover) hoverTimer = setTimeout(() => { if (hoverDir === hover) { expanded.add(hover); saveOpen(); render(); } }, 500);
    }
  });
  tree.addEventListener("dragleave", e => { if (!e.relatedTarget || !tree.contains(e.relatedTarget)) clearDrop(); });
  tree.addEventListener("drop", e => {
    e.preventDefault();
    const dir = dirAt(e);
    clearDrop();
    if (drag) { const ps = drag; drag = null; const m = moveTo(ps, dir); if (m && m !== "busy") o.note(m); return; }
    const dt = e.dataTransfer;
    if (!external(dt)) return;
    const its = [...dt.items].filter(i => i.kind === "file").map(i => (i.webkitGetAsEntry && i.webkitGetAsEntry()) || i.getAsFile());
    const files = its.length ? its : [...dt.files];
    readDrop(files).then(list => upload(dir, list), err => o.note("Could not read the dropped files: " + err));
  });
  async function readDrop(items) {
    const out = [];
    const readEntry = async (en, pre) => {
      const p = pre + en.name;
      if (en instanceof File) return out.push([p, new Uint8Array(await en.arrayBuffer())]);
      if (en.isFile) { const f = await new Promise((res, rej) => en.file(res, rej)); return out.push([p, new Uint8Array(await f.arrayBuffer())]); }
      if (!en.isDirectory) return;
      out.push([p, null]);
      const rd = en.createReader();
      for (;;) { const b = await new Promise((res, rej) => rd.readEntries(res, rej)); if (!b.length) break; for (const c of b) await readEntry(c, p + "/"); }
    };
    for (const it of items) if (it) await readEntry(it, "");
    return out;
  }

  // The toolbar.
  const tb = (id, fn) => o.side.querySelector("#" + id).addEventListener("click", fn);
  tb("exnewfile", () => startNew("file"));
  tb("exnewdir", () => startNew("folder"));
  tb("exrefresh", () => render());
  tb("excollapse", collapseAll);
  o.side.querySelector("#exhead").addEventListener("dragover", e => { if (drag || external(e.dataTransfer)) { e.preventDefault(); setDrop(root); } });
  o.side.querySelector("#exhead").addEventListener("drop", e => { tree.dispatchEvent(new DragEvent("drop", { dataTransfer: e.dataTransfer, bubbles: false })); e.preventDefault(); });

  return {
    render, decorate: () => decorate(), reveal, openMenu, closeMenu, archive,
    state: () => ({ expanded: [...expanded], selected: [...selected], focus, edit: edit && { ...edit }, rows: rows.map(r => r.path), drop: dropDir }),
  };
}
