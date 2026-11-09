// Writes OUT/assets and OUT/manifest.json of the playground page.
// Usage: node mkmanifest.mjs OUT WORK BASE GOVERSION VERSION COMMIT NAME:KIND:EXT...
//   Each item is WORK/raw/NAME.EXT. When WORK/z/NAME.EXT.zst exists, the asset is that zstd file
//   at assets/zstd/, and _headers gives it Content-Encoding: zstd, so the browser decodes it.
//   Else the asset is the raw file at assets/. The name of each asset holds the start of the
//   SHA-256 of the raw file, so a server can cache it for a long time. BASE, when not empty, is
//   an earlier output: its items that are not in the list stay, with their assets.
//   checkmanifest.mjs checks the manifest of BASE first.
import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { check, NAMES } from "./checkmanifest.mjs";

const [out, work, base, go, version, commit, ...items] = process.argv.slice(2);
fs.mkdirSync(path.join(out, "assets", "zstd"), { recursive: true });
const files = new Map();
if (base) {
  const m = check(base);
  if (m.go !== go) throw new Error(`${base}/manifest.json is for ${m.go}, not ${go}`);
  for (const f of m.files) { files.set(f.name, f); fs.copyFileSync(path.join(base, f.url), path.join(out, f.url)); }
}
for (const it of items) {
  const [name, kind, ext] = it.split(":");
  const raw = path.join(work, "raw", `${name}.${ext}`);
  const data = fs.readFileSync(raw);
  const sha256 = crypto.createHash("sha256").update(data).digest("hex");
  const z = path.join(work, "z", `${name}.${ext}.zst`);
  const zstd = fs.existsSync(z);
  const url = zstd ? `assets/zstd/${name}-${sha256.slice(0, 16)}.${ext}.zst` : `assets/${name}-${sha256.slice(0, 16)}.${ext}`;
  fs.copyFileSync(zstd ? z : raw, path.join(out, url));
  files.set(name, { name, kind, url, encoding: zstd ? "zstd" : "identity", size: data.length, sent: fs.statSync(path.join(out, url)).size, sha256 });
}
for (const n of NAMES) if (!files.has(n)) throw new Error(`no ${n} asset`);
const list = NAMES.map(n => files.get(n));
fs.writeFileSync(path.join(out, "manifest.json"), JSON.stringify({ go, docgen: { version, commit }, files: list }, null, 1) + "\n");
for (const f of list) console.log(`${f.name.padEnd(8)} ${String(f.size).padStart(9)} ${String(f.sent).padStart(9)} ${f.url}`);
