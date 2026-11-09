// The page at a width of 390 px: the explorer is a drawer with a dropdown, and the menu opens with a long press.
import { test, expect, rowSelector } from "../fixtures.mjs";

async function noOverflow(playground) {
  const size = await playground.js(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    innerWidth,
    scrollHeight: document.documentElement.scrollHeight,
    innerHeight,
  }));
  expect(size.scrollWidth).toBeLessThanOrEqual(size.innerWidth);
  expect(size.scrollHeight).toBeLessThanOrEqual(size.innerHeight);
}

/** longPress presses a touch on selector until the context menu opens, then lifts it, and returns whether the menu stays open. */
async function longPress(playground, selector) {
  await playground.js(selector => {
    const target = document.querySelector(selector);
    const box = target.getBoundingClientRect();
    window.__touch = { bubbles: true, cancelable: true, pointerType: "touch", pointerId: 7, isPrimary: true, clientX: box.left + 40, clientY: box.top + box.height / 2 };
    target.dispatchEvent(new PointerEvent("pointerdown", window.__touch));
  }, selector);
  await playground.until(() => !document.getElementById("menu").hidden);
  return playground.js(selector => {
    const target = document.querySelector(selector);
    target.dispatchEvent(new PointerEvent("pointerup", window.__touch));
    target.dispatchEvent(new MouseEvent("click", window.__touch));
    return !document.getElementById("menu").hidden;
  }, selector);
}

test("the page fits 390 px, with the explorer as a dropdown", async ({ playground, page }) => {
  expect(await playground.js(() => innerWidth)).toBe(390);
  await playground.setTheme("dark");
  await noOverflow(playground);
  await expect(page.locator(".vs-side")).toBeHidden();
  await expect(page.locator("#pick")).toBeVisible();
  const values = await page.locator("#pick option").evaluateAll(options => options.map(option => option.value));
  expect(values).toContain("preview:/work/dg/docs/states/machine-lamp.svg");
});

test("docgen update works at 390 px, and the page still fits", async ({ playground }) => {
  await playground.replace("machine/machine.go", "const DefaultWait = 250 * time.Millisecond", "const DefaultWait = 600 * time.Millisecond");
  const update = await playground.runButton("update");
  expect(update.code, update.status).toBe(0);
  expect(await playground.text("docs/lamp.md")).toContain("`600ms`");
  await playground.open("docs/lamp.md", "preview");
  await noOverflow(playground);
});

for (const theme of ["light", "dark"]) {
  test(`the explorer drawer and its long-press menu, ${theme}`, async ({ playground, page }) => {
    await playground.setTheme(theme);
    await page.locator("#files").click();
    await expect(page.locator("#side")).toHaveCSS("display", "flex");
    expect((await page.locator("#side").boundingBox()).width).toBeGreaterThan(200);
    expect(await longPress(playground, rowSelector("docs/lamp.md"))).toBe(true);
    const labels = await playground.menuLabels();
    expect(labels).toContain("Rename...");
    expect(labels).toContain("Delete");
    expect(labels).toContain("Open Preview");
    expect(labels).not.toContain("Open Preview to the Side");
    await page.keyboard.press("Escape");
    await page.locator("#scrim").click({ position: { x: 380, y: 400 } });
    await expect(page.locator("#side")).toBeHidden();
  });
}

test("the actions button opens the menu of the current file, and Rename opens the drawer", async ({ playground, page }) => {
  await page.locator("#more").click();
  expect(await playground.menuLabels()).toContain("Rename...");
  await playground.menuItem("Rename...");
  await expect(page.locator("#vs")).toHaveClass(/ex-open/);
  await expect(page.locator(".ex-input")).toBeVisible();
});

test("the auto theme follows a dark system setting", async ({ playground, page }) => {
  await playground.setTheme("auto");
  await page.emulateMedia({ colorScheme: "dark" });
  expect(await playground.js(() => matchMedia("(prefers-color-scheme: dark)").matches)).toBe(true);
  await expect(page.locator("body")).toHaveCSS("background-color", "rgb(30, 30, 30)");
});
