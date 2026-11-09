// The Markdown and SVG previews.
import { test, expect } from "../fixtures.mjs";

const UNSAFE =
  '# Unsafe\n\n<img src=x onerror="window.__xss = 1"> <a href="javascript:window.__xss = 2" id="out">link</a>\n' +
  "<svg><script>window.__xss = 3</script></svg><script>window.__xss = 4</script>\n" +
  '<p onclick="window.__xss = 5">text</p> <iframe srcdoc="<script>parent.__xss = 6</script>"></iframe>\n\n' +
  "![Transitions](states/machine-lamp.svg)\n";

test("HTML in a Markdown page runs no script in the preview, and its images still show", async ({ playground }) => {
  await playground.js(text => {
    window.__pg.session.fsys.writeFile("/work/dg/docs/xss.md", new TextEncoder().encode(text));
  }, UNSAFE);
  await playground.open("docs/xss.md", "preview");
  await playground.until(() => {
    const images = window.__pg.previewImages();
    return images.length && images.every(image => image.complete);
  });
  const result = await playground.js(() => {
    const box = document.getElementById("preview");
    const all = [...box.querySelectorAll("*")];
    const urls = [...box.querySelectorAll("[href], [src]")].map(node => node.getAttribute("href") || node.getAttribute("src") || "");
    return {
      ran: window.__xss || null,
      eventAttributes: all.filter(node => [...node.attributes].some(attribute => attribute.name.startsWith("on"))).length,
      scriptUrls: urls.filter(url => /^javascript:/i.test(url)).length,
      forbidden: box.querySelectorAll("script, svg, iframe, [id]").length,
      text: box.textContent.includes("link") && box.textContent.includes("text"),
      images: window.__pg.previewImages(),
    };
  });
  expect(result.ran).toBeNull();
  expect(result.eventAttributes).toBe(0);
  expect(result.scriptUrls).toBe(0);
  expect(result.forbidden).toBe(0);
  expect(result.text).toBe(true);
  expect(result.images.some(image => image.path === "dg/docs/states/machine-lamp.svg" && image.w > 0)).toBe(true);
});

test("the explorer opens the SVG source, with Open Preview and a side preview that follows edits", async ({ playground, page }) => {
  await playground.row("docs/states/machine-lamp.svg").click();
  const source = await playground.current();
  expect(source).toMatchObject({ kind: "file", code: true, lang: "SVG" });
  expect(await playground.editorText()).toMatch(/^\s*<(\?xml|svg)/);
  expect(await page.locator(".cm-content span[class]").count()).toBeGreaterThan(5);
  await expect(page.locator("#openpv")).toBeVisible();
  await expect(page.locator("#showsrc")).toBeHidden();

  await page.locator("#openpv").click();
  await playground.until(() => window.__pg.previewImages().length && window.__pg.previewImages()[0].complete);
  const [image] = await playground.previewImages();
  expect((await playground.current()).kind).toBe("preview");
  expect(image.w).toBeGreaterThan(0);
  expect(image.src).toMatch(/^blob:/);
  await expect(page.locator("#showsrc")).toBeVisible();
  await expect(page.locator("#openpv")).toBeHidden();

  await page.locator("#showsrc").click();
  expect((await playground.current()).kind).toBe("file");
  await page.locator("#openside").click();
  await playground.until(() => window.__pg.sideImages().length && window.__pg.sideImages()[0].w > 0);
  const [before] = await playground.sideImages();
  expect(await playground.editorText()).toMatch(/<\/svg>\s*$/);
  await playground.replace("docs/states/machine-lamp.svg", "</svg>", '<rect x="0" y="0" width="40" height="40" fill="#e11"/></svg>');
  await playground.until(src => {
    const [image] = window.__pg.sideImages();
    return image && image.src !== src && image.w > 0;
  }, before.src);
});

test("the explorer opens the Markdown source, with a preview tab and a side preview that follow edits", async ({ playground, page }) => {
  await playground.row("docs/lamp.md").click();
  expect(await playground.current()).toMatchObject({ kind: "file", code: true, lang: "Markdown" });
  await page.locator("#openpv").click();
  expect((await playground.current()).kind).toBe("preview");
  expect(await playground.js(() => window.__pg.previewText())).toContain("The lamp");
  await page.locator("#showsrc").click();
  await page.locator("#openside").click();
  await playground.until(() => window.__pg.sideText() && window.__pg.sideText().includes("The lamp"));
  await playground.replace("docs/lamp.md", "# The lamp", "# The lamp, edited live");
  await playground.until(() => window.__pg.sideText().includes("The lamp, edited live"));
  await page.locator("#openpv").click();
  expect(await playground.js(() => window.__pg.previewText())).toContain("The lamp, edited live");
  await page.locator("#sideclose").click();
  await playground.row("docs/lamp.md").click({ button: "right" });
  const labels = await playground.menuLabels();
  expect(labels).toContain("Open Preview");
  expect(labels).toContain("Open Preview to the Side");
});
