// The explorer: its look, selection, keyboard, drag and drop, rename, context menu, upload, download,
// the saved workspace and Reset Sample.
import fs from "node:fs";
import { execFileSync } from "node:child_process";
import { test, expect, rowSelector, RUN_MS } from "../fixtures.mjs";

/** mouseDrag drags the mouse from 40 px left of the center of from to 10 px right of the center of to. */
async function mouseDrag(page, from, to) {
  const start = await page.locator(from).boundingBox();
  const end = await page.locator(to).boundingBox();
  await page.mouse.move(start.x + start.width / 2 - 40, start.y + start.height / 2);
  await page.mouse.down();
  await page.mouse.move(end.x + end.width / 2 + 10, end.y + end.height / 2, { steps: 10 });
  await page.mouse.up();
}

/**
 * startDrag fires dragstart of the row from and dragenter and dragover on the row to, as a drag in
 * the tree does, and returns true when the tree accepts the drop there.
 */
function startDrag(playground, from, to) {
  return playground.js(
    ([from, to]) => {
      const source = document.querySelector(from);
      const target = document.querySelector(to);
      window.__dragData = new DataTransfer();
      const box = target.getBoundingClientRect();
      window.__dragPoint = { clientX: box.left + 30, clientY: box.top + box.height / 2 };
      const fire = (node, type) =>
        node.dispatchEvent(new DragEvent(type, { bubbles: true, cancelable: true, dataTransfer: window.__dragData, ...window.__dragPoint }));
      window.__dragSource = source;
      fire(source, "dragstart");
      fire(target, "dragenter");
      return !fire(target, "dragover");
    },
    [from, to],
  );
}

/** endDrag fires dragover and, with drop, drop on the row to, then dragend. */
function endDrag(playground, to, drop = true) {
  return playground.js(
    ([to, drop]) => {
      const target = document.querySelector(to);
      const fire = (node, type) =>
        node.dispatchEvent(new DragEvent(type, { bubbles: true, cancelable: true, dataTransfer: window.__dragData, ...window.__dragPoint }));
      fire(target, "dragover");
      if (drop) fire(target, "drop");
      const source = window.__dragSource;
      fire(source.isConnected ? source : document.getElementById("tree"), "dragend");
    },
    [to, drop],
  );
}

/** drag drags the row from onto the row to, and returns true when the tree accepted the drop. */
async function drag(playground, from, to) {
  const accepted = await startDrag(playground, from, to);
  await endDrag(playground, to);
  return accepted;
}

async function iconOfRow(playground, path) {
  return playground.row(path).locator(".ex-icon use").getAttribute("href");
}

test("the explorer selects no text, and the editor and the output do", async ({ playground, page }) => {
  await playground.runCommand("echo some output");
  expect(await page.locator("#side").evaluate(node => getComputedStyle(node).userSelect)).toBe("none");
  expect(await page.locator("#out").evaluate(node => getComputedStyle(node).userSelect)).not.toBe("none");
  expect(await page.locator(".cm-content").evaluate(node => getComputedStyle(node).userSelect)).not.toBe("none");
  await playground.js(() => getSelection().removeAllRanges());
  await mouseDrag(page, ".ex-head > span", rowSelector("machine/port.go"));
  expect(await playground.js(() => getSelection().toString())).toBe("");
  await mouseDrag(page, "#out", ".term-head");
  expect(await playground.js(() => getSelection().toString().length)).toBeGreaterThan(0);
});

test("the explorer shows icons by type, folders first and indent guides", async ({ playground, page }) => {
  const icons = {
    "machine/machine.go": "#i-go",
    "docs/lamp.md": "#i-md",
    "docs/states/machine-lamp.svg": "#i-svg",
    "go.mod": "#i-mod",
    docs: "#i-fo",
  };
  for (const [path, icon] of Object.entries(icons)) expect(await iconOfRow(playground, path), path).toBe(icon);
  const order = await playground.js(() =>
    [...document.querySelectorAll(".ex-row[aria-level='2']")]
      .filter(row => row.dataset.path.startsWith("/work/dg/"))
      .map(row => row.dataset.path.slice(9)),
  );
  expect(order).toEqual(["cmd", "docs", "machine", "doc.go", "go.mod"]);
  await expect(page.locator(rowSelector("docs/lamp.md") + " .ex-ind")).toHaveCSS("width", "24px");
});

test("a click collapses a folder, a tab reveals its file, and a refresh keeps the folders", async ({ playground, page }) => {
  await playground.row("machine").click();
  expect((await playground.state()).expanded).not.toContain("/work/dg/machine");
  await playground.clickTab("machine/machine.go");
  const state = await playground.state();
  expect(state.expanded).toContain("/work/dg/machine");
  expect(state.selected).toEqual(["/work/dg/machine/machine.go"]);
  await page.locator('#tree .ex-row[data-path="/work/loop"]').click();
  await page.locator("#exrefresh").click();
  expect((await playground.state()).expanded).not.toContain("/work/loop");
});

test("arrows move in the tree, Left goes to the parent, and Enter opens", async ({ playground, page }) => {
  await playground.row("docs").click();
  expect((await playground.state()).expanded).not.toContain("/work/dg/docs");
  await page.keyboard.press("ArrowRight");
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("ArrowDown");
  const down = await playground.state();
  expect(down.expanded).toContain("/work/dg/docs");
  expect(down.focus).toBe("/work/dg/docs/states/machine-lamp.svg");
  await page.keyboard.press("ArrowLeft");
  expect((await playground.state()).focus).toBe("/work/dg/docs/states");
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  expect((await playground.current()).path).toBe("dg/docs/states/machine-lamp.svg");
});

test("a drag moves a file to another folder, and a folder cannot move into itself", async ({ playground }) => {
  await playground.open("machine/port.go");
  await playground.row("cmd").click();
  expect(await startDrag(playground, rowSelector("machine/port.go"), rowSelector("cmd"))).toBe(true);
  await playground.until(() => window.__pg.explorer.state().expanded.includes("/work/dg/cmd"));
  expect((await playground.state()).drop).toBe("/work/dg/cmd");
  await expect(playground.row("cmd/lamp")).toHaveClass(/ex-drop/);
  await endDrag(playground, rowSelector("cmd"), false);

  expect(await drag(playground, rowSelector("machine/port.go"), rowSelector("docs"))).toBe(true);
  expect(await playground.exists("docs/port.go")).toBe(true);
  expect(await playground.exists("machine/port.go")).toBe(false);
  const tabs = await playground.tabs();
  expect(tabs.map(tab => tab.path)).toContain("dg/docs/port.go");
  expect(tabs.map(tab => tab.path)).not.toContain("dg/machine/port.go");
  expect((await playground.marks())["dg/docs/port.go"]).toBe("U");
  await expect(playground.row("docs/port.go")).toHaveText(/U$/);

  expect(await drag(playground, rowSelector("docs"), rowSelector("docs/states"))).toBe(false);
  expect(await playground.exists("docs/lamp.md")).toBe(true);
  expect(await playground.lastNote()).toContain("into itself");
});

test("F2 renames a file, with the name checks in the tree", async ({ playground, page }) => {
  await playground.open("machine/port.go");
  await playground.row("machine/port.go").click();
  await page.keyboard.press("F2");
  await expect(page.locator(".ex-input")).toBeVisible();
  expect(await playground.nameError("machine.go")).toContain("already exists");
  expect(await playground.nameError("a/b.go")).toContain("slash");
  expect(await playground.nameError("..")).toContain("not valid");
  expect(await playground.nameError("  ")).toContain("must be provided");
  await page.locator(".ex-input").press("Enter");
  await expect(page.locator(".ex-input")).toBeVisible();
  await playground.nameError("plug.go");
  await page.locator(".ex-input").press("Enter");
  await expect(page.locator(".ex-input")).toHaveCount(0);
  expect(await playground.exists("machine/plug.go")).toBe(true);
  expect(await playground.exists("machine/port.go")).toBe(false);
  const tabs = (await playground.tabs()).map(tab => tab.path);
  expect(tabs).toContain("dg/machine/plug.go");
  expect(tabs).not.toContain("dg/machine/port.go");
  await playground.clickTab("plug.go");
  expect((await playground.current()).lang).toBe("Go");
  await playground.replace("machine/plug.go", "package machine", "package machine // renamed");
  expect(await playground.text("machine/plug.go")).toContain("// renamed");
});

test("the context menu creates, duplicates, deletes and copies paths", async ({ playground, page }) => {
  await playground.row("docs").click({ button: "right" });
  const labels = await playground.menuLabels();
  for (const label of ["New File...", "New Folder...", "Rename...", "Delete", "Duplicate", "Copy Path", "Copy Relative Path", "Download", "Collapse All"]) {
    expect(labels).toContain(label);
  }
  await playground.menuItem("New File...");
  await page.locator(".ex-input").fill("notes.md");
  await page.locator(".ex-input").press("Enter");
  await playground.until(() => window.__pg.exists("docs/notes.md"));
  expect(await playground.current()).toMatchObject({ path: "dg/docs/notes.md", kind: "file", code: true });
  await playground.js(() => window.__pg.cursorEnd());
  await page.keyboard.type("# Notes\n\nSome text.");
  expect(await playground.text("docs/notes.md")).toContain("Some text.");

  await playground.row("docs/notes.md").click({ button: "right" });
  await playground.menuItem("Duplicate");
  expect(await playground.text("docs/notes copy.md")).toContain("Some text.");

  await playground.row("docs/notes copy.md").click({ button: "right" });
  await playground.menuItem("Delete");
  await expect(page.locator("#dlg")).toBeVisible();
  expect((await playground.dialog()).title).toBe("Delete notes copy.md?");
  await page.locator("#dlgok").click();
  await expect(page.locator("#dlg")).toBeHidden();
  expect(await playground.exists("docs/notes copy.md")).toBe(false);

  await playground.row("docs/notes.md").click({ button: "right" });
  await playground.menuItem("Delete");
  await expect(page.locator("#dlg")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.locator("#dlg")).toBeHidden();
  expect(await playground.exists("docs/notes.md")).toBe(true);

  await playground.row("docs/lamp.md").click({ button: "right" });
  await playground.menuItem("Copy Path");
  expect(await playground.js(() => window.__pg.copied)).toBe("/work/dg/docs/lamp.md");
  await playground.row("docs/lamp.md").click({ button: "right" });
  await playground.menuItem("Copy Relative Path");
  expect(await playground.js(() => window.__pg.copied)).toBe("dg/docs/lamp.md");
});

test("the toolbar makes a folder, and files dropped from the OS land in it", async ({ playground, page }) => {
  await playground.row("docs").click();
  await page.locator("#exnewdir").click();
  await page.locator(".ex-input").fill("extra");
  await page.locator(".ex-input").press("Enter");
  await playground.until(() => window.__pg.isDir("docs/extra"));
  const accepted = await playground.js(selector => {
    const target = document.querySelector(selector);
    const data = new DataTransfer();
    data.items.add(new File(["alpha\n"], "a.txt", { type: "text/plain" }));
    data.items.add(new File(['{"b": 1}\n'], "b.json", { type: "text/plain" }));
    const box = target.getBoundingClientRect();
    const init = { bubbles: true, cancelable: true, dataTransfer: data, clientX: box.left + 30, clientY: box.top + 5 };
    target.dispatchEvent(new DragEvent("dragenter", init));
    const accepted = !target.dispatchEvent(new DragEvent("dragover", init));
    target.dispatchEvent(new DragEvent("drop", init));
    return accepted;
  }, rowSelector("docs/extra"));
  expect(accepted).toBe(true);
  await playground.until(() => window.__pg.exists("docs/extra/b.json"));
  expect(await playground.text("docs/extra/a.txt")).toBe("alpha\n");
  expect(await iconOfRow(playground, "docs/extra/b.json")).toBe("#i-json");
});

test("Ctrl and Shift select rows, and Delete removes them with a note on lost edits", async ({ playground, page }) => {
  await playground.row("machine/example_test.go").click();
  await playground.row("machine/machine_test.go").click({ modifiers: ["ControlOrMeta"] });
  await playground.row("machine/port.go").click({ modifiers: ["ControlOrMeta"] });
  expect((await playground.state()).selected).toHaveLength(3);
  await playground.row("machine/example_test.go").click();
  await playground.row("machine/port.go").click({ modifiers: ["Shift"] });
  expect((await playground.state()).selected).toHaveLength(4);

  await playground.replace("docs/lamp.md", "# The lamp", "# The lamp, edited");
  await playground.row("docs/lamp.md").click();
  await playground.row("machine/port.go").click({ modifiers: ["ControlOrMeta"] });
  await playground.row("go.mod").click({ modifiers: ["ControlOrMeta"] });
  await page.keyboard.press("Delete");
  await expect(page.locator("#dlg")).toBeVisible();
  const dialog = await playground.dialog();
  expect(dialog.title).toBe("Delete 3 items?");
  expect(dialog.list).toHaveLength(3);
  expect(dialog.text).toContain("dg/docs/lamp.md");
  await page.locator("#dlgok").click();
  await playground.until(() => !window.__pg.exists("docs/lamp.md"));
  expect(await playground.exists("machine/port.go")).toBe(false);
  expect((await playground.tabs()).map(tab => tab.path)).not.toContain("dg/docs/lamp.md");
  const note = await playground.lastNote();
  expect(note).toContain("lamp.md");
  expect(note).toContain("edits");
});

test("a folder downloads as a zip and a file as itself", async ({ playground }, testInfo) => {
  const archive = await playground.js(async () => {
    const { name, blob } = window.__pg.explorer.archive(["/work/dg/docs"]);
    return { name, bytes: [...new Uint8Array(await blob.arrayBuffer())] };
  });
  expect(archive.name).toBe("docs.zip");
  const file = testInfo.outputPath("docs.zip");
  fs.writeFileSync(file, Buffer.from(archive.bytes));
  const check = "import sys, zipfile; z = zipfile.ZipFile(sys.argv[1]); assert z.testzip() is None; print('\\n'.join(z.namelist()))";
  const names = execFileSync("python3", ["-I", "-c", check, file]).toString();
  expect(names).toContain("docs/lamp.md");
  expect(names).toContain("docs/states/machine-lamp.svg");
  expect(names).toContain("docs/states/");
  await playground.row("docs/lamp.md").click({ button: "right" });
  await playground.menuItem("Download");
  const download = await playground.js(() => window.__pg.download && { name: window.__pg.download.name, size: window.__pg.download.size });
  expect(download.name).toBe("lamp.md");
  expect(download.size).toBeGreaterThan(1000);
});

test("the workspace, the open tabs and the open folders are kept over a reload", async ({ playground, page }) => {
  await playground.open("machine/port.go");
  await playground.replace("machine/port.go", "package machine", "package machine // kept");
  await playground.row("docs").click();
  await page.locator("#exnewdir").click();
  await page.locator(".ex-input").fill("extra");
  await page.locator(".ex-input").press("Enter");
  await page.locator('#tree .ex-row[data-path="/work/loop"]').click();
  await playground.js(() => window.__pg.saveNow());
  const marks = await playground.marks();
  expect(Object.keys(marks).length).toBeGreaterThan(0);
  await playground.load();
  expect(await playground.js(() => window.__pg.restored)).toBe(true);
  expect(await playground.marks()).toEqual(marks);
  expect(await playground.text("machine/port.go")).toContain("// kept");
  expect(await playground.isDir("docs/extra")).toBe(true);
  expect((await playground.tabs()).map(tab => tab.path)).toContain("dg/machine/port.go");
  expect((await playground.state()).expanded).not.toContain("/work/loop");
});

test("Collapse All closes each folder", async ({ playground, page }) => {
  await page.locator("#excollapse").click();
  const state = await playground.state();
  expect(state.expanded).toEqual([]);
  expect(state.rows).toHaveLength(2);
});

test("Reset Sample brings back the starter files, also after a reload", async ({ playground, page }) => {
  await playground.replace("docs/lamp.md", "# The lamp", "# The lamp, edited");
  await playground.row("docs/lamp.md").click({ button: "right" });
  await playground.menuItem("Duplicate");
  await page.locator(".ex-head > span").click({ button: "right" });
  expect(await playground.menuLabels()).toContain("Reset Sample...");
  await playground.menuItem("Reset Sample...");
  await expect(page.locator("#dlg")).toBeVisible();
  await page.locator("#dlgok").click();
  await playground.until(() => !window.__pg.exists("docs/lamp copy.md"));
  expect(await playground.marks()).toEqual({});
  await playground.load();
  expect(await playground.exists("docs/lamp copy.md")).toBe(false);
  expect(await playground.marks()).toEqual({});
});

test("docgen update writes a page that the explorer renamed", async ({ playground, page }) => {
  test.setTimeout(3 * RUN_MS);
  await playground.row("docs/lamp.md").click();
  await page.keyboard.press("F2");
  await page.locator(".ex-input").fill("lamp-guide.md");
  await page.locator(".ex-input").press("Enter");
  await playground.until(() => window.__pg.exists("docs/lamp-guide.md"));
  await playground.replace("machine/machine.go", "const DefaultWait = 250 * time.Millisecond", "const DefaultWait = 700 * time.Millisecond");
  await playground.open("docs/lamp-guide.md");
  const update = await playground.runButton("update");
  expect(update.code, update.status).toBe(0);
  expect(update.changed).toContain("dg/docs/lamp-guide.md");
  expect(await playground.text("docs/lamp-guide.md")).toContain("`700ms`");
  expect(await playground.exists("docs/lamp.md")).toBe(false);
  expect(await playground.editorText()).toContain("`700ms`");
  expect((await playground.current()).path).toBe("dg/docs/lamp-guide.md");
  const check = await playground.runButton("check");
  expect(check.code, check.status).toBe(0);
});
