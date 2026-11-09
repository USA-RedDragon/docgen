// The HTTP headers of the playground and of a docs page, as Cloudflare Pages sends them with _headers.
import { test, expect } from "@playwright/test";
import { PAGE } from "../fixtures.mjs";

const DOCS = process.env.PLAYGROUND_DOCS || "";
const [docsPage, playgroundLink] = DOCS.split(" ");

test("the playground page has the cross-origin isolation headers", async ({ request }) => {
  const headers = (await request.head(PAGE)).headers();
  expect(headers["cross-origin-opener-policy"]).toBe("same-origin");
  expect(headers["cross-origin-embedder-policy"]).toBe("require-corp");
  expect(headers["cross-origin-resource-policy"]).toBe("same-origin");
});

test("each asset is cached for a year, with its Content-Encoding and the type application/wasm", async ({ request }) => {
  const manifest = await (await request.get(PAGE + "manifest.json")).json();
  expect(manifest.files.length).toBeGreaterThan(0);
  for (const file of manifest.files) {
    const headers = (await request.head(PAGE + file.url)).headers();
    expect(headers["cache-control"], file.url).toBe("public, max-age=31536000, immutable");
    expect(headers["content-encoding"], file.url).toBe(file.encoding === "identity" ? undefined : file.encoding);
    expect(headers["content-type"], file.url).toBe("application/wasm");
  }
});

test.describe("the docs page", () => {
  test.skip(!DOCS, "PLAYGROUND_DOCS is not set");

  test("has no cross-origin isolation header", async ({ request }) => {
    const headers = (await request.head(docsPage)).headers();
    expect(Object.keys(headers).filter(name => name.startsWith("cross-origin-"))).toEqual([]);
  });

  test("is not isolated, loads its Google Fonts and links the playground", async ({ page }) => {
    await page.goto(docsPage);
    const result = await page.evaluate(async () => {
      await document.fonts.ready;
      return {
        isolated: crossOriginIsolated,
        font: document.fonts.check('16px "Nunito Sans"'),
        loaded: [...document.fonts].filter(font => font.status === "loaded").length,
        links: [...document.querySelectorAll("header nav a")]
          .filter(link => link.textContent === "Playground")
          .map(link => link.getAttribute("href")),
      };
    });
    expect(result.isolated).toBe(false);
    expect(result.font).toBe(true);
    expect(result.loaded).toBeGreaterThan(0);
    expect(result.links).toEqual([playgroundLink]);
  });
});
