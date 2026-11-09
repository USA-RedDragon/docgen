// Writes OUT/assets and OUT/manifest.json of the playground page.
// Usage: node mkmanifest.mjs OUT WORK BASE GOVERSION VERSION COMMIT NAME:KIND:EXT...
//   Each item is WORK/raw/NAME.EXT, compressed in WORK/z/NAME.EXT.zst. The decoder item is not
//   compressed. The name of each asset holds the start of its SHA-256, so a server can cache it
//   for a long time. BASE, when not empty, is an earlier output: its items that are not in the
//   list stay, with their assets.
import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";

const [out, work, base, go, version, commit, ...items] = process.argv.slice(2);
const assets = path.join(out, "assets");
fs.mkdirSync(assets, { recursive: true });
const order = ["go", "docgen", "bb", "compile", "link", "asm", "vet", "root", "gocache", "sample"];
const files = new Map();
let decoder = null;
if (base) {
  const m = JSON.parse(fs.readFileSync(path.join(base, "manifest.json"), "utf8"));
  if (m.go !== go) throw new Error(`${base}/manifest.json is for ${m.go}, not ${go}`);
  const keep = url => fs.copyFileSync(path.join(base, url), path.join(out, url));
  for (const f of m.files) { files.set(f.name, f); keep(f.url); }
  decoder = m.decoder; keep(decoder);
}
for (const it of items) {
  const [name, kind, ext] = it.split(":");
  const raw = path.join(work, "raw", `${name}.${ext}`);
  const data = fs.readFileSync(raw);
  const sha256 = crypto.createHash("sha256").update(data).digest("hex");
  if (kind === "decoder") {
    decoder = `assets/${name}.${sha256.slice(0, 16)}.${ext}`;
    fs.copyFileSync(raw, path.join(out, decoder));
    continue;
  }
  const url = `assets/${name}.${sha256.slice(0, 16)}.${ext}.zst`;
  fs.copyFileSync(path.join(work, "z", `${name}.${ext}.zst`), path.join(out, url));
  files.set(name, { name, kind, url, size: data.length, zsize: fs.statSync(path.join(out, url)).size, sha256 });
}
const list = [...files.values()].sort((a, b) => order.indexOf(a.name) - order.indexOf(b.name));
for (const n of order) if (!files.has(n)) throw new Error(`no ${n} asset`);
if (!decoder) throw new Error("no decoder asset");
const manifest = { go, docgen: { version, commit }, decoder, files: list };
fs.writeFileSync(path.join(out, "manifest.json"), JSON.stringify(manifest, null, 1) + "\n");
for (const f of list) console.log(`${f.name.padEnd(8)} ${String(f.size).padStart(9)} ${String(f.zsize).padStart(9)} ${f.url}`);
