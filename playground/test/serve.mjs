// A static server for a build of the docs site, for the browser test. It binds 127.0.0.1 only, and it
// applies the _headers file of the site in the way of Cloudflare Pages: each rule whose URL pattern
// matches the path adds its headers, and a splat (*) matches any text. A rule can set Content-Type and
// Content-Encoding. The server sends the bytes of the file as they are, so a file with
// Content-Encoding: zstd goes to the browser compressed, and the browser decodes it.
// Usage: node serve.mjs SITE_DIR PORT
import http from "node:http";
import fs from "node:fs";
import path from "node:path";

const root = path.resolve(process.argv[2]);
const port = Number(process.argv[3]);
const TYPES = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript",
  ".mjs": "text/javascript",
  ".css": "text/css",
  ".json": "application/json",
  ".svg": "image/svg+xml",
  ".wasm": "application/wasm",
  ".md": "text/markdown; charset=utf-8",
  ".txt": "text/plain; charset=utf-8",
};

/** readRules returns the rules of the _headers file: [{pattern, headers: [[name, value]]}]. */
function readRules() {
  let text = "";
  try {
    text = fs.readFileSync(path.join(root, "_headers"), "utf8");
  } catch {
    return [];
  }
  const rules = [];
  for (const line of text.split("\n")) {
    if (!line.trim() || line.trim().startsWith("#")) continue;
    if (/^\s/.test(line)) {
      if (!rules.length) continue;
      const colon = line.indexOf(":");
      rules.at(-1).headers.push([line.slice(0, colon).trim().toLowerCase(), line.slice(colon + 1).trim()]);
      continue;
    }
    const pattern = line
      .trim()
      .replace(/[.+?^${}()|[\]\\]/g, "\\$&")
      .replace(/\*/g, ".*")
      .replace(/:[A-Za-z]\w*/g, "[^/]+");
    rules.push({ pattern: new RegExp("^" + pattern + "$"), headers: [] });
  }
  return rules;
}

const rules = readRules();

function headersFor(urlPath) {
  const headers = {};
  for (const rule of rules) {
    if (!rule.pattern.test(urlPath)) continue;
    for (const [name, value] of rule.headers) headers[name] = headers[name] ? headers[name] + ", " + value : value;
  }
  return headers;
}

function statOf(file) {
  try {
    return fs.statSync(file);
  } catch {
    return null;
  }
}

const server = http.createServer((request, response) => {
  const url = new URL(request.url, "http://127.0.0.1");
  const urlPath = decodeURIComponent(url.pathname);
  let file = path.join(root, urlPath);
  if (file !== root && !file.startsWith(root + path.sep)) {
    response.writeHead(403);
    return response.end();
  }
  let stat = statOf(file);
  if (stat && stat.isDirectory() && !urlPath.endsWith("/")) {
    response.writeHead(308, { location: urlPath + "/" + url.search });
    return response.end();
  }
  if (stat && stat.isDirectory()) {
    file = path.join(file, "index.html");
    stat = statOf(file);
  }
  let status = 200;
  if (!stat || !stat.isFile()) {
    status = 404;
    file = path.join(root, "404.html");
    stat = statOf(file);
    if (!stat) {
      response.writeHead(404);
      return response.end();
    }
  }
  response.writeHead(status, {
    "content-type": TYPES[path.extname(file)] || "application/octet-stream",
    "content-length": stat.size,
    ...headersFor(url.pathname),
  });
  if (request.method === "HEAD") return response.end();
  fs.createReadStream(file).pipe(response);
});
server.listen(port, "127.0.0.1", () => console.log(`serving ${root} at http://127.0.0.1:${port}/`));
