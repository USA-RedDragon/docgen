// Browser test steps of the explorer and of the Markdown and SVG previews, for scenario.mjs.
// t has: ff, check, shot, runButton, load, R and outDir. The page is ready at 1280 px when the steps start.
import fs from "node:fs";
import { execFileSync } from "node:child_process";

const EL = "element-6066-11e4-a52e-4f735466cecf";
const K = { F2: "", Delete: "", Enter: "", Escape: "", Ctrl: "", Shift: "", Down: "", Up: "", Left: "", Right: "" };
const sleep = ms => new Promise(r => setTimeout(r, ms));
export const rowSel = p => `.ex-row[data-path="/work/dg/${p}"]`;

export function helpers(ff) {
  const find = async sel => { const r = await ff.cmd("WebDriver:FindElement", { using: "css selector", value: sel }); return Object.values(r.value || r)[0]; };
  const act = actions => ff.cmd("WebDriver:PerformActions", { actions }).then(() => ff.cmd("WebDriver:ReleaseActions", {}));
  const pointer = (id, steps) => ({ type: "pointer", id: "mouse", parameters: { pointerType: "mouse" }, actions: steps });
  const at = (eid, x = 0, y = 0) => ({ type: "pointerMove", origin: { [EL]: eid }, x, y, duration: 0 });
  const h = {
    find,
    click: async sel => ff.cmd("WebDriver:ElementClick", { id: await find(sel) }),
    modClick: async (sel, key) => {
      const id = await find(sel);
      await act([
        { type: "key", id: "kb", actions: [{ type: "keyDown", value: key }, { type: "pause", duration: 0 }, { type: "pause", duration: 0 }, { type: "pause", duration: 0 }, { type: "keyUp", value: key }] },
        pointer(id, [{ type: "pause", duration: 0 }, at(id), { type: "pointerDown", button: 0 }, { type: "pointerUp", button: 0 }, { type: "pause", duration: 0 }]),
      ]);
    },
    rightClick: async (sel, x = 0, y = 0) => { const id = await find(sel); await act([pointer(id, [at(id, x, y), { type: "pointerDown", button: 2 }, { type: "pointerUp", button: 2 }])]); },
    mouseDrag: async (from, to) => {
      const a = await find(from), b = await find(to);
      await act([pointer(a, [at(a, -40, 0), { type: "pointerDown", button: 0 }, { type: "pointerMove", origin: { [EL]: b }, x: 10, y: 0, duration: 300 }, { type: "pointerUp", button: 0 }])]);
    },
    keys: async (...ks) => act([{ type: "key", id: "kb", actions: ks.flatMap(k => Array.isArray(k) ? [...k.map(v => ({ type: "keyDown", value: v })), ...k.slice().reverse().map(v => ({ type: "keyUp", value: v }))] : [{ type: "keyDown", value: k }, { type: "keyUp", value: k }]) }]),
    type: async (sel, text) => ff.cmd("WebDriver:ElementSendKeys", { id: await find(sel), text }),
    menuItem: label => ff.js(`const b = [...document.querySelectorAll("#menu button")].find(b => b.firstChild.textContent === arguments[0]); if (!b || b.disabled) return false; b.click(); return true;`, [label]),
    menuLabels: () => ff.js(`return document.getElementById("menu").hidden ? null : [...document.querySelectorAll("#menu button")].map(b => b.firstChild.textContent + (b.disabled ? " (off)" : ""))`),
    state: () => ff.js("return window.__pg.explorer.state()"),
    focusTree: () => ff.js("document.getElementById('tree').focus()"),
    dialog: () => ff.js(`const d = document.getElementById("dlg"); return d.open ? { title: document.getElementById("dlgt").textContent, text: document.getElementById("dlgm").textContent, list: [...document.querySelectorAll("#dlgl li")].map(l => l.textContent) } : null`),
    // drag fires the HTML drag and drop events of a drag of row src onto row dst, with one DataTransfer. Without drop it stops at dragover.
    drag: (src, dst, drop = true, hoverMs = 0) => ff.ajs(`
      const [src, dst, drop, hoverMs, done] = arguments;
      const a = document.querySelector(src), b = document.querySelector(dst), dt = new DataTransfer();
      const r = b.getBoundingClientRect(), x = r.left + 30, y = r.top + r.height / 2;
      const fire = (el, type) => el.dispatchEvent(new DragEvent(type, { bubbles: true, cancelable: true, dataTransfer: dt, clientX: x, clientY: y }));
      fire(a, "dragstart"); fire(b, "dragenter");
      const over = fire(b, "dragover");
      const accepted = !over;
      setTimeout(() => {
        const b2 = document.querySelector(dst) || b;
        fire(b2, "dragover");
        if (drop) { fire(b2, "drop"); fire(a.isConnected ? a : document.getElementById("tree"), "dragend"); }
        done({ accepted, drop: window.__pg.explorer.state().drop });
      }, hoverMs);`, [src, dst, drop, hoverMs]),
    endDrag: () => ff.js(`document.getElementById("tree").dispatchEvent(new DragEvent("dragend", { bubbles: true }))`),
    // dropFiles fires a drop of OS files [[name, text]] onto row dst, as a file manager does.
    dropFiles: (dst, files) => ff.js(`
      const [dst, files] = arguments;
      const b = document.querySelector(dst), dt = new DataTransfer();
      for (const [n, t] of files) dt.items.add(new File([t], n, { type: "text/plain" }));
      const r = b.getBoundingClientRect(), o = { bubbles: true, cancelable: true, dataTransfer: dt, clientX: r.left + 30, clientY: r.top + 5 };
      b.dispatchEvent(new DragEvent("dragenter", o));
      const accepted = !b.dispatchEvent(new DragEvent("dragover", o));
      b.dispatchEvent(new DragEvent("drop", o));
      return accepted;`, [dst, files]),
    longPress: (sel, ms = 700) => ff.ajs(`
      const [sel, ms, done] = arguments;
      const el = document.querySelector(sel), r = el.getBoundingClientRect();
      const o = { bubbles: true, cancelable: true, pointerType: "touch", pointerId: 7, isPrimary: true, clientX: r.left + 40, clientY: r.top + r.height / 2 };
      el.dispatchEvent(new PointerEvent("pointerdown", o));
      setTimeout(() => { el.dispatchEvent(new PointerEvent("pointerup", o)); el.dispatchEvent(new MouseEvent("click", o)); done(!document.getElementById("menu").hidden); }, ms);`, [sel, ms]),
  };
  return h;
}

export async function explorerSteps(t) {
  const { ff, check, shot } = t;
  const h = helpers(ff);
  const until = (e, ms = 10000) => ff.until(e, ms, 100);
  const iconOfRow = p => ff.js(`const r = document.querySelector(arguments[0]); return r && r.querySelector(".ex-icon use").getAttribute("href")`, [rowSel(p)]);
  const tabsNow = () => ff.js("return window.__pg.tabs()");
  const cur = () => ff.js("return window.__pg.current()");
  const notes = () => ff.js("return window.__pg.notes.slice(-3)");
  const exists = p => ff.js("return window.__pg.exists(arguments[0])", [p]);
  await ff.js("window.__pg.setTheme('light')");

  // 1. No text selection in the explorer; selection still works in the output.
  check("explorer: user-select none", await ff.js("return getComputedStyle(document.getElementById('side')).userSelect === 'none'"));
  check("explorer: editor and output keep text selection", await ff.js("return getComputedStyle(document.getElementById('out')).userSelect !== 'none' && getComputedStyle(document.querySelector('.cm-content') || document.body).userSelect !== 'none'"));
  await ff.js("getSelection().removeAllRanges()");
  await h.mouseDrag(".ex-head > span", rowSel("machine/port.go"));
  const selTree = await ff.js("return getSelection().toString()");
  await h.mouseDrag("#out", ".term-head");
  const selOut = await ff.js("return getSelection().toString().length");
  check("explorer: a mouse drag across the tree selects no text", selTree === "", { tree: selTree, outputChars: selOut });
  check("explorer: the same drag selects text in the output", selOut > 0, selOut);
  await ff.js("getSelection().removeAllRanges()");

  // 2. Icons, order, indent guides.
  const icons = { "machine/machine.go": "#i-go", "docs/lamp.md": "#i-md", "docs/states/machine-lamp.svg": "#i-svg", "go.mod": "#i-mod", docs: "#i-fo" };
  const got = {};
  for (const p of Object.keys(icons)) got[p] = await iconOfRow(p);
  check("explorer: icons by type", Object.entries(icons).every(([p, i]) => got[p] === i), got);
  const order = await ff.js(`return [...document.querySelectorAll(".ex-row[aria-level='2']")].filter(r => r.dataset.path.startsWith("/work/dg/")).map(r => r.dataset.path.slice(9))`);
  check("explorer: folders first, then files by name", JSON.stringify(order) === JSON.stringify(["cmd", "docs", "machine", "doc.go", "go.mod"]), order);
  check("explorer: indent guides", await ff.js(`return getComputedStyle(document.querySelector(arguments[0] + " .ex-ind")).width === "24px"`, [rowSel("docs/lamp.md")]));
  await shot("1280-light-explorer");

  // 3. Collapse, expand and reveal of the active file when a tab is chosen.
  await h.click(rowSel("machine"));
  check("explorer: click collapses a folder", !(await h.state()).expanded.includes("/work/dg/machine"));
  await ff.js(`[...document.querySelectorAll(".vs-tab")].find(t => t.dataset.path.endsWith("machine.go") && t.dataset.kind === "file").querySelector("button").click()`);
  const st3 = await h.state();
  check("explorer: choosing a tab reveals and selects its file", st3.expanded.includes("/work/dg/machine") && st3.selected.join() === "/work/dg/machine/machine.go", st3.selected);
  await h.click("#tree .ex-row[data-path='/work/loop']");
  await h.click("#exrefresh");
  check("explorer: collapse state kept over a refresh", !(await h.state()).expanded.includes("/work/loop"));

  // 4. Keyboard navigation.
  await h.focusTree();
  await h.click(rowSel("docs"));
  const kc = await h.state();
  await h.keys(K.Right, K.Down, K.Down);
  const kn = await h.state();
  await h.keys(K.Left);
  const kl = await h.state();
  await h.keys(K.Down, K.Enter);
  const ko = await cur();
  check("explorer: arrows move, Left goes to the parent, Enter opens", !kc.expanded.includes("/work/dg/docs") && kn.expanded.includes("/work/dg/docs") && kn.focus === "/work/dg/docs/states/machine-lamp.svg" && kl.focus === "/work/dg/docs/states" && ko && ko.path === "dg/docs/states/machine-lamp.svg", { down: kn.focus, left: kl.focus, opened: ko });
  await h.focusTree();

  // 5. SVG source and preview, like VS Code: the explorer opens the source.
  await h.click(rowSel("docs/states/machine-lamp.svg"));
  const sv = await cur();
  const svText = await ff.js("return window.__pg.editorText()");
  check("svg: the explorer opens the XML source in the editor", sv.kind === "file" && sv.code && sv.lang === "SVG" && /^\s*<(\?xml|svg)/.test(svText), { sv, head: svText.slice(0, 40) });
  check("svg: XML highlighting", await ff.js("return document.querySelectorAll('.cm-content span[class]').length > 5"));
  check("svg: Open Preview shown on the source", await ff.js("return !document.getElementById('openpv').hidden && document.getElementById('showsrc').hidden"));
  await shot("1280-light-svg-source");
  await h.click("#openpv");
  await until("window.__pg.previewImages().length && window.__pg.previewImages()[0].complete");
  const svp = await ff.js("return window.__pg.previewImages()");
  check("svg: Open Preview renders the memfs bytes as an image", (await cur()).kind === "preview" && svp[0].w > 0 && svp[0].src.startsWith("blob:"), svp);
  check("svg: Show Source shown on the preview", await ff.js("return !document.getElementById('showsrc').hidden && document.getElementById('openpv').hidden"));
  await shot("1280-light-svg-preview");
  await h.click("#showsrc");
  check("svg: Show Source goes back to the source tab", (await cur()).kind === "file");
  await h.click("#openside");
  await until("window.__pg.sideImages().length && window.__pg.sideImages()[0].w > 0");
  const before = (await ff.js("return window.__pg.sideImages()"))[0];
  const svgSrc = await ff.js("return window.__pg.editorText()");
  const m = svgSrc.match(/<\/svg>\s*$/);
  check("svg: source ends with </svg>", !!m);
  await ff.js(`window.__pg.replace("docs/states/machine-lamp.svg", "</svg>", '<rect x="0" y="0" width="40" height="40" fill="#e11"/></svg>')`);
  await until(`window.__pg.sideImages().length && window.__pg.sideImages()[0].src !== ${JSON.stringify(before.src)} && window.__pg.sideImages()[0].w > 0`);
  check("svg: the side preview updates after an edit of the source", true);
  await shot("1280-light-svg-side");
  await ff.js("document.getElementById('sideclose').click()");

  // 6. Markdown: source, preview, and a side preview that updates after edits.
  await h.click(rowSel("docs/lamp.md"));
  const mdc = await cur();
  check("md: the explorer opens the Markdown source", mdc.kind === "file" && mdc.code && mdc.lang === "Markdown", mdc);
  await h.click("#openpv");
  check("md: Open Preview opens the preview tab", (await cur()).kind === "preview" && (await ff.js("return window.__pg.previewText()")).includes("The lamp"));
  await h.click("#showsrc");
  await h.click("#openside");
  await until("window.__pg.sideText() && window.__pg.sideText().includes('The lamp')");
  const r0 = await ff.js("return window.__pg.renders");
  await ff.js(`window.__pg.replace("docs/lamp.md", "# The lamp", "# The lamp, edited live")`);
  await until("window.__pg.sideText().includes('The lamp, edited live')");
  check("md: the side preview updates after edits", true, { renders: (await ff.js("return window.__pg.renders")) - r0 });
  await shot("1280-light-md-side");
  await h.click("#openpv");
  check("md: the preview tab shows the edit", (await ff.js("return window.__pg.previewText()")).includes("The lamp, edited live"));
  await ff.js(`window.__pg.replace("docs/lamp.md", "# The lamp, edited live", "# The lamp")`);
  await ff.js("document.getElementById('sideclose').click()");
  await h.rightClick(rowSel("docs/lamp.md"));
  const mdMenu = await h.menuLabels();
  check("md: the context menu has Open Preview and Open Preview to the Side", mdMenu && mdMenu.includes("Open Preview") && mdMenu.includes("Open Preview to the Side"), mdMenu);
  await h.keys(K.Escape);

  // 7. Drag a file between folders, with the drop highlight and auto-expand; a folder cannot move into itself.
  await ff.js("window.__pg.explorer.reveal('/work/dg/machine/port.go')");
  await h.click(rowSel("machine/port.go"));
  await h.click(rowSel("cmd"));
  const hov = await h.drag(rowSel("machine/port.go"), rowSel("cmd"), false, 800);
  await shot("1280-light-explorer-drag");
  check("dnd: drop target highlighted and auto-expanded on hover", hov.accepted && hov.drop === "/work/dg/cmd" && (await h.state()).expanded.includes("/work/dg/cmd") && await ff.js(`return document.querySelector(arguments[0]).classList.contains("ex-drop")`, [rowSel("cmd/lamp")]), hov);
  await h.endDrag();
  const mv = await h.drag(rowSel("machine/port.go"), rowSel("docs"));
  check("dnd: a file moves to another folder", mv.accepted && await exists("docs/port.go") && !(await exists("machine/port.go")), mv);
  const tp = await tabsNow();
  check("dnd: the open tab gets the new path after the move", tp.some(x => x.path === "dg/docs/port.go") && !tp.some(x => x.path === "dg/machine/port.go"), tp);
  check("dnd: badge U on the moved file", (await ff.js("return window.__pg.marks()"))["dg/docs/port.go"] === "U" && await ff.js(`return document.querySelector(arguments[0]).textContent.endsWith("U")`, [rowSel("docs/port.go")]));
  const self = await h.drag(rowSel("docs"), rowSel("docs/states"));
  check("dnd: a folder cannot move into itself", !self.accepted && await exists("docs/lamp.md") && (await notes()).at(-1).includes("into itself"), { self, note: (await notes()).at(-1) });

  // 8. Rename by F2, with the name checks inline.
  await h.click(rowSel("docs/port.go"));
  await h.keys(K.F2);
  await until("document.querySelector('.ex-input')");
  const inp = ".ex-input";
  const tryName = async name => { await ff.js(`const i = document.querySelector(".ex-input"); i.value = ""; i.dispatchEvent(new Event("input"))`); await h.type(inp, name); return ff.js(`const m = document.querySelector(".ex-msg"); return m.hidden ? "" : m.textContent`); };
  const errDup = await tryName("lamp.md");
  await shot("1280-light-explorer-rename-error");
  const errSlash = await tryName("a/b.go"), errDots = await tryName(".."), errEmpty = await tryName("  ");
  await h.keys(K.Enter);
  const stillEditing = await ff.js("return !!document.querySelector('.ex-input')");
  check("rename: a duplicate, a slash, .. and an empty name show an inline error", errDup.includes("already exists") && errSlash.includes("slash") && errDots.includes("not valid") && errEmpty.includes("must be provided") && stillEditing, { errDup, errSlash, errDots, errEmpty, stillEditing });
  await tryName("plug.go");
  await h.keys(K.Enter);
  await until("!document.querySelector('.ex-input')");
  const tr = await tabsNow();
  check("rename: F2 renames the file in the memfs", await exists("docs/plug.go") && !(await exists("docs/port.go")));
  check("rename: the open tab gets the new path after the rename", tr.some(x => x.path === "dg/docs/plug.go") && !tr.some(x => x.path === "dg/docs/port.go"), tr);
  await ff.js(`[...document.querySelectorAll(".vs-tab")].find(t => t.dataset.path.endsWith("plug.go")).querySelector("button").click()`);
  check("rename: the editor of the renamed file still writes to it", (await cur()).lang === "Go" && await ff.js(`window.__pg.cursorEnd(); return true`) && (await ff.js(`window.__pg.replace("docs/plug.go", "package machine", "package machine // renamed"); return window.__pg.text("docs/plug.go").includes("// renamed")`)));

  // 9. Context menu: New File, Duplicate, Delete with a confirm in the page; New Folder from the toolbar.
  await h.rightClick(rowSel("docs"));
  const dmenu = await h.menuLabels();
  check("menu: right-click opens the context menu", dmenu && ["New File...", "New Folder...", "Rename...", "Delete", "Duplicate", "Copy Path", "Copy Relative Path", "Download", "Collapse All"].every(l => dmenu.includes(l)), dmenu);
  await shot("1280-light-explorer-menu");
  await h.menuItem("New File...");
  await until("document.querySelector('.ex-input')");
  await h.type(inp, "notes.md");
  await h.keys(K.Enter);
  await until("window.__pg.exists('docs/notes.md')");
  const nc = await cur();
  check("menu: New File creates the file and opens its source", nc.path === "dg/docs/notes.md" && nc.kind === "file" && nc.code, nc);
  await ff.js(`window.__pg.cursorEnd()`);
  await h.type(".cm-content", "# Notes\n\nSome text.");
  check("menu: typing in the new file reaches the memfs", (await ff.js("return window.__pg.text('docs/notes.md')")).includes("Some text."));
  await h.rightClick(rowSel("docs/notes.md"));
  await h.menuItem("Duplicate");
  check("menu: Duplicate copies the file", await exists("docs/notes copy.md") && (await ff.js("return window.__pg.text('docs/notes copy.md')")).includes("Some text."));
  await h.rightClick(rowSel("docs/notes copy.md"));
  await h.menuItem("Delete");
  await until("document.getElementById('dlg').open");
  const dd = await h.dialog();
  await shot("1280-light-explorer-delete-confirm");
  check("menu: Delete opens a confirmation dialog in the page", dd && dd.title === "Delete notes copy.md?", dd);
  await h.click("#dlgok");
  await until("!document.getElementById('dlg').open");
  check("menu: Delete removes the file after the confirm", !(await exists("docs/notes copy.md")));
  await h.rightClick(rowSel("docs/plug.go"));
  await h.menuItem("Delete");
  await until("document.getElementById('dlg').open");
  await h.keys(K.Escape);
  await until("!document.getElementById('dlg').open");
  check("menu: Cancel keeps the file", await exists("docs/plug.go"));
  await h.rightClick(rowSel("docs/lamp.md"));
  await h.menuItem("Copy Path");
  const cp = await ff.js("return window.__pg.copied");
  await h.rightClick(rowSel("docs/lamp.md"));
  await h.menuItem("Copy Relative Path");
  const crp = await ff.js("return window.__pg.copied");
  check("menu: Copy Path and Copy Relative Path", cp === "/work/dg/docs/lamp.md" && crp === "dg/docs/lamp.md", { cp, crp });
  await h.click(rowSel("docs"));
  await h.click("#exnewdir");
  await until("document.querySelector('.ex-input')");
  await h.type(inp, "extra");
  await h.keys(K.Enter);
  check("toolbar: New Folder creates a folder in the selected folder", await ff.js("return window.__pg.isDir('docs/extra')"));

  // 10. Upload by a drop of OS files on a folder.
  const acc = await h.dropFiles(rowSel("docs/extra"), [["a.txt", "alpha\n"], ["b.json", '{"b": 1}\n']]);
  await until("window.__pg.exists('docs/extra/b.json')");
  check("upload: files dropped from the OS land in the folder", acc && (await ff.js("return window.__pg.text('docs/extra/a.txt')")) === "alpha\n" && (await iconOfRow("docs/extra/b.json")) === "#i-json");

  // 11. Multi-select with Ctrl and Shift, and Delete of the selection with a note for an open file with edits.
  await h.click(rowSel("docs/extra/a.txt"));
  await h.modClick(rowSel("docs/extra/b.json"), K.Ctrl);
  await h.modClick(rowSel("docs/notes.md"), K.Ctrl);
  const ms = await h.state();
  check("select: Ctrl adds to the selection", ms.selected.length === 3, ms.selected);
  await h.click(rowSel("docs/extra/a.txt"));
  await h.modClick(rowSel("docs/states"), K.Shift);
  check("select: Shift selects a range", (await h.state()).selected.length === 3, (await h.state()).selected);
  await h.click(rowSel("docs/extra/a.txt"));
  await h.modClick(rowSel("docs/extra/b.json"), K.Ctrl);
  await h.modClick(rowSel("docs/notes.md"), K.Ctrl);
  await h.keys(K.Delete);
  await until("document.getElementById('dlg').open");
  const md3 = await h.dialog();
  check("select: Delete of 3 items lists them and names the edits", md3.title === "Delete 3 items?" && md3.list.length === 3 && md3.text.includes("dg/docs/notes.md"), md3);
  await h.click("#dlgok");
  await until("!window.__pg.exists('docs/notes.md')");
  const tn = await tabsNow(), nn = (await notes()).at(-1);
  check("select: the files are gone, the tab closed, with a note on the lost edits", !(await exists("docs/extra/a.txt")) && !tn.some(x => x.path === "dg/docs/notes.md") && nn.includes("notes.md") && nn.includes("edits"), { nn, tabs: tn });

  // 12. Download: a folder is a zip.
  const z = await ff.ajs(`const [done] = arguments; const a = window.__pg.explorer.archive(["/work/dg/docs"]); a.blob.arrayBuffer().then(b => { let s = ""; const u = new Uint8Array(b); for (let i = 0; i < u.length; i++) s += String.fromCharCode(u[i]); done({ name: a.name, b64: btoa(s) }); });`);
  const zip = `${t.outDir}/docs.zip`;
  fs.writeFileSync(zip, Buffer.from(z.b64, "base64"));
  let zl = "";
  try { zl = execFileSync("python3", ["-I", "-c", "import sys, zipfile; z = zipfile.ZipFile(sys.argv[1]); assert z.testzip() is None; print('\\n'.join(z.namelist()))", zip]).toString(); } catch (e) { zl = "bad zip: " + e; }
  check("download: a folder downloads as a valid zip", z.name === "docs.zip" && zl.includes("docs/lamp.md") && zl.includes("docs/states/machine-lamp.svg") && zl.includes("docs/extra/"), zl);
  await h.rightClick(rowSel("docs/lamp.md"));
  await h.menuItem("Download");
  const dl = await ff.js("return window.__pg.download && { name: window.__pg.download.name, size: window.__pg.download.size }");
  check("download: a file downloads as itself", dl && dl.name === "lamp.md" && dl.size > 1000, dl);

  // 13. Reload: the workspace is kept.
  await ff.js("return window.__pg.saveNow()");
  const kept = await ff.js("return window.__pg.marks()");
  await t.load("", "reload with a saved workspace");
  const after = await ff.js("return { marks: window.__pg.marks(), restored: window.__pg.restored, storage: window.__pg.storage, tabs: window.__pg.tabs(), plug: window.__pg.text('docs/plug.go').includes('// renamed'), extra: window.__pg.isDir('docs/extra') }");
  check("persist: the workspace is kept over a reload", after.restored && JSON.stringify(after.marks) === JSON.stringify(kept) && after.plug && after.extra, after);
  check("persist: open tabs are kept", after.tabs.some(x => x.path === "dg/docs/plug.go"), after.tabs);
  check("persist: collapse state is kept", !(await h.state()).expanded.includes("/work/loop"));
  await ff.js("window.__pg.setTheme('dark')");
  await h.rightClick(rowSel("docs/plug.go"));
  await shot("1280-dark-explorer-menu");
  await h.keys(K.Escape);
  await ff.js("window.__pg.open('docs/lamp.md'); document.getElementById('openside').click()");
  await until("window.__pg.sideText() && window.__pg.sideText().includes('The lamp')");
  await shot("1280-dark-md-side");
  await ff.js("document.getElementById('sideclose').click()");
  await h.click(rowSel("docs/states/machine-lamp.svg"));
  await shot("1280-dark-svg-source");
  await ff.js("window.__pg.setTheme('light')");

  await h.click("#excollapse");
  const ca = await h.state();
  check("toolbar: Collapse All", ca.expanded.length === 0 && ca.rows.length === 2, ca.rows);
  await ff.js("window.__pg.explorer.reveal('/work/dg/docs/lamp.md')");

  // 14. Reset Sample from the menu of the root.
  await h.rightClick(".ex-head > span");
  const rmenu = await h.menuLabels();
  check("reset: the root menu has Reset Sample", rmenu && rmenu.includes("Reset Sample..."), rmenu);
  await h.menuItem("Reset Sample...");
  await until("document.getElementById('dlg').open");
  await shot("1280-light-reset-confirm");
  await h.click("#dlgok");
  await until("window.__pg.exists('machine/port.go')");
  check("reset: starter files back, no marks", !(await exists("docs/plug.go")) && !(await ff.js("return window.__pg.isDir('docs/extra')")) && Object.keys(await ff.js("return window.__pg.marks()")).length === 0, await ff.js("return window.__pg.marks()"));
  await t.load("", "reload after Reset Sample");
  check("reset: a reload after the reset shows the starter files", !(await exists("docs/plug.go")) && Object.keys(await ff.js("return window.__pg.marks()")).length === 0);

  // 15. A rename by F2, then docgen update: the memfs and docgen agree.
  await h.click(rowSel("docs/lamp.md"));
  await h.keys(K.F2);
  await until("document.querySelector('.ex-input')");
  await ff.js(`const i = document.querySelector(".ex-input"); i.value = ""`);
  await h.type(inp, "lamp-guide.md");
  await h.keys(K.Enter);
  await until("window.__pg.exists('docs/lamp-guide.md')");
  await ff.js(`window.__pg.replace("machine/machine.go", "const DefaultWait = 250 * time.Millisecond", "const DefaultWait = 700 * time.Millisecond")`);
  await ff.js("window.__pg.open('docs/lamp-guide.md')");
  const up = await t.runButton("update", "docgen update after a rename in the explorer");
  const guide = await ff.js("return window.__pg.text('docs/lamp-guide.md')");
  check("rename+update: docgen update writes the renamed page", up.code === 0 && guide.includes("`700ms`") && up.changed.includes("dg/docs/lamp-guide.md") && !(await exists("docs/lamp.md")), { status: up.status, changed: up.changed });
  check("rename+update: the renamed tab shows the new text", (await ff.js("return window.__pg.editorText()")).includes("`700ms`") && (await cur()).path === "dg/docs/lamp-guide.md");
  const ck = await t.runButton("check", "docgen check after the rename");
  check("rename+update: docgen check is clean", ck.code === 0, ck.status);
  await shot("1280-light-after-rename-update");
  await ff.js("return window.__pg.resetSample()");
  await until("window.__pg.exists('docs/lamp.md')");
}

/** narrowSteps run in the 390 px frame (the test is switched into it). shot390 takes a screenshot of the frame. */
export async function narrowSteps(t) {
  const { ff, check, shot390 } = t;
  const h = helpers(ff);
  const until = (e, ms = 10000) => ff.until(e, ms, 100);
  check("390: dropdown has the SVG preview", await ff.js(`return [...document.getElementById("pick").options].some(o => o.value === "preview:/work/dg/docs/states/machine-lamp.svg")`));
  for (const theme of ["light", "dark"]) {
    await ff.js(`window.__pg.setTheme(${JSON.stringify(theme)})`);
    await ff.js("document.getElementById('files').click()");
    check(`390 ${theme}: the files button opens the explorer drawer`, await ff.js("return getComputedStyle(document.getElementById('side')).display === 'flex' && document.getElementById('side').getBoundingClientRect().width > 200"));
    await shot390(`390-${theme}-explorer-drawer`);
    const opened = await h.longPress(rowSel("docs/lamp.md"));
    const labels = await h.menuLabels();
    check(`390 ${theme}: a long press opens the menu with the same actions`, opened && labels.includes("Rename...") && labels.includes("Delete") && labels.includes("Open Preview") && !labels.includes("Open Preview to the Side"), labels);
    await shot390(`390-${theme}-explorer-menu`);
    await ff.js("document.getElementById('menu').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))");
    await ff.js("document.getElementById('scrim').click()");
    check(`390 ${theme}: the drawer closes`, await ff.js("return getComputedStyle(document.getElementById('side')).display === 'none'"));
  }
  await ff.js("document.getElementById('more').click()");
  const ml = await h.menuLabels();
  check("390: the actions button opens the menu of the current file", ml && ml.includes("Rename..."), ml);
  await h.menuItem("Rename...");
  check("390: Rename from the menu opens the drawer with the name field", await ff.js("return document.getElementById('vs').classList.contains('ex-open') && !!document.querySelector('.ex-input')"));
  await shot390("390-dark-explorer-rename");
  await ff.js("document.querySelector('.ex-input').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))");
  await ff.js("document.getElementById('scrim').click()");
  await ff.js("window.__pg.setTheme('light')");
}
