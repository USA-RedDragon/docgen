// Shell commands of the bb shell, the go test timeout, Stop and the process limit.
import { test, expect, RUN_MS } from "../fixtures.mjs";

test("the shell runs commands and pipelines", async ({ playground }) => {
  const head = await playground.runCommand("head -n 1 go.mod");
  const pipeline = await playground.runCommand("cat go.mod | grep module | cut -d' ' -f2");
  const output = await playground.output(400);
  expect(head.code).toBe(0);
  expect(output).toContain("$ head -n 1 go.mod\nmodule github.com/USA-RedDragon/docgen/v2\n");
  expect(pipeline.code).toBe(0);
  expect(output).toContain("cut -d' ' -f2\ngithub.com/USA-RedDragon/docgen/v2\n");
  const misc = await playground.runCommand(
    "echo a b | tr a-z A-Z; printf '%s=%d\\n' n 3; ls docs; wc -l < go.mod; sed -n 's/^go //p' go.mod; " +
      "date +%Y | cut -c1-2; basename $PWD; test -d docs && echo dir",
  );
  expect(misc.code, await playground.output(300)).toBe(0);
});

test("go test -timeout ends a test that does not stop", async ({ playground }) => {
  const run = await playground.runCommand("cd ../loop && go test -timeout 2s .");
  const output = await playground.output(600);
  expect(run.code).toBe(1);
  expect(output).toContain("panic: test timed out after 2s");
  expect(output).toMatch(/FAIL\texample\.com\/loop\t2\.\d+s/);
  // The deadline is 2 s; the rest is the build of the test, which is slower when other tests share the CPU.
  expect(run.ms).toBeLessThan(15000);
  await playground.runCommand("cd ../loop && go test -v -timeout 2s .");
  expect(await playground.output(600)).toContain("running tests:\n\t\tTestForever (2s)");
});

test("Stop kills the process tree of a build", async ({ playground, page }) => {
  test.setTimeout(2 * RUN_MS);
  const build = await playground.runCommand("go build -a -o /tmp/lamp ./cmd/lamp", async () => {
    // Stop comes while the compiler runs, so it kills a tree of processes.
    await playground.until(() => document.getElementById("step").textContent.includes("compile"), null, RUN_MS);
    await page.locator("#stop").click();
  });
  expect(build.code).toBeGreaterThanOrEqual(256);
  expect(build.status).toContain("SIGKILL");
  expect(await playground.js(() => window.__pg.session.kernel.stats.live)).toBe(0);
  const check = await playground.runButton("check");
  expect(check.code, check.status).toBe(0);
});

test("the process limit of ?limit=3 kills a long process", async ({ playground }) => {
  await playground.load("?limit=3");
  const build = await playground.runCommand("go build -a -o /tmp/lamp ./cmd/lamp");
  expect(build.code).toBeGreaterThanOrEqual(256);
  expect(await playground.output(600)).toContain("process limit of 3s");
});
