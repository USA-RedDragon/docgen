// Prints the license notices of the third-party code in a playground file.
// Usage:
//   node notices.mjs npm META NODE_MODULES  the npm packages in the esbuild metafile META
//   node notices.mjs go < LIST              the Go modules of LIST, lines of "path version dir"
//   node notices.mjs cargo SYSROOT < META    the crates of META, the output of cargo metadata of a
//                                           program, other than the program, and the Rust standard
//                                           library of the toolchain at SYSROOT
// Each notice is the name and version of the package or module, and the text of each of its
// license files. A package or module with no license file stops the script with exit 1. The notice of
// the Rust standard library is the text of COPYRIGHT-library.html of the toolchain, which holds the
// licenses of the library and of the crates that it uses.
import fs from "node:fs";
import path from "node:path";

const [mode, meta, modules] = process.argv.slice(2);

/** htmlText returns the text of an HTML page: the tags removed, the entities decoded and blank lines cut. */
function htmlText(html) {
  const named = { amp: "&", lt: "<", gt: ">", quot: '"', apos: "'", nbsp: " " };
  return html
    .replace(/<(script|style)[^>]*>[\s\S]*?<\/\1>/gi, "")
    .replace(/<[^>]*>/g, "")
    .replace(/&#x([0-9a-f]+);/gi, (_, hex) => String.fromCodePoint(parseInt(hex, 16)))
    .replace(/&#([0-9]+);/g, (_, dec) => String.fromCodePoint(Number(dec)))
    .replace(/&([a-z]+);/gi, (entity, name) => named[name] ?? entity)
    .split("\n")
    .map(line => line.trimEnd())
    .join("\n")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}
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
} else if (mode === "cargo") {
  const metadata = JSON.parse(fs.readFileSync(0, "utf8"));
  const members = new Set(metadata.workspace_members);
  const used = new Set(metadata.resolve.nodes.map(node => node.id));
  for (const pkg of metadata.packages) {
    if (members.has(pkg.id) || !used.has(pkg.id)) continue;
    items.set(pkg.name, { version: pkg.version, dir: path.dirname(pkg.manifest_path) });
  }
} else {
  console.error("usage: notices.mjs npm META NODE_MODULES | notices.mjs go < LIST | notices.mjs cargo SYSROOT < META");
  process.exit(2);
}
let failed = false;
for (const [name, { version, dir }] of [...items].sort((a, b) => a[0].localeCompare(b[0]))) {
  const files = fs.readdirSync(dir).filter(n => /^(LICEN[CS]E|COPYING|NOTICE)/i.test(n)).sort();
  if (!files.length) { console.error(`notices.mjs: ${name} ${version} has no license file in ${dir}`); failed = true; continue; }
  for (const n of files) process.stdout.write(`${"=".repeat(72)}\n${name} ${version}, ${n}\n\n${fs.readFileSync(path.join(dir, n), "utf8").trim()}\n\n`);
}
if (mode === "cargo") {
  const copyright = path.join(meta, "share", "doc", "rust", "COPYRIGHT-library.html");
  if (!fs.existsSync(copyright)) {
    console.error(`notices.mjs: no ${copyright}; it comes with the rustc component of the toolchain`);
    failed = true;
  } else {
    process.stdout.write(`${"=".repeat(72)}\nThe Rust standard library, COPYRIGHT-library.html of the toolchain\n\n${htmlText(fs.readFileSync(copyright, "utf8"))}\n\n`);
  }
}
process.exit(failed ? 1 : 0);
