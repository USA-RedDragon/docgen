// docgen update and docgen check on the sample, and the exec= markers.
import { test, expect, RUN_MS } from "../fixtures.mjs";

const WAIT_250 = "const DefaultWait = 250 * time.Millisecond";

test("docgen update writes an edit of the code into the page", async ({ playground, page }) => {
  await playground.open("docs/lamp.md");
  expect(await playground.replace("machine/machine.go", WAIT_250, "const DefaultWait = 400 * time.Millisecond")).toBe(true);
  const count = await playground.js(() => window.__pg.steps.length);
  await page.locator("#update").click();
  await playground.until(() => window.__pg.busy);
  await expect(page.locator("#update")).toBeDisabled();
  await expect(page.locator("#check")).toBeDisabled();
  await expect(page.locator("#stop")).toBeEnabled();
  const update = await playground.lastStepAfter(count);
  expect(update.code, update.status).toBe(0);
  expect(update.changed).toContain("dg/docs/lamp.md");
  expect(update.reloaded).toContain("dg/docs/lamp.md");
  const text = await playground.text("docs/lamp.md");
  expect(text).toContain("`400ms`");
  expect(text).toContain("(default 400ms)");
  expect(text).toContain("```text\nmodule github.com/USA-RedDragon/docgen/v2\n```");
  expect(text).toContain("```text\ngithub.com/USA-RedDragon/docgen/v2\n```");

  await playground.open("docs/lamp.md", "preview");
  await playground.until(() => {
    const images = window.__pg.previewImages();
    return images.length && images.every(image => image.complete);
  });
  expect(await playground.js(() => window.__pg.previewText())).toContain("400ms");
  const images = await playground.previewImages();
  expect(images).toHaveLength(1);
  expect(images[0].w).toBeGreaterThan(0);

  await playground.open("docs/lamp.md");
  expect(await playground.editorText()).toContain("`400ms`");
});

test("docgen check is clean, fails on a stale page and runs the exec= markers", async ({ playground }) => {
  test.setTimeout(3 * RUN_MS);
  const clean = await playground.runButton("check");
  expect(clean.code, clean.status).toBe(0);

  await playground.replace("docs/lamp.md", "`250ms`<!-- /docgen:const -->", "`1s`<!-- /docgen:const -->");
  const stale = await playground.runButton("check");
  expect(stale.code, stale.status).toBe(1);
  const output = await playground.output(2500);
  expect(output).toMatch(/stale: docs\/lamp\.md:3: docgen:const/);
  expect(output).toContain("docgen: running head -n 1 go.mod");
  expect(output).toContain("docgen: running cat go.mod | grep module | cut -d' ' -f2");

  await playground.replace("docs/lamp.md", "`1s`<!-- /docgen:const -->", "`250ms`<!-- /docgen:const -->");
  const restored = await playground.runButton("check");
  expect(restored.code, restored.status).toBe(0);
});
