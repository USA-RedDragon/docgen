// The Playwright Test configuration of the browser test of the playground.
// Environment:
//   PLAYGROUND_SITE      the built docs site to serve, with the playground in it (required)
//   PLAYGROUND_PAGE      the path of the playground page (default /playground/)
//   PLAYGROUND_DOCS      a docs page and the Playground link that it must have, such as
//                        "/index.html playground/"; empty skips the docs page tests
//   PLAYGROUND_TEST_OUT  the directory of the HTML report and the test results
//                        (default artifacts/playground-test of the repository)
//   PORT                 the port of the server (default 18350)
import path from "node:path";
import { defineConfig } from "@playwright/test";

const here = path.dirname(new URL(import.meta.url).pathname);
const site = process.env.PLAYGROUND_SITE;
if (!site) throw new Error("PLAYGROUND_SITE is not set: give the directory of a built docs site");
const port = Number(process.env.PORT || 18350);
const page = process.env.PLAYGROUND_PAGE || "/playground/";
const out = path.resolve(process.env.PLAYGROUND_TEST_OUT || path.join(here, "../../artifacts/playground-test"));
const desktop = { width: 1280, height: 800 };
const phone = { width: 390, height: 844 };

export default defineConfig({
  testDir: "specs",
  outputDir: path.join(out, "results"),
  reporter: [["list"], ["html", { outputFolder: path.join(out, "report"), open: "never" }]],
  // A test loads the whole toolchain into a fresh browser context, and a docgen update or a go build
  // in the browser takes up to a few minutes on a slow runner.
  timeout: 10 * 60 * 1000,
  expect: { timeout: 15 * 1000 },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  workers: Number(process.env.PLAYGROUND_TEST_WORKERS || 4),
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    colorScheme: "light",
  },
  webServer: {
    command: `node ${JSON.stringify(path.join(here, "serve.mjs"))} ${JSON.stringify(path.resolve(site))} ${port}`,
    url: `http://127.0.0.1:${port}${page}`,
    reuseExistingServer: false,
    stdout: "ignore",
  },
  projects: [
    { name: "firefox", use: { browserName: "firefox", viewport: desktop }, testIgnore: /phone\.spec/ },
    { name: "chromium", use: { browserName: "chromium", viewport: desktop }, testIgnore: /phone\.spec/ },
    { name: "firefox-390", use: { browserName: "firefox", viewport: phone }, testMatch: /phone\.spec/ },
    { name: "chromium-390", use: { browserName: "chromium", viewport: phone }, testMatch: /phone\.spec/ },
  ],
});
