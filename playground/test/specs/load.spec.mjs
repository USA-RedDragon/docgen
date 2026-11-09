// The first load of the page, a load from the Cache API, and the start state.
import { test, expect } from "../fixtures.mjs";

test("the first load fetches each file, and a reload takes them from the Cache API", async ({ playground }) => {
  const first = await playground.js(() => window.__pg.load);
  expect(first.hits).toBe(0);
  const second = await playground.load();
  expect(second.hits).toBe(second.files);
});

test("the page is cross-origin isolated and shows the docgen version and commit", async ({ playground }) => {
  expect(await playground.js(() => crossOriginIsolated)).toBe(true);
  const version = await playground.js(() => ({ pg: window.__pg.version, text: document.getElementById("ver").textContent }));
  expect(version.pg.commit).toBeTruthy();
  expect(version.text).toContain("docgen ");
  expect(version.text).toContain(version.pg.go);
});

test("the first load shows the source of lamp.md and no preview tab", async ({ playground }) => {
  const tabs = await playground.tabs();
  expect(tabs).toContainEqual({ path: "dg/docs/lamp.md", kind: "file", current: true });
  expect(tabs.filter(tab => tab.kind === "preview")).toEqual([]);
});

test("keys typed into the editor reach the memfs", async ({ playground, page }) => {
  await playground.open("machine/machine.go");
  await page.locator(".cm-content").click();
  await playground.js(() => window.__pg.cursorEnd());
  await page.keyboard.type("\n// typed in the tab");
  expect(await playground.text("machine/machine.go")).toContain("// typed in the tab");
});
