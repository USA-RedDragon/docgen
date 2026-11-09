// The explorer of the playground page: a tree of one memfs folder, after the Explorer of VS Code. It
// does the file operations (new, rename, move, duplicate, delete, upload, download, copy path) on the
// memfs and gives the changed paths to the page, which then updates the tabs, the badges and the
// storage.
import { element, icon, longPress } from "./dom.mjs";
import { parentOf, baseName, isUnder, movePath } from "./paths.mjs";

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });
/** byName orders names as VS Code does: numbers by value, then case, then code point. */
const byName = (a, b) => collator.compare(a, b) || (a < b ? -1 : a > b ? 1 : 0);
/** The localStorage key of the list of open folders. */
const OPEN_FOLDERS_KEY = "pg-explorer-open";
const ROW_HEIGHT = 22;
const HOVER_EXPAND_MS = 500;
/** The time in which typed letters add up to one name to find. */
const TYPE_AHEAD_MS = 700;
const IS_MAC = /Mac|iPhone|iPad/.test(navigator.platform || "");

/** previewable reports whether the file path has a preview: Markdown and SVG. */
export const previewable = path => /\.(md|markdown|svg)$/i.test(path);

const ICON_OF_EXTENSION = {
  go: "go",
  md: "md",
  markdown: "md",
  yaml: "yaml",
  yml: "yaml",
  json: "json",
  toml: "toml",
  svg: "svg",
};

/** iconOf returns the id of the icon symbol of a file or folder name. */
export function iconOf(name, dir, open) {
  if (dir) return open ? "fo" : "fc";
  if (name === "go.mod" || name === "go.sum" || name === "go.work") return "mod";
  const dot = name.lastIndexOf(".");
  return (dot > 0 && ICON_OF_EXTENSION[name.slice(dot + 1).toLowerCase()]) || "plain";
}

/** shortcut returns the text of a keyboard shortcut, with the symbols of the Mac on a Mac. */
function shortcut(text) {
  if (!IS_MAC) return text;
  return text.replace("Ctrl+", "⌘").replace("Alt+", "⌥").replace("Shift+", "⇧");
}

const CRC_TABLE = (() => {
  const table = new Uint32Array(256);
  for (let n = 0; n < 256; n++) {
    let crc = n;
    for (let bit = 0; bit < 8; bit++) crc = crc & 1 ? 0xedb88320 ^ (crc >>> 1) : crc >>> 1;
    table[n] = crc >>> 0;
  }
  return table;
})();

function crc32(bytes) {
  let crc = 0xffffffff;
  for (let i = 0; i < bytes.length; i++) crc = CRC_TABLE[(crc ^ bytes[i]) & 255] ^ (crc >>> 8);
  return (crc ^ 0xffffffff) >>> 0;
}

const ZIP_LOCAL_HEADER = 0x04034b50;
const ZIP_CENTRAL_HEADER = 0x02014b50;
const ZIP_END = 0x06054b50;
const ZIP_VERSION = 20;
/** The "made by" field of a central header: Unix (3), version 3.0 (30). */
const ZIP_MADE_BY_UNIX = 0x31e;
/** The flag of a name in UTF-8. */
const ZIP_UTF8 = 0x800;
const ZIP_MSDOS_DIRECTORY = 0x10;

/** zip returns a zip archive (stored, no compression) of entries, each [name, bytes, or null for a folder]. */
function zip(entries) {
  const textEncoder = new TextEncoder();
  const parts = [];
  const central = [];
  const now = new Date();
  const time = (now.getHours() << 11) | (now.getMinutes() << 5) | (now.getSeconds() >> 1);
  const date = ((now.getFullYear() - 1980) << 9) | ((now.getMonth() + 1) << 5) | now.getDate();
  let offset = 0;
  for (const [entryName, data] of entries) {
    const name = textEncoder.encode(data ? entryName : entryName + "/");
    const body = data || new Uint8Array(0);
    const crc = crc32(body);
    const local = new DataView(new ArrayBuffer(30));
    local.setUint32(0, ZIP_LOCAL_HEADER, true);
    local.setUint16(4, ZIP_VERSION, true);
    local.setUint16(6, ZIP_UTF8, true);
    local.setUint16(10, time, true);
    local.setUint16(12, date, true);
    local.setUint32(14, crc, true);
    local.setUint32(18, body.length, true);
    local.setUint32(22, body.length, true);
    local.setUint16(26, name.length, true);
    const header = new DataView(new ArrayBuffer(46));
    header.setUint32(0, ZIP_CENTRAL_HEADER, true);
    header.setUint16(4, ZIP_MADE_BY_UNIX, true);
    header.setUint16(6, ZIP_VERSION, true);
    header.setUint16(8, ZIP_UTF8, true);
    header.setUint16(12, time, true);
    header.setUint16(14, date, true);
    header.setUint32(16, crc, true);
    header.setUint32(20, body.length, true);
    header.setUint32(24, body.length, true);
    header.setUint16(28, name.length, true);
    // The external attributes: the Unix mode in the high 16 bits, and the MS-DOS folder bit.
    header.setUint32(38, data ? 0o100644 << 16 : (0o40755 << 16) | ZIP_MSDOS_DIRECTORY, true);
    header.setUint32(42, offset, true);
    parts.push(new Uint8Array(local.buffer), name, body);
    central.push(new Uint8Array(header.buffer), name);
    offset += 30 + name.length + body.length;
  }
  const centralSize = central.reduce((total, part) => total + part.length, 0);
  const end = new DataView(new ArrayBuffer(22));
  end.setUint32(0, ZIP_END, true);
  end.setUint16(8, entries.length, true);
  end.setUint16(10, entries.length, true);
  end.setUint32(12, centralSize, true);
  end.setUint32(16, offset, true);
  return new Blob([...parts, ...central, new Uint8Array(end.buffer)], { type: "application/zip" });
}

/**
 * readDrop reads the dropped items, each a File or a FileSystemEntry, as a list of [relative path,
 * bytes or null for a folder].
 */
async function readDrop(items) {
  const files = [];
  const readEntry = async (item, prefix) => {
    const path = prefix + item.name;
    if (item instanceof File) {
      files.push([path, new Uint8Array(await item.arrayBuffer())]);
      return;
    }
    if (item.isFile) {
      const file = await new Promise((resolve, reject) => item.file(resolve, reject));
      files.push([path, new Uint8Array(await file.arrayBuffer())]);
      return;
    }
    if (!item.isDirectory) return;
    files.push([path, null]);
    const reader = item.createReader();
    for (;;) {
      const batch = await new Promise((resolve, reject) => reader.readEntries(resolve, reject));
      if (!batch.length) break;
      for (const child of batch) await readEntry(child, path + "/");
    }
  };
  for (const item of items) if (item) await readEntry(item, "");
  return files;
}

/**
 * createExplorer shows the folder page.root of page.fsys in page.tree, with the context menu in
 * page.menu and the toolbar in page.side. The page gives these functions:
 * open(path, how) opens a file ("file", "preview" or "side"); changes() returns a Map of path to "M"
 * or "U"; edited(path) returns true when a file differs from the starter; moved(pairs) and
 * removed(paths, edited) run after file operations; changed() runs after each change of the memfs;
 * busy() returns true during a run; note(text) shows a message; ask({title, text, list, ok}) resolves
 * to true on OK; reset() restores the sample; showTree() makes the tree visible; canSide() returns
 * true when a preview can open to the side; copied(text) and downloaded(name, blob), when set, get
 * each copy and download.
 */
export function createExplorer(page) {
  const { fsys, root, tree, menu } = page;
  let expanded = readOpenFolders();
  let selected = new Set();
  let focus = null;
  /** The start of a Shift selection. */
  let anchor = null;
  /** The name input in the tree: {kind: "file" or "folder", parent, dir} or {kind: "rename", path, dir}. */
  let edit = null;
  /** The visible rows in order, each {path, name, dir, depth, element}. */
  let rows = [];
  let rowOf = new Map();
  let nextRowId = 0;
  /** The paths that a drag in the tree moves. */
  let dragged = null;
  let dropDir = null;
  let hoverDir = null;
  let hoverTimer = 0;
  let typed = "";
  let typedAt = 0;
  let menuReturnFocus = null;

  function readOpenFolders() {
    try {
      const saved = JSON.parse(localStorage.getItem(OPEN_FOLDERS_KEY));
      if (Array.isArray(saved)) return new Set(saved);
    } catch {
      // No storage, or a value that is not JSON: all folders open.
    }
    const all = new Set();
    fsys.walk(root, (path, entry) => {
      if (entry.dir && path !== root) all.add(path);
    });
    return all;
  }

  function saveOpenFolders() {
    try {
      localStorage.setItem(OPEN_FOLDERS_KEY, JSON.stringify([...expanded]));
    } catch {
      // The open folders then last until a reload.
    }
  }

  const entryOf = path => fsys.lookup(path);
  const isDir = path => {
    const entry = entryOf(path);
    return !!entry && entry.dir;
  };
  const relative = path => path.slice(root.length + 1);
  /** folderLabel returns the name of the folder dir in a message. */
  const folderLabel = dir => (dir === root ? "work" : relative(dir));
  /** topLevel returns the paths that are not inside another path of the list. */
  const topLevel = paths => paths.filter(path => !paths.some(other => other !== path && isUnder(path, other)));

  function filesUnder(path) {
    const files = [];
    if (!entryOf(path)) return files;
    fsys.walk(path, (filePath, entry) => {
      if (!entry.dir) files.push(filePath);
    });
    return files;
  }

  /** canChange shows a note and returns false during a run. */
  function canChange() {
    if (!page.busy()) return true;
    page.note("Wait for the run to end before you change files.");
    return false;
  }

  /**
   * nameError returns the error text of the new name raw in the folder dir, or "". self is the current
   * name of a rename.
   */
  function nameError(raw, dir, self) {
    const name = raw.trim();
    if (!name) return "A file or folder name must be provided.";
    if (name.includes("/")) return "A name cannot contain a slash (/).";
    if (name === "." || name === "..") return `The name ${name} is not valid as a file or folder name.`;
    if (name.includes("\0")) return "A name cannot contain a null character.";
    const dirEntry = entryOf(dir);
    if (name !== self && dirEntry && dirEntry.dir && dirEntry.children.has(name)) {
      return `A file or folder ${name} already exists in this folder. Choose a different name.`;
    }
    return "";
  }

  function selectOnly(path) {
    selected = new Set([path]);
    focus = path;
    anchor = path;
  }

  /** foldersWithChanges returns the folders under the root that hold a path of changes. */
  function foldersWithChanges(changes) {
    const folders = new Set();
    for (const path of changes.keys()) {
      for (let dir = parentOf(path); dir.startsWith(root + "/"); dir = parentOf(dir)) folders.add(dir);
    }
    return folders;
  }

  // Rendering.

  function render() {
    const changes = page.changes();
    rows = [];
    rowOf = new Map();
    const fragment = document.createDocumentFragment();
    let editInput = null;
    const addRows = (dir, depth) => {
      const dirEntry = entryOf(dir);
      if (!dirEntry || !dirEntry.dir) return;
      if (edit && edit.kind !== "rename" && edit.parent === dir) {
        const newRow = newEntryRow(depth);
        fragment.append(newRow.element);
        editInput = newRow.input;
      }
      const children = [...dirEntry.children].sort(([a, x], [b, y]) => y.dir - x.dir || byName(a, b));
      for (const [name, child] of children) {
        const path = dir + "/" + name;
        const row = { path, name, dir: child.dir, depth };
        row.element = rowElement(row);
        if (edit && edit.kind === "rename" && edit.path === path) editInput = renameInput(row);
        rows.push(row);
        rowOf.set(path, row);
        fragment.append(row.element);
        if (child.dir && expanded.has(path)) addRows(path, depth + 1);
      }
    };
    addRows(root, 0);
    tree.replaceChildren(fragment);
    for (const path of [...selected]) if (!entryOf(path)) selected.delete(path);
    if (focus && !entryOf(focus)) focus = null;
    decorate(changes);
    paint();
    if (dropDir) setDrop(dropDir);
    if (editInput) {
      editInput.focus();
      editInput.dispatchEvent(new Event("select-name"));
    }
  }

  function rowElement(row) {
    const open = expanded.has(row.path);
    const rowNode = element("div", {
      className: "ex-row",
      id: "ex-" + ++nextRowId,
      draggable: true,
      title: relative(row.path),
      attributes: { role: "treeitem", "aria-level": row.depth + 1 },
    });
    rowNode.dataset.path = row.path;
    rowNode.style.setProperty("--d", row.depth);
    const twistie = element("span", { className: "ex-tw" });
    if (row.dir) {
      rowNode.setAttribute("aria-expanded", String(open));
      twistie.append(icon("t-chev", ""));
    }
    rowNode.append(
      element("span", { className: "ex-ind" }),
      twistie,
      icon("i-" + iconOf(row.name, row.dir, open)),
      element("span", { className: "ex-name", textContent: row.name }),
      element("span", { className: "ex-badge", attributes: { "aria-hidden": "true" } }),
    );
    return rowNode;
  }

  /**
   * nameInput returns {box, input}: a name input with the start text value and an error message.
   * check(text) returns the error text of a name, or "". done(name) gets the name on Enter, or on a
   * blur with a valid name, and null on Escape or on a blur with a bad name.
   */
  function nameInput(value, check, done) {
    const input = element("input", {
      className: "ex-input",
      value,
      spellcheck: false,
      autocomplete: "off",
      attributes: { "aria-label": "Name" },
    });
    const message = element("div", { className: "ex-msg", hidden: true, attributes: { role: "alert" } });
    const validate = () => {
      const error = check(input.value);
      message.textContent = error;
      message.hidden = !error;
      input.classList.toggle("bad", !!error);
      input.setAttribute("aria-invalid", String(!!error));
      return error;
    };
    let ended = false;
    const end = accept => {
      if (ended) return;
      ended = true;
      done(accept ? input.value.trim() : null);
    };
    input.addEventListener("input", validate);
    input.addEventListener("keydown", event => {
      event.stopPropagation();
      if (event.key === "Enter") {
        event.preventDefault();
        if (!validate()) end(true);
      } else if (event.key === "Escape") {
        event.preventDefault();
        end(false);
      }
    });
    // The check runs after the blur event, when document.activeElement shows where the focus went.
    input.addEventListener("blur", () =>
      setTimeout(() => {
        if (!ended && document.activeElement !== input) end(!validate());
      }, 0),
    );
    for (const type of ["click", "dblclick", "pointerdown", "mousedown", "contextmenu", "dragstart"]) {
      input.addEventListener(type, event => event.stopPropagation());
    }
    // The name of a file is selected without its extension, as in VS Code.
    input.addEventListener("select-name", () => {
      const dot = value.lastIndexOf(".");
      input.setSelectionRange(0, dot > 0 && !edit?.dir ? dot : value.length);
    });
    return { box: element("span", { className: "ex-edit" }, input, message), input };
  }

  function renameInput(row) {
    const check = text => nameError(text, parentOf(row.path), row.name);
    const { box, input } = nameInput(row.name, check, name => {
      const path = edit.path;
      edit = null;
      if (name && name !== row.name) rename(path, name);
      else render();
      tree.focus({ preventScroll: true });
    });
    row.element.querySelector(".ex-name").replaceWith(box);
    row.element.draggable = false;
    return input;
  }

  /** newEntryRow returns {element, input}: the row of the name of a new file or folder. */
  function newEntryRow(depth) {
    const { kind, parent } = edit;
    const check = text => nameError(text, parent, null);
    const { box, input } = nameInput("", check, name => {
      edit = null;
      if (name) create(kind, parent + "/" + name);
      else render();
      tree.focus({ preventScroll: true });
    });
    const row = element(
      "div",
      { className: "ex-row ex-new" },
      element("span", { className: "ex-ind" }),
      element("span", { className: "ex-tw" }),
      icon(kind === "folder" ? "i-fc" : "i-plain"),
      box,
    );
    row.style.setProperty("--d", depth);
    return { element: row, input };
  }

  /** decorate sets the M, U and folder marks without a new render. */
  function decorate(changes = page.changes()) {
    const changedFolders = foldersWithChanges(changes);
    for (const row of rows) {
      const mark = row.dir ? "" : changes.get(row.path) || "";
      const holdsChanges = row.dir && changedFolders.has(row.path);
      row.element.classList.toggle("ex-M", mark === "M");
      row.element.classList.toggle("ex-U", mark === "U");
      row.element.classList.toggle("ex-dirty", holdsChanges);
      const badge = row.element.lastChild;
      badge.textContent = mark || (holdsChanges ? "●" : "");
      if (mark === "M") badge.title = "Modified";
      else if (mark === "U") badge.title = "New";
      else badge.title = holdsChanges ? "Contains changes" : "";
    }
  }

  /** paint shows the selection and the focus. */
  function paint() {
    let focusId = "";
    for (const row of rows) {
      const isSelected = selected.has(row.path);
      const isFocus = focus === row.path;
      row.element.classList.toggle("sel", isSelected);
      row.element.classList.toggle("foc", isFocus);
      row.element.setAttribute("aria-selected", String(isSelected));
      if (isFocus) focusId = row.element.id;
    }
    if (focusId) tree.setAttribute("aria-activedescendant", focusId);
    else tree.removeAttribute("aria-activedescendant");
  }

  function scrollTo(path) {
    const row = rowOf.get(path);
    if (row) row.element.scrollIntoView({ block: "nearest" });
  }

  function setOpen(path, open) {
    if (open === expanded.has(path)) return;
    if (open) expanded.add(path);
    else expanded.delete(path);
    saveOpenFolders();
    render();
  }

  /** expandTo opens the folders above path. It returns true when it opened one. */
  function expandTo(path) {
    let changed = false;
    for (let dir = parentOf(path); dir.startsWith(root + "/"); dir = parentOf(dir)) {
      if (expanded.has(dir)) continue;
      expanded.add(dir);
      changed = true;
    }
    if (changed) saveOpenFolders();
    return changed;
  }

  /** openFolder opens the folder dir and the folders above it. */
  function openFolder(dir) {
    if (dir === root) return;
    expandTo(dir + "/x");
    expanded.add(dir);
    saveOpenFolders();
  }

  /** reveal opens the folders above path, selects path and scrolls it into view. */
  function reveal(path) {
    if (!entryOf(path) || edit) return;
    const changed = expandTo(path);
    selectOnly(path);
    if (changed) render();
    else paint();
    scrollTo(path);
  }

  function collapseAll() {
    expanded.clear();
    saveOpenFolders();
    render();
  }

  // Selection.

  const indexOf = path => rows.findIndex(row => row.path === path);

  /** selectRange selects the rows from the anchor to path. */
  function selectRange(path) {
    const from = indexOf(anchor);
    const to = indexOf(path);
    if (from < 0 || to < 0) {
      selected = new Set([path]);
      anchor = path;
      return;
    }
    selected = new Set(rows.slice(Math.min(from, to), Math.max(from, to) + 1).map(row => row.path));
  }

  /** targets returns the paths of an action: the selection, else the focus, else the root. */
  function targets() {
    const paths = topLevel([...selected]);
    if (paths.length) return paths;
    return focus ? [focus] : [root];
  }

  /** folderFor returns the folder of a new entry next to path: path when it is a folder, else its parent. */
  function folderFor(path) {
    if (!path || path === root) return root;
    return isDir(path) ? path : parentOf(path);
  }

  // Operations.

  function startNew(kind, dir = folderFor(focus)) {
    if (!canChange()) return;
    page.showTree();
    openFolder(dir);
    edit = { kind, parent: dir, dir: kind === "folder" };
    render();
  }

  function startRename(path) {
    if (!path || path === root || !entryOf(path) || !canChange()) return;
    page.showTree();
    expandTo(path);
    edit = { kind: "rename", path, dir: isDir(path) };
    selectOnly(path);
    render();
  }

  /** movePaths changes the open folders, the selection, the focus and the anchor after the moves of pairs. */
  function movePaths(pairs) {
    expanded = new Set([...expanded].map(path => movePath(path, pairs)));
    saveOpenFolders();
    selected = new Set([...selected].map(path => movePath(path, pairs)));
    if (focus) focus = movePath(focus, pairs);
    if (anchor) anchor = movePath(anchor, pairs);
  }

  function moveEntry(from, to) {
    const fromDir = entryOf(parentOf(from));
    const toDir = entryOf(parentOf(to));
    const entry = fromDir.children.get(baseName(from));
    fromDir.children.delete(baseName(from));
    toDir.children.set(baseName(to), entry);
    entry.mtime = Date.now();
  }

  function rename(path, name) {
    if (!canChange()) return render();
    const error = nameError(name, parentOf(path), baseName(path));
    if (error) {
      page.note(error);
      return render();
    }
    const newPath = parentOf(path) + "/" + name;
    moveEntry(path, newPath);
    movePaths([[path, newPath]]);
    selectOnly(newPath);
    page.moved([[path, newPath]]);
    render();
    page.changed();
  }

  function create(kind, path) {
    if (!canChange()) return render();
    const error = nameError(baseName(path), parentOf(path), null);
    if (error) {
      page.note(error);
      return render();
    }
    if (kind === "folder") {
      fsys.mkdirAll(path);
      expanded.add(path);
      saveOpenFolders();
    } else {
      fsys.writeFile(path, new Uint8Array(0));
    }
    selectOnly(path);
    render();
    scrollTo(path);
    page.changed();
    if (kind !== "folder") page.open(path, "file");
  }

  function copyEntry(entry) {
    const copy = fsys.newEntry(entry.dir);
    copy.mode = entry.mode;
    if (entry.dir) {
      for (const [name, child] of entry.children) copy.children.set(name, copyEntry(child));
    } else {
      copy.data = entry.data.slice(0, entry.size);
      copy.size = entry.size;
    }
    return copy;
  }

  /** copyName returns the first free name of a copy of name in dir: "a copy.go", then "a copy 2.go". */
  function copyName(dir, name, isFolder) {
    const children = entryOf(dir).children;
    const dot = isFolder ? -1 : name.lastIndexOf(".");
    const stem = dot > 0 ? name.slice(0, dot) : name;
    const extension = dot > 0 ? name.slice(dot) : "";
    for (let i = 1; ; i++) {
      const candidate = `${stem} copy${i > 1 ? " " + i : ""}${extension}`;
      if (!children.has(candidate)) return candidate;
    }
  }

  function duplicate(paths) {
    if (!canChange()) return;
    const made = [];
    for (const path of topLevel(paths)) {
      if (path === root) continue;
      const dir = parentOf(path);
      const entry = entryOf(path);
      const name = copyName(dir, baseName(path), entry.dir);
      entryOf(dir).children.set(name, copyEntry(entry));
      made.push(dir + "/" + name);
    }
    if (!made.length) return;
    selected = new Set(made);
    focus = anchor = made.at(-1);
    render();
    scrollTo(focus);
    page.changed();
  }

  async function remove(paths) {
    paths = topLevel(paths).filter(path => path !== root && entryOf(path));
    if (!paths.length || !canChange()) return;
    const lost = paths.flatMap(filesUnder).filter(page.edited);
    const folders = paths.filter(isDir).length;
    let text = folders ? "The contents of folders are deleted too. " : "";
    if (lost.length) text += `These files have edits that will be lost: ${lost.map(relative).join(", ")}. `;
    text += "You cannot undo this.";
    const confirmed = await page.ask({
      title: paths.length === 1 ? `Delete ${baseName(paths[0])}?` : `Delete ${paths.length} items?`,
      text,
      list: paths.length > 1 ? paths.map(relative) : [],
      ok: "Delete",
      danger: true,
    });
    if (!confirmed || !canChange()) return tree.focus({ preventScroll: true });
    // The selection goes to the next row that stays, else to the previous one.
    const stays = row => !paths.some(path => isUnder(row.path, path));
    const next =
      rows.slice(indexOf(paths.at(-1)) + 1).find(stays) || rows.slice(0, indexOf(paths[0])).reverse().find(stays);
    for (const path of paths) entryOf(parentOf(path)).children.delete(baseName(path));
    for (const dir of [...expanded]) if (paths.some(path => isUnder(dir, path))) expanded.delete(dir);
    saveOpenFolders();
    selected = new Set(next ? [next.path] : []);
    focus = anchor = next ? next.path : null;
    page.removed(paths, lost);
    render();
    page.changed();
    tree.focus({ preventScroll: true });
  }

  /** moveTo moves paths into the folder dir. It returns an error text, "busy", or "" when it moved the files. */
  function moveTo(paths, dir) {
    paths = topLevel(paths).filter(path => path !== root);
    if (!canChange()) return "busy";
    for (const path of paths) if (isUnder(dir, path)) return `Cannot move ${baseName(path)} into itself.`;
    paths = paths.filter(path => parentOf(path) !== dir);
    if (!paths.length) return "";
    const children = entryOf(dir).children;
    for (const path of paths) {
      if (children.has(baseName(path))) {
        const where = dir === root ? "the root folder" : relative(dir);
        return `A file or folder ${baseName(path)} already exists in ${where}.`;
      }
    }
    const pairs = paths.map(path => [path, dir + "/" + baseName(path)]);
    for (const [from, to] of pairs) moveEntry(from, to);
    movePaths(pairs);
    openFolder(dir);
    selected = new Set(pairs.map(([, to]) => to));
    focus = anchor = pairs.at(-1)[1];
    page.moved(pairs);
    render();
    scrollTo(focus);
    page.changed();
    return "";
  }

  /** upload writes files, each [relative path, bytes or null for a folder], into dir. */
  async function upload(dir, files) {
    if (!files.length || !canChange()) return;
    const children = entryOf(dir).children;
    const topNames = [...new Set(files.map(([path]) => path.split("/")[0]))];
    const clashes = topNames.filter(name => children.has(name));
    if (clashes.length) {
      const confirmed = await page.ask({
        title: "Replace existing files?",
        text: `The folder ${folderLabel(dir)} already has items with these names. Replace them?`,
        list: clashes,
        ok: "Replace",
        danger: true,
      });
      if (!confirmed || !canChange()) return;
      const replaced = clashes.map(name => dir + "/" + name);
      for (const path of replaced) children.delete(baseName(path));
      page.removed(replaced, []);
    }
    for (const [path, data] of files) {
      if (data) fsys.writeFile(dir + "/" + path, data);
      else fsys.mkdirAll(dir + "/" + path);
    }
    openFolder(dir);
    const made = topNames.map(name => dir + "/" + name);
    selected = new Set(made);
    focus = anchor = made.at(-1);
    render();
    scrollTo(focus);
    page.changed();
    const fileCount = files.filter(([, data]) => data).length;
    page.note(`Added ${fileCount} ${fileCount === 1 ? "file" : "files"} to ${folderLabel(dir)}.`);
  }

  async function copyText(text, what) {
    if (page.copied) page.copied(text);
    let copied = false;
    try {
      await navigator.clipboard.writeText(text);
      copied = true;
    } catch {
      // No Clipboard API, or no permission: execCommand below.
    }
    if (!copied) {
      const scratch = element("textarea", { value: text });
      scratch.style.position = "fixed";
      scratch.style.opacity = "0";
      document.body.append(scratch);
      scratch.select();
      try {
        copied = document.execCommand("copy");
      } catch {
        // copied stays false.
      }
      scratch.remove();
      tree.focus({ preventScroll: true });
    }
    page.note(copied ? `Copied the ${what}: ${text}` : `The browser did not allow a copy. The ${what} is ${text}`);
  }

  /** archive returns {name, blob} of a download of paths: the file itself, or a zip. */
  function archive(paths) {
    paths = topLevel(paths);
    if (paths.length === 1 && !isDir(paths[0])) {
      const blob = new Blob([fsys.readFile(paths[0])], { type: "application/octet-stream" });
      return { name: baseName(paths[0]), blob };
    }
    const entries = [];
    const add = (path, name) => {
      const entry = entryOf(path);
      if (!entry) return;
      if (!entry.dir) {
        entries.push([name, entry.data.slice(0, entry.size)]);
        return;
      }
      entries.push([name, null]);
      for (const child of [...entry.children.keys()].sort(byName)) add(path + "/" + child, name + "/" + child);
    };
    for (const path of paths) {
      if (path !== root) {
        add(path, baseName(path));
        continue;
      }
      for (const child of [...entryOf(root).children.keys()].sort(byName)) add(root + "/" + child, child);
    }
    let name;
    if (paths.length === 1) name = paths[0] === root ? "work" : baseName(paths[0]);
    else name = baseName(parentOf(paths[0])) || "work";
    return { name: name + ".zip", blob: zip(entries) };
  }

  function download(paths) {
    const { name, blob } = archive(paths);
    if (page.downloaded) page.downloaded(name, blob);
    const url = URL.createObjectURL(blob);
    const link = element("a", { href: url, download: name });
    document.body.append(link);
    link.click();
    link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 30000);
  }

  async function resetSample() {
    if (!canChange()) return;
    const confirmed = await page.ask({
      title: "Reset the sample?",
      text:
        "This restores the starter files of the sample in /work. All your edits, new files and new folders " +
        "in /work are lost. You cannot undo this.",
      ok: "Reset Sample",
      danger: true,
    });
    if (confirmed) await page.reset();
    else tree.focus({ preventScroll: true });
  }

  // The context menu.

  /**
   * menuItems returns the items of the context menu of paths: "-" for a separator, else
   * [label, action, shortcut, disabled].
   */
  function menuItems(paths) {
    const isRoot = paths.length === 1 && paths[0] === root;
    const one = paths.length === 1 ? paths[0] : null;
    const oneFile = !!one && !isDir(one);
    const busy = page.busy();
    const newFolder = () => folderFor(one || paths[0]);
    const relativePaths = () => paths.map(relative).join("\n");
    const items = [];
    if (oneFile && previewable(one)) {
      items.push(["Open Preview", () => page.open(one, "preview")]);
      if (page.canSide()) items.push(["Open Preview to the Side", () => page.open(one, "side")]);
      items.push("-");
    }
    if (oneFile && !previewable(one)) items.push(["Open", () => page.open(one, "file")], "-");
    items.push(["New File...", () => startNew("file", newFolder()), "", busy]);
    items.push(["New Folder...", () => startNew("folder", newFolder()), "", busy], "-");
    if (!isRoot) items.push(["Duplicate", () => duplicate(paths), "", busy]);
    items.push(["Download", () => download(paths)], "-");
    const pathWord = paths.length > 1 ? "paths" : "path";
    items.push(["Copy Path", () => copyText(paths.join("\n"), pathWord), shortcut("Shift+Alt+C")]);
    if (!isRoot) {
      const copyRelative = () => copyText(relativePaths(), "relative " + pathWord);
      items.push(["Copy Relative Path", copyRelative, shortcut("Ctrl+Shift+Alt+C")]);
      items.push("-", ["Rename...", () => startRename(one), "F2", busy || !one]);
      items.push(["Delete", () => remove(paths), IS_MAC ? "⌘⌫" : "Delete", busy]);
    }
    items.push("-", ["Collapse All", collapseAll]);
    if (isRoot) items.push("-", ["Reset Sample...", resetSample, "", busy]);
    return items;
  }

  /** openMenu opens the context menu of paths at x, y. With keyboard true, the first item gets the focus. */
  function openMenu(x, y, paths, keyboard) {
    closeMenu();
    menuReturnFocus = document.activeElement;
    menu.replaceChildren(
      ...menuItems(paths).map(item => {
        if (item === "-") return document.createElement("hr");
        const [label, action, keys, disabled] = item;
        const button = element(
          "button",
          { type: "button", disabled: !!disabled, attributes: { role: "menuitem" } },
          element("span", { textContent: label }),
        );
        if (keys) button.append(element("kbd", { textContent: keys }));
        button.addEventListener("click", () => {
          closeMenu(true);
          action();
        });
        return button;
      }),
    );
    menu.dataset.targets = JSON.stringify(paths);
    menu.hidden = false;
    const box = menu.getBoundingClientRect();
    menu.style.left = Math.max(4, Math.min(x, innerWidth - box.width - 4)) + "px";
    menu.style.top = Math.max(4, y + box.height > innerHeight - 4 ? y - box.height : y) + "px";
    if (box.height > innerHeight - 8) {
      menu.style.top = "4px";
      menu.style.maxHeight = innerHeight - 8 + "px";
      menu.style.overflow = "auto";
    }
    const first = menu.querySelector("button:not(:disabled)");
    if (keyboard && first) first.focus();
    else menu.focus({ preventScroll: true });
  }

  /** closeMenu closes the menu. The focus goes back where it was, or to the tree with keepFocus. */
  function closeMenu(keepFocus) {
    if (menu.hidden) return;
    menu.hidden = true;
    if (keepFocus) tree.focus({ preventScroll: true });
    else if (menuReturnFocus && menuReturnFocus.isConnected) menuReturnFocus.focus({ preventScroll: true });
  }

  menu.addEventListener("keydown", event => {
    const buttons = [...menu.querySelectorAll("button:not(:disabled)")];
    const index = buttons.indexOf(document.activeElement);
    if (event.key === "ArrowDown") buttons[(index + 1) % buttons.length].focus();
    else if (event.key === "ArrowUp") buttons[(index - 1 + buttons.length) % buttons.length].focus();
    else if (event.key === "Home") buttons[0].focus();
    else if (event.key === "End") buttons.at(-1).focus();
    else if (event.key === "Escape" || event.key === "Tab") closeMenu();
    else return;
    event.preventDefault();
    event.stopPropagation();
  });
  document.addEventListener(
    "pointerdown",
    event => {
      if (!menu.hidden && !menu.contains(event.target)) closeMenu(true);
    },
    true,
  );
  addEventListener("resize", () => closeMenu(true));
  addEventListener("blur", () => closeMenu(true));

  // Mouse, touch and keyboard.

  function rowAt(event) {
    const rowNode = event.target.closest && event.target.closest(".ex-row[data-path]");
    return rowNode && tree.contains(rowNode) ? rowOf.get(rowNode.dataset.path) : null;
  }

  function toggleSelected(path) {
    if (selected.has(path)) selected.delete(path);
    else selected.add(path);
  }

  tree.addEventListener("click", event => {
    if (edit) return;
    const row = rowAt(event);
    tree.focus({ preventScroll: true });
    if (!row) {
      selected.clear();
      focus = anchor = null;
      return paint();
    }
    const path = row.path;
    if (event.ctrlKey || event.metaKey) {
      toggleSelected(path);
      focus = anchor = path;
      return paint();
    }
    if (event.shiftKey && anchor) {
      selectRange(path);
      focus = path;
      return paint();
    }
    selectOnly(path);
    if (row.dir) {
      setOpen(path, !expanded.has(path));
      return;
    }
    paint();
    page.open(path, "file");
  });
  // A Shift click or a double click selects rows, not text.
  tree.addEventListener("mousedown", event => {
    if (event.shiftKey || event.detail > 1) event.preventDefault();
  });

  /** menuAt opens the context menu of the row of event, or of the root, at x, y. */
  function menuAt(event, x, y) {
    const row = rowAt(event);
    if (row) {
      if (!selected.has(row.path)) {
        selected = new Set([row.path]);
        anchor = row.path;
      }
      focus = row.path;
      paint();
    }
    openMenu(x, y, row ? targets() : [root], false);
  }

  page.side.addEventListener("contextmenu", event => {
    if (event.target.closest(".ex-input")) return;
    event.preventDefault();
    // The context menu key gives an event at 0, 0: the menu then opens at the focused row.
    if (!event.clientX && !event.clientY && focus && rowOf.get(focus)) {
      const box = rowOf.get(focus).element.getBoundingClientRect();
      return openMenu(box.left + 24, box.bottom, targets(), true);
    }
    if (!tree.contains(event.target)) return openMenu(event.clientX, event.clientY, [root], false);
    menuAt(event, event.clientX, event.clientY);
  });
  longPress(tree, (x, y, event) => {
    if (!edit) menuAt(event, x, y);
  });

  /** findTyped adds key to the typed letters and calls moveFocus with the next row whose name starts with them. */
  function findTyped(key, index, moveFocus) {
    const now = Date.now();
    typed = now - typedAt < TYPE_AHEAD_MS ? typed + key.toLowerCase() : key.toLowerCase();
    typedAt = now;
    const start = typed.length > 1 ? Math.max(index, 0) : index + 1;
    for (let n = 0; n < rows.length; n++) {
      const candidate = (start + n) % rows.length;
      if (rows[candidate].name.toLowerCase().startsWith(typed)) {
        moveFocus(candidate);
        break;
      }
    }
  }

  tree.addEventListener("keydown", event => {
    if (edit || event.target !== tree) return;
    const modifier = event.ctrlKey || event.metaKey;
    const index = indexOf(focus);
    const row = rows[index];
    const moveFocus = target => {
      if (!rows.length) return;
      const path = rows[Math.max(0, Math.min(rows.length - 1, target))].path;
      focus = path;
      if (event.shiftKey) {
        selectRange(path);
      } else if (!modifier) {
        selected = new Set([path]);
        anchor = path;
      }
      paint();
      scrollTo(path);
    };
    const pageRows = Math.max(1, Math.floor(tree.clientHeight / ROW_HEIGHT) - 1);
    if (event.code === "KeyC" && event.altKey && event.shiftKey) {
      event.preventDefault();
      const paths = targets();
      if (modifier) return copyText(paths.map(relative).join("\n"), "relative path");
      return copyText(paths.join("\n"), "path");
    }
    switch (event.key) {
      case "ArrowDown":
        moveFocus(index < 0 ? 0 : index + 1);
        break;
      case "ArrowUp":
        moveFocus(index < 0 ? 0 : index - 1);
        break;
      case "Home":
        moveFocus(0);
        break;
      case "End":
        moveFocus(rows.length - 1);
        break;
      case "PageDown":
        moveFocus(index + pageRows);
        break;
      case "PageUp":
        moveFocus(index - pageRows);
        break;
      case "ArrowRight":
        if (!row) moveFocus(0);
        else if (row.dir && !expanded.has(row.path)) setOpen(row.path, true);
        else if (row.dir && rows[index + 1] && rows[index + 1].depth > row.depth) moveFocus(index + 1);
        break;
      case "ArrowLeft":
        if (!row) break;
        if (row.dir && expanded.has(row.path)) setOpen(row.path, false);
        else if (row.depth > 0) moveFocus(indexOf(parentOf(row.path)));
        break;
      case "Enter":
        if (!row) break;
        if (row.dir) setOpen(row.path, !expanded.has(row.path));
        else page.open(row.path, "file", true);
        break;
      case " ":
        if (!row) break;
        if (modifier) {
          toggleSelected(row.path);
          anchor = row.path;
          paint();
        } else if (row.dir) {
          setOpen(row.path, !expanded.has(row.path));
        } else {
          page.open(row.path, "file");
        }
        break;
      case "F2":
        if (row) startRename(row.path);
        break;
      case "Delete":
        remove(targets());
        break;
      case "Backspace":
        if (!event.metaKey) return;
        remove(targets());
        break;
      case "Escape":
        selected = new Set(focus ? [focus] : []);
        paint();
        break;
      default:
        if (modifier && event.key.toLowerCase() === "a") {
          selected = new Set(rows.map(each => each.path));
          paint();
          break;
        }
        if (event.key.length === 1 && !modifier && !event.altKey && rows.length) {
          findTyped(event.key, index, moveFocus);
          break;
        }
        return;
    }
    event.preventDefault();
  });

  // Drag and drop.

  function setDrop(dir) {
    dropDir = dir;
    tree.classList.toggle("ex-drop-root", dir === root);
    for (const row of rows) row.element.classList.toggle("ex-drop", !!dir && dir !== root && isUnder(row.path, dir));
  }

  function clearDrop() {
    setDrop(null);
    clearTimeout(hoverTimer);
    hoverDir = null;
  }

  /** dropFolderAt returns the folder of a drop on the row of event: the row when it is a folder, else its folder. */
  function dropFolderAt(event) {
    const row = rowAt(event);
    if (!row) return root;
    return row.dir ? row.path : parentOf(row.path);
  }

  const hasFiles = dataTransfer => !!dataTransfer && [...dataTransfer.types].includes("Files");

  tree.addEventListener("dragstart", event => {
    const row = rowAt(event);
    if (!row || edit) return event.preventDefault();
    if (!selected.has(row.path)) {
      selected = new Set([row.path]);
      anchor = row.path;
    }
    focus = row.path;
    paint();
    dragged = topLevel([...selected]);
    event.dataTransfer.effectAllowed = "copyMove";
    event.dataTransfer.setData("text/plain", dragged.map(relative).join("\n"));
    event.dataTransfer.setData("application/x-docgen-paths", JSON.stringify(dragged));
    for (const path of dragged) rowOf.get(path)?.element.classList.add("ex-cut");
  });
  tree.addEventListener("dragend", () => {
    dragged = null;
    clearDrop();
    for (const row of rows) row.element.classList.remove("ex-cut");
  });
  tree.addEventListener("dragover", event => {
    const dir = dropFolderAt(event);
    if (dragged) {
      if (dragged.some(path => isUnder(dir, path)) || dragged.every(path => parentOf(path) === dir)) {
        event.dataTransfer.dropEffect = "none";
        return setDrop(null);
      }
      event.preventDefault();
      event.dataTransfer.dropEffect = "move";
    } else if (hasFiles(event.dataTransfer)) {
      event.preventDefault();
      event.dataTransfer.dropEffect = "copy";
    } else {
      return;
    }
    if (dropDir !== dir) setDrop(dir);
    // A closed folder under the pointer opens after HOVER_EXPAND_MS.
    const row = rowAt(event);
    const hover = row && row.dir && !expanded.has(row.path) ? row.path : null;
    if (hover === hoverDir) return;
    clearTimeout(hoverTimer);
    hoverDir = hover;
    if (!hover) return;
    hoverTimer = setTimeout(() => {
      if (hoverDir !== hover) return;
      expanded.add(hover);
      saveOpenFolders();
      render();
    }, HOVER_EXPAND_MS);
  });
  tree.addEventListener("dragleave", event => {
    if (!event.relatedTarget || !tree.contains(event.relatedTarget)) clearDrop();
  });
  tree.addEventListener("drop", event => {
    event.preventDefault();
    const dir = dropFolderAt(event);
    clearDrop();
    if (dragged) {
      const paths = dragged;
      dragged = null;
      const error = moveTo(paths, dir);
      if (error && error !== "busy") page.note(error);
      return;
    }
    const dataTransfer = event.dataTransfer;
    if (!hasFiles(dataTransfer)) return;
    const items = [...dataTransfer.items]
      .filter(item => item.kind === "file")
      .map(item => (item.webkitGetAsEntry && item.webkitGetAsEntry()) || item.getAsFile());
    const dropped = items.length ? items : [...dataTransfer.files];
    readDrop(dropped).then(
      files => upload(dir, files),
      error => page.note("Could not read the dropped files: " + error),
    );
  });

  // The toolbar.

  const onToolbar = (id, action) => page.side.querySelector("#" + id).addEventListener("click", action);
  onToolbar("exnewfile", () => startNew("file"));
  onToolbar("exnewdir", () => startNew("folder"));
  onToolbar("exrefresh", () => render());
  onToolbar("excollapse", collapseAll);
  const header = page.side.querySelector("#exhead");
  header.addEventListener("dragover", event => {
    if (!dragged && !hasFiles(event.dataTransfer)) return;
    event.preventDefault();
    setDrop(root);
  });
  // A drop on the header goes to the root folder, as a drop on the empty part of the tree.
  header.addEventListener("drop", event => {
    tree.dispatchEvent(new DragEvent("drop", { dataTransfer: event.dataTransfer, bubbles: false }));
    event.preventDefault();
  });

  return {
    render,
    decorate: () => decorate(),
    reveal,
    openMenu,
    closeMenu,
    archive,
    state: () => ({
      expanded: [...expanded],
      selected: [...selected],
      focus,
      edit: edit && { ...edit },
      rows: rows.map(row => row.path),
      drop: dropDir,
    }),
  };
}
