// Checks a docs page of the site: it is not cross-origin isolated, its Google Fonts load, and its
// header has the Playground link. Usage: node docs-check.mjs MARIONETTE_PORT PAGE_URL WANT_HREF
import { connect } from "./ff.mjs";
const [port, url, want] = process.argv.slice(2);
const ff = await connect(port);
await ff.size(1280, 800);
await ff.go(url);
const r = await ff.ajs(`const done = arguments[arguments.length - 1];
  document.fonts.ready.then(() => done({
    isolated: crossOriginIsolated,
    font: document.fonts.check('16px "Nunito Sans"'),
    loaded: [...document.fonts].filter(f => f.status === "loaded").map(f => f.family),
    playground: [...document.querySelectorAll("header nav a")].filter(a => a.textContent === "Playground").map(a => a.getAttribute("href")),
  }));`);
await ff.close();
console.log(JSON.stringify(r));
const ok = !r.isolated && r.font && r.loaded.length > 0 && r.playground.length === 1 && r.playground[0] === want;
console.log(`${ok ? "PASS" : "FAIL"} docs page ${url}: not isolated, Google Fonts loaded, Playground link ${want}`);
process.exit(ok ? 0 : 1);
