// A static server for a build of the docs site, for tests. It binds 127.0.0.1 only, and it
// applies the _headers file of the site in the way of Cloudflare Pages: each rule whose URL
// pattern matches the path adds its headers, and a splat (*) matches any text. A rule can set
// Content-Type and Content-Encoding. The server sends the bytes of the file as they are, so a file
// with Content-Encoding: zstd goes to the browser compressed, and the browser decodes it.
// /__test/frame.html?src=URL is a page with the cross-origin isolation headers and a 390 x 844
// iframe of URL, because headless Firefox has a minimum window width of 500 px.
// Usage: node serve.mjs SITE_DIR PORT
import http from "node:http";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(process.argv[2]);
const port = +process.argv[3];
const types = { ".html": "text/html; charset=utf-8", ".js": "text/javascript", ".mjs": "text/javascript", ".css": "text/css", ".json": "application/json",
  ".svg": "image/svg+xml", ".wasm": "application/wasm", ".md": "text/markdown; charset=utf-8", ".txt": "text/plain; charset=utf-8" };
const isolation = { "cross-origin-opener-policy": "same-origin", "cross-origin-embedder-policy": "require-corp", "cross-origin-resource-policy": "same-origin" };

function readRules() {
  let text = "";
  try { text = fs.readFileSync(path.join(root, "_headers"), "utf8"); } catch { return []; }
  const rules = [];
  for (const line of text.split("\n")) {
    if (!line.trim() || line.trim().startsWith("#")) continue;
    if (!/^\s/.test(line)) {
      const re = "^" + line.trim().replace(/[.+?^${}()|[\]\\]/g, "\\$&").replace(/\*/g, ".*").replace(/:[A-Za-z]\w*/g, "[^/]+") + "$";
      rules.push({ re: new RegExp(re), headers: [] });
    } else if (rules.length) {
      const i = line.indexOf(":");
      rules.at(-1).headers.push([line.slice(0, i).trim().toLowerCase(), line.slice(i + 1).trim()]);
    }
  }
  return rules;
}
const rules = readRules();
function headersFor(p) {
  const h = {};
  for (const r of rules) if (r.re.test(p)) for (const [k, v] of r.headers) h[k] = h[k] ? h[k] + ", " + v : v;
  return h;
}

http.createServer((req, res) => {
  const u = new URL(req.url, "http://127.0.0.1");
  if (u.pathname === "/__test/frame.html") {
    const src = (u.searchParams.get("src") || "/").replace(/[^A-Za-z0-9/._?=&-]/g, "");
    res.writeHead(200, { "content-type": "text/html; charset=utf-8", ...isolation });
    return res.end(`<!doctype html><meta charset="utf-8"><title>390 px</title><style>body { margin: 0; background: #777; } iframe { display: block; width: 390px; height: 844px; border: 0; }</style><iframe id="f" src="${src}"></iframe>`);
  }
  const rel = decodeURIComponent(u.pathname);
  let p = path.join(root, rel);
  if (p !== root && !p.startsWith(root + path.sep)) { res.writeHead(403); return res.end(); }
  let st = null;
  try { st = fs.statSync(p); } catch {}
  if (st && st.isDirectory() && !rel.endsWith("/")) { res.writeHead(308, { location: rel + "/" + u.search }); return res.end(); }
  if (st && st.isDirectory()) { p = path.join(p, "index.html"); try { st = fs.statSync(p); } catch { st = null; } }
  let status = 200;
  if (!st || !st.isFile()) {
    status = 404;
    p = path.join(root, "404.html");
    try { st = fs.statSync(p); } catch { res.writeHead(404); return res.end(); }
  }
  res.writeHead(status, { "content-type": types[path.extname(p)] || "application/octet-stream", "content-length": st.size, ...headersFor(u.pathname) });
  if (req.method === "HEAD") return res.end();
  fs.createReadStream(p).pipe(res);
}).listen(port, "127.0.0.1", () => console.log(`serving ${root} at http://127.0.0.1:${port}/`));
