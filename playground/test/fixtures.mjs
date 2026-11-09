// The fixtures of the browser test. The playground fixture opens the page in the fresh context of the
// test, waits until the page is ready, and at the end of the test expects no page errors. The page
// exposes its state and test hooks as window.__pg.
import { test as base, expect } from "@playwright/test";

export { expect };

/** The path of the playground page. */
export const PAGE = process.env.PLAYGROUND_PAGE || "/playground/";
/** The time limit of a load of the page, and of a run such as docgen update or go build. */
export const LOAD_MS = 5 * 60 * 1000;
export const RUN_MS = 8 * 60 * 1000;

/** rowSelector returns the CSS selector of the explorer row of path, relative to /work/dg. */
export const rowSelector = path => `.ex-row[data-path="/work/dg/${path}"]`;

class Playground {
  constructor(page) {
    this.page = page;
  }

  /** load opens the page with the query and waits until it is ready. It returns window.__pg.load. */
  async load(query = "") {
    await this.page.goto(PAGE + query);
    await this.page.waitForFunction(() => window.__pg && (window.__pg.ready || window.__pg.errors.length), null, {
      timeout: LOAD_MS,
    });
    expect(await this.errors()).toEqual([]);
    return this.js(() => window.__pg.load);
  }

  /** js runs fn in the page with arg and returns its result. */
  js(fn, arg) {
    return this.page.evaluate(fn, arg);
  }

  errors() {
    return this.js(() => window.__pg.errors);
  }

  /** until waits until fn(arg) returns a truthy value in the page, and returns it. */
  async until(fn, arg, timeout = 15 * 1000) {
    const handle = await this.page.waitForFunction(fn, arg, { timeout, polling: 100 });
    return handle.jsonValue();
  }

  async lastStepAfter(count) {
    await this.until(n => window.__pg.steps.length > n, count, RUN_MS);
    return this.js(() => window.__pg.steps.at(-1));
  }

  /** runButton clicks the run button id and resolves with the step of the run. */
  async runButton(id) {
    const count = await this.js(() => window.__pg.steps.length);
    await this.page.locator("#" + id).click();
    return this.lastStepAfter(count);
  }

  /** runCommand runs the shell command and resolves with its step. during runs while the command runs. */
  async runCommand(command, during) {
    const count = await this.js(() => window.__pg.steps.length);
    await this.page.locator("#cmd").fill(command);
    await this.js(() => document.getElementById("cmdform").requestSubmit());
    if (during) await during();
    return this.lastStepAfter(count);
  }

  /** output returns the end of the text of the output pane. */
  async output(length = 1500) {
    return (await this.js(() => window.__pg.term())).slice(-length);
  }

  replace(path, from, to) {
    return this.js(([path, from, to]) => window.__pg.replace(path, from, to), [path, from, to]);
  }

  open(path, kind = "file") {
    return this.js(([path, kind]) => void window.__pg.open(path, kind), [path, kind]);
  }

  text(path) {
    return this.js(path => window.__pg.text(path), path);
  }

  exists(path) {
    return this.js(path => !!window.__pg.exists(path), path);
  }

  isDir(path) {
    return this.js(path => !!window.__pg.isDir(path), path);
  }

  tabs() {
    return this.js(() => window.__pg.tabs());
  }

  current() {
    return this.js(() => window.__pg.current());
  }

  marks() {
    return this.js(() => window.__pg.marks());
  }

  lastNote() {
    return this.js(() => window.__pg.notes.at(-1));
  }

  editorText() {
    return this.js(() => window.__pg.editorText());
  }

  setTheme(theme) {
    return this.js(theme => window.__pg.setTheme(theme), theme);
  }

  previewImages() {
    return this.js(() => window.__pg.previewImages());
  }

  sideImages() {
    return this.js(() => window.__pg.sideImages());
  }

  /** state returns the state of the explorer: {expanded, selected, focus, edit, rows, drop}. */
  state() {
    return this.js(() => window.__pg.explorer.state());
  }

  row(path) {
    return this.page.locator(rowSelector(path));
  }

  /** menuLabels returns the labels of the open context menu, with " (off)" on a disabled item, or null. */
  menuLabels() {
    return this.js(() => {
      const menu = document.getElementById("menu");
      if (menu.hidden) return null;
      return [...menu.querySelectorAll("button")].map(button => button.firstChild.textContent + (button.disabled ? " (off)" : ""));
    });
  }

  /** menuItem clicks the enabled item label of the open context menu. */
  async menuItem(label) {
    const item = this.page.locator("#menu button:not(:disabled)").filter({ has: this.page.getByText(label, { exact: true }) });
    await item.click();
  }

  /** dialog returns the title, text and list of the open confirm dialog, or null. */
  dialog() {
    return this.js(() => {
      if (!document.getElementById("dlg").open) return null;
      return {
        title: document.getElementById("dlgt").textContent,
        text: document.getElementById("dlgm").textContent,
        list: [...document.querySelectorAll("#dlgl li")].map(item => item.textContent),
      };
    });
  }

  /** nameError returns the error under the name input after the name is typed into it, or "". */
  async nameError(name) {
    await this.page.locator(".ex-input").fill(name);
    return this.js(() => {
      const message = document.querySelector(".ex-msg");
      return message.hidden ? "" : message.textContent;
    });
  }

  /** clickTab clicks the tab of the file whose path ends with suffix. */
  async clickTab(suffix, kind = "file") {
    await this.page.locator(`.vs-tab[data-kind="${kind}"][data-path$="${suffix}"] > button:first-of-type`).click();
  }
}

export const test = base.extend({
  playground: async ({ page }, use) => {
    const playground = new Playground(page);
    await playground.load();
    await use(playground);
    expect(await playground.errors()).toEqual([]);
  },
});
