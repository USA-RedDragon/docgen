// The browser test of the playground page. Usage: node scenario.mjs MARIONETTE_PORT PAGE_URL OUT_DIR
// PAGE_URL is the page on a server of serve.mjs, such as http://127.0.0.1:18350/playground/.
// OUT_DIR gets scenario.json and the screenshots shot-*.png. The exit status is 1 when a check fails.
import fs from "node:fs";
import { connect } from "./ff.mjs";
import { explorerSteps, narrowSteps } from "./explorer-steps.mjs";
const [port, pageUrl, outDir] = process.argv.slice(2);
const frameUrl = new URL("/__test/frame.html?src=" + new URL(pageUrl).pathname, pageUrl).href;
fs.mkdirSync(outDir, { recursive: true });
const R = { steps: [], checks: [], loads: [], errors: [] };
let ff;
// An error outside the try below, such as in a callback, ends the run with exit 1.
for (const ev of ["uncaughtException", "unhandledRejection"]) process.on(ev, e => { console.log("FATAL", ev, e && e.stack || e); process.exit(1); });
const shot = async name => { await new Promise(r => setTimeout(r, 300)); await ff.shot(`${outDir}/shot-${name}.png`); R.checks.push({ shot: name }); };
const check = (name, ok, detail) => { R.checks.push({ name, ok: !!ok, detail }); console.log(`${ok ? "PASS" : "FAIL"} ${name}${detail !== undefined ? ": " + JSON.stringify(detail).slice(0, 300) : ""}`); };
const load = async (query = "", label = "") => {
  const t0 = Date.now();
  await ff.go(pageUrl + query);
  const r = await ff.until("window.__pg && (window.__pg.ready || window.__pg.errors.length) && window.__pg.load", 300000);
  r.wall = Date.now() - t0; r.label = label;
  R.loads.push(r);
  console.log(`load ${label}: ${r.wall} ms wall, ${r.total} ms page, cache hits ${r.hits}/${r.files}`);
  return r;
};
const click = id => ff.js(`document.getElementById(${JSON.stringify(id)}).click()`);
const idle = (ms = 600000) => ff.until("!window.__pg.busy", ms, 100);
const lastStep = () => ff.js("return window.__pg.steps.at(-1)");
const runButton = async (id, label) => {
  const n = await ff.js("return window.__pg.steps.length");
  await click(id);
  await ff.until(`window.__pg.steps.length > ${n}`, 600000, 100);
  const s = await lastStep();
  s.name = label; R.steps.push(s);
  console.log(`step ${label}: ${s.status} in ${s.ms} ms, ${s.spawns} processes, reloaded ${s.changed.join(", ") || "none"}`);
  return s;
};
const runCmd = async (cmd, label = cmd, during) => {
  await ff.js(`document.getElementById("cmd").value = arguments[0]`, [cmd]);
  const n = await ff.js("return window.__pg.steps.length");
  await ff.js(`document.getElementById("cmdform").requestSubmit()`);
  if (during) await during();
  await ff.until(`window.__pg.steps.length > ${n}`, 600000, 100);
  const s = await lastStep();
  s.name = label; R.steps.push(s);
  console.log(`step ${label}: ${s.status} in ${s.ms} ms`);
  return s;
};
const termTail = async (n = 1500) => (await ff.js("return window.__pg.term()")).slice(-n);
const noOverflow = () => ff.js("return {sw: document.documentElement.scrollWidth, iw: innerWidth, sh: document.documentElement.scrollHeight, ih: innerHeight}");

try {
  ff = await connect(port);
  await ff.size(1280, 800);
  // First load with an empty Cache API, then a reload from the cache.
  await load("?clearcache", "first (empty cache)");
  await load("", "second (from cache)");
  await ff.js("window.__pg.setTheme('light')");
  check("isolated", await ff.js("return crossOriginIsolated"));
  { const v = await ff.js("return {pg: window.__pg.version, text: document.getElementById('ver').textContent}"); check("page shows the docgen version and commit", v.pg && v.pg.commit && v.text.includes("docgen ") && v.text.includes(v.pg.go), v); }
  { const t = await ff.js("return window.__pg.tabs()"); check("first load shows lamp.md source, no preview tab", t.some(x => x.current && x.path === "dg/docs/lamp.md" && x.kind === "file") && !t.some(x => x.kind === "preview"), t); }
  await shot("1280-light-start");

  // Typing into the editor with WebDriver key events.
  await ff.js("window.__pg.open('machine/machine.go')");
  const el = await ff.cmd("WebDriver:FindElement", { using: "css selector", value: ".cm-content" });
  const ref = el.value || el;
  const before = await ff.js("return window.__pg.editorText().length");
  await ff.cmd("WebDriver:ElementClick", { id: Object.values(ref)[0] });
  await ff.js("window.__pg.cursorEnd()");
  await ff.cmd("WebDriver:ElementSendKeys", { id: Object.values(ref)[0], text: "\n// typed in the tab" });
  const typed = await ff.js("return window.__pg.text('machine/machine.go').includes('// typed in the tab')");
  check("keys typed into the editor reach the memfs", typed, { before, after: await ff.js("return window.__pg.editorText().length") });

  // Edit DefaultWait and update.
  await ff.js("window.__pg.open('docs/lamp.md')");
  await ff.js("window.__pg.open('machine/machine.go')");
  check("edit DefaultWait", await ff.js("return window.__pg.replace('machine/machine.go', 'const DefaultWait = 250 * time.Millisecond', 'const DefaultWait = 400 * time.Millisecond')"));
  await shot("1280-light-edit");
  const busyShot = (async () => { await ff.until("window.__pg.busy", 10000, 50); await new Promise(r => setTimeout(r, 2500)); await shot("1280-light-busy"); check("buttons disabled while busy", await ff.js("return document.getElementById('update').disabled && document.getElementById('check').disabled && !document.getElementById('stop').disabled")); })();
  const up = await runButton("update", "docgen update after the edit");
  await busyShot;
  check("update exit 0", up.code === 0, up.status);
  const page = await ff.js("return window.__pg.text('docs/lamp.md')");
  check("page has 400ms", page.includes("`400ms`") && page.includes("(default 400ms)"), page.match(/default \S+\)/g));
  await ff.js("window.__pg.open('docs/lamp.md', 'preview')");
  await ff.until("window.__pg.previewImages().length && window.__pg.previewImages().every(i => i.complete)", 10000);
  const imgs = await ff.js("return window.__pg.previewImages()");
  check("preview shows 400ms", (await ff.js("return window.__pg.previewText()")).includes("400ms"));
  check("preview SVG diagram rendered", imgs.length === 1 && imgs[0].w > 0, imgs);
  await shot("1280-light-preview");
  await ff.js("window.__pg.open('docs/lamp.md')");
  check("page tab shows the new text", (await ff.js("return window.__pg.editorText()")).includes("`400ms`"));
  await shot("1280-light-page-after-update");

  const c0 = await runButton("check", "docgen check (clean)");
  check("check exit 0", c0.code === 0, c0.status);

  // Make the page stale.
  await ff.js("window.__pg.replace('docs/lamp.md', '`400ms`<!-- /docgen:const -->', '`1s`<!-- /docgen:const -->')");
  const c1 = await runButton("check", "docgen check (stale page)");
  check("stale check exit 1", c1.code === 1, c1.status);
  const t1 = await termTail(2500);
  check("check lists the stale marker", /stale: docs\/lamp\.md:3: docgen:const/.test(t1), t1.slice(-600));
  await shot("1280-light-stale");

  // exec= markers ran through sh: their lines in the page and the docgen progress lines.
  check("exec markers ran", t1.includes("docgen: running head -n 1 go.mod") && t1.includes("docgen: running cat go.mod | grep module | cut -d' ' -f2"));
  check("exec output in page", page.includes("```text\nmodule github.com/USA-RedDragon/docgen/v2\n```") && page.includes("```text\ngithub.com/USA-RedDragon/docgen/v2\n```"));
  const h = await runCmd("head -n 1 go.mod");
  const p1 = await runCmd("cat go.mod | grep module | cut -d' ' -f2");
  const tt = await termTail(400);
  check("sh head", h.code === 0 && tt.includes("$ head -n 1 go.mod\nmodule github.com/USA-RedDragon/docgen/v2\n"));
  check("sh pipeline", p1.code === 0 && tt.includes("cut -d' ' -f2\ngithub.com/USA-RedDragon/docgen/v2\n"));
  const misc = await runCmd("echo a b | tr a-z A-Z; printf '%s=%d\\n' n 3; ls docs; wc -l < go.mod; sed -n 's/^go //p' go.mod; date +%Y | cut -c1-2; basename $PWD; test -d docs && echo dir");
  check("sh misc commands", misc.code === 0, (await termTail(300)));

  // Restore the page, then check is clean again.
  await ff.js("window.__pg.replace('docs/lamp.md', '`1s`<!-- /docgen:const -->', '`400ms`<!-- /docgen:const -->')");
  const c2 = await runButton("check", "docgen check (restored)");
  check("restored check exit 0", c2.code === 0, c2.status);

  // Test timeout.
  const to = await runCmd("cd ../loop && go test -timeout 2s .", "go test -timeout 2s (infinite loop)");
  const tto = await termTail(600);
  check("timeout test fails", to.code === 1 && tto.includes("panic: test timed out after 2s") && /FAIL\texample\.com\/loop\t2\.\d+s/.test(tto), tto);
  check("timeout test ends in about 2 s", to.ms < 6000, to.ms);
  const tov = await runCmd("cd ../loop && go test -v -timeout 2s .", "go test -v -timeout 2s (infinite loop)");
  check("timeout -v lists the test", (await termTail(600)).includes("running tests:\n\t\tTestForever (2s)"), tov.ms);
  await shot("1280-light-timeout");

  // Stop during a long build.
  let clickAt = 0;
  const st = await runCmd("go build -a -o /tmp/lamp ./cmd/lamp", "go build -a (Stop after 4 s)", async () => {
    await ff.until("window.__pg.busy", 10000, 50);
    await new Promise(r => setTimeout(r, 4000));
    await shot("1280-light-long-build");
    clickAt = Date.now();
    await click("stop");
  });
  const stopMs = Date.now() - clickAt;
  check("Stop kills the tree", st.code >= 256 && st.status.includes("SIGKILL"), { status: st.status, ms: st.ms, stopToDoneMs: stopMs });
  check("no live processes after Stop", (await ff.js("return window.__pg.session.kernel.stats.live")) === 0);
  await shot("1280-light-stopped");
  const after = await runButton("check", "docgen check (after Stop)");
  check("check after Stop exit 0", after.code === 0, after.status);

  // Dark theme at 1280.
  await ff.js("window.__pg.setTheme('dark')");
  await ff.js("window.__pg.open('machine/machine.go')");
  await shot("1280-dark-editor");
  await ff.js("window.__pg.open('docs/lamp.md', 'preview')");
  await shot("1280-dark-preview");

  // The explorer, and the Markdown and SVG sources and previews. The steps end with Reset Sample.
  await explorerSteps({ ff, check, shot, load, runButton, R, outDir });

  R.errors = await ff.js("return window.__pg.errors");
  R.kernelStats = await ff.js("const s = window.__pg.session.kernel.stats; return {spawns: s.spawns, maxLive: s.maxLive, maxMem: s.maxMem}");
  check("update reloaded the page tab", up.changed.includes("dg/docs/lamp.md") && up.reloaded.includes("dg/docs/lamp.md"), { changed: up.changed, reloaded: up.reloaded });

  // HTML in a Markdown page runs no script in the preview, and its images still show.
  await ff.js(`const s = window.__pg.session.fsys;
    s.writeFile("/work/dg/docs/xss.md", new TextEncoder().encode('# Unsafe\\n\\n<img src=x onerror="window.__xss = 1"> <a href="javascript:window.__xss = 2" id="out">link</a>\\n<svg><script>window.__xss = 3</script></svg><script>window.__xss = 4</script>\\n<p onclick="window.__xss = 5">text</p> <iframe srcdoc="<script>parent.__xss = 6</script>"></iframe>\\n\\n![Transitions](states/machine-lamp.svg)\\n'));`);
  await ff.js("window.__pg.open('docs/xss.md', 'preview')");
  await ff.until("window.__pg.previewImages().length && window.__pg.previewImages().every(i => i.complete)", 10000);
  await new Promise(r => setTimeout(r, 500));
  const xss = await ff.js(`const b = document.getElementById("preview");
    return { ran: window.__xss || null, onAttrs: [...b.querySelectorAll("*")].filter(e => [...e.attributes].some(a => a.name.startsWith("on"))).length,
      js: [...b.querySelectorAll("[href], [src]")].filter(e => /^javascript:/i.test(e.getAttribute("href") || e.getAttribute("src") || "")).length,
      bad: b.querySelectorAll("script, svg, iframe, [id]").length, imgs: window.__pg.previewImages(), text: b.textContent.includes("link") && b.textContent.includes("text") };`);
  check("md preview: page HTML runs no script, and the diagram still shows", !xss.ran && !xss.onAttrs && !xss.js && !xss.bad && xss.text && xss.imgs.some(i => i.path === "dg/docs/states/machine-lamp.svg" && i.w > 0), xss);
  await ff.js(`const [d, n] = window.__pg.session.fsys.split("/work/dg/docs/xss.md"); d.children.delete(n);`);
  await ff.js("window.__pg.open('docs/lamp.md')");
  await ff.js("document.getElementById('exrefresh').click()");

  // 390 px: headless Firefox has a minimum window width of 500 px, so the page runs in a 390 x 844 iframe.
  await ff.size(1280, 1000);
  await ff.go(frameUrl);
  await ff.frame("#f");
  await ff.until("window.__pg && (window.__pg.ready || window.__pg.errors.length)", 300000);
  const shot390 = async name => { await ff.frame(); await new Promise(r => setTimeout(r, 300)); await ff.shotEl("#f", `${outDir}/shot-${name}.png`); R.checks.push({ shot: name }); await ff.frame("#f"); };
  check("390: viewport", (await ff.js("return innerWidth")) === 390);
  await ff.js("window.__pg.setTheme('dark')");
  const o1 = await noOverflow();
  check("390: no horizontal overflow", o1.sw <= o1.iw && o1.sh <= o1.ih, o1);
  check("390: explorer is a dropdown", await ff.js("return getComputedStyle(document.querySelector('.vs-side')).display === 'none' && getComputedStyle(document.getElementById('pick')).display !== 'none'"));
  await shot390("390-dark-preview");
  await ff.js("window.__pg.replace('machine/machine.go', 'const DefaultWait = 250 * time.Millisecond', 'const DefaultWait = 600 * time.Millisecond')");
  await shot390("390-dark-editor");
  await ff.js("window.__pg.setTheme('light')");
  await shot390("390-light-editor");
  const u390 = await runButton("update", "docgen update at 390 px");
  check("390: update exit 0", u390.code === 0, u390.status);
  check("390: page has 600ms", (await ff.js("return window.__pg.text('docs/lamp.md')")).includes("`600ms`"));
  await ff.js("window.__pg.open('docs/lamp.md', 'preview')");
  await shot390("390-light-preview");
  const o2 = await noOverflow();
  check("390: no overflow after a run", o2.sw <= o2.iw && o2.sh <= o2.ih, o2);
  await ff.js("document.getElementById('out').scrollTop = 1e9");
  await shot390("390-light-output");
  await narrowSteps({ ff, check, shot390 });
  // The auto theme uses the system setting.
  await ff.js("window.__pg.setTheme('auto')");
  await ff.pref("layout.css.prefers-color-scheme.content-override", 0);
  await new Promise(r => setTimeout(r, 300));
  check("auto theme uses a dark system setting", await ff.js("return matchMedia('(prefers-color-scheme: dark)').matches && getComputedStyle(document.body).backgroundColor"));
  await shot390("390-auto-dark-system");
  await ff.pref("layout.css.prefers-color-scheme.content-override", 2);
  R.errors.push(...await ff.js("return window.__pg.errors"));
  await ff.frame();
  await ff.size(1280, 800);

  // Hard per-process limit, a kernel setting: ?limit=3 kills any process after 3 s.
  await load("?limit=3", "with ?limit=3");
  const lim = await runCmd("go build -a -o /tmp/lamp ./cmd/lamp", "go build -a with a 3 s process limit");
  const tl = await termTail(600);
  check("process limit kills", lim.code >= 256 && tl.includes("process limit of 3s"), { status: lim.status, ms: lim.ms, tail: tl.slice(-300) });
  R.errors.push(...await ff.js("return window.__pg.errors"));
  check("no page errors", R.errors.length === 0, R.errors);
} catch (e) {
  R.fatal = String(e && e.stack || e);
  console.log("FATAL", R.fatal);
  try { R.errors.push(...await ff.js("return window.__pg && window.__pg.errors || []")); await ff.shot(`${outDir}/shot-fatal.png`); } catch {}
}
// A known console line, to show that the console output of the page reaches firefox.log.
try { if (ff) await ff.js("console.error('console-capture-probe')"); } catch {}
fs.writeFileSync(`${outDir}/scenario.json`, JSON.stringify(R, null, 1));
if (ff) await ff.close();
const failed = R.checks.filter(c => c.name && !c.ok).length;
console.log(`checks: ${R.checks.filter(c => c.name && c.ok).length} pass, ${failed} fail${R.fatal ? ", fatal error" : ""}`);
process.exit(failed || R.fatal ? 1 : 0);
