// Prints the license notices of the third-party code in a playground file.
// Usage:
//   node notices.mjs npm META NODE_MODULES  the npm packages in the esbuild metafile META
//   node notices.mjs go < LIST              the Go modules of LIST, lines of "path version dir"
// Each notice is the name and version of the package or module, and the text of each of its
// license files. A package or module with no license file stops the script with exit 1.
import fs from "node:fs";
import path from "node:path";

const [mode, meta, modules] = process.argv.slice(2);
const items = new Map();
if (mode === "npm") {
  for (const f of Object.keys(JSON.parse(fs.readFileSync(meta, "utf8")).inputs)) {
    const m = f.match(/node_modules\/((@[^/]+\/)?[^/]+)\//);
    if (!m || items.has(m[1])) continue;
    const dir = path.join(modules, m[1]);
    items.set(m[1], { version: JSON.parse(fs.readFileSync(path.join(dir, "package.json"), "utf8")).version, dir });
  }
} else if (mode === "go") {
  for (const line of fs.readFileSync(0, "utf8").split("\n")) {
    const [p, version, dir] = line.trim().split(" ");
    if (p && version && dir) items.set(p, { version, dir });
  }
} else {
  console.error("usage: notices.mjs npm META NODE_MODULES | notices.mjs go < LIST");
  process.exit(2);
}
let failed = false;
for (const [name, { version, dir }] of [...items].sort((a, b) => a[0].localeCompare(b[0]))) {
  const files = fs.readdirSync(dir).filter(n => /^(LICEN[CS]E|COPYING|NOTICE)/i.test(n)).sort();
  if (!files.length) { console.error(`notices.mjs: ${name} ${version} has no license file in ${dir}`); failed = true; continue; }
  for (const n of files) process.stdout.write(`${"=".repeat(72)}\n${name} ${version}, ${n}\n\n${fs.readFileSync(path.join(dir, n), "utf8").trim()}\n\n`);
}
process.exit(failed ? 1 : 0);
