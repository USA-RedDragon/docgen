// Checks DIR/manifest.json of a playground from a download before any of its values becomes a
// path: the Go version has the form go1.27.0; each url is assets/zstd/ and a plain file name
// that ends in .zst for an asset with the encoding zstd, or assets/ and a plain file name for
// the encoding identity; and each asset is a regular file in DIR with the size of the manifest.
// Usage: node checkmanifest.mjs DIR. It prints the Go version, or a message and exit 1.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const URL_FORM = { zstd: /^assets\/zstd\/[A-Za-z0-9][A-Za-z0-9._-]*\.zst$/, identity: /^assets\/[A-Za-z0-9][A-Za-z0-9._-]*$/ };
export const GO_FORM = /^go[0-9]+\.[0-9]+(\.[0-9]+)?$/;
export const NAMES = ["go", "docgen", "bb", "compile", "link", "asm", "vet", "root", "gocache", "sample"];

/** check returns the manifest of dir, or throws an Error with the first problem. */
export function check(dir) {
  const m = JSON.parse(fs.readFileSync(path.join(dir, "manifest.json"), "utf8"));
  if (typeof m.go !== "string" || !GO_FORM.test(m.go)) throw new Error("the Go version is not of the form go1.27.0");
  if (!Array.isArray(m.files)) throw new Error("no file list");
  const names = m.files.map(f => f && f.name);
  for (const n of NAMES) if (!names.includes(n)) throw new Error(`no ${n} asset`);
  for (const f of m.files) {
    if (!NAMES.includes(f.name) || !/^[0-9a-f]{64}$/.test(f.sha256) || !Number.isSafeInteger(f.size) || !Number.isSafeInteger(f.sent) || !Object.hasOwn(URL_FORM, f.encoding)) throw new Error(`the entry of ${JSON.stringify(f && f.name)} is not valid`);
    if (typeof f.url !== "string" || !URL_FORM[f.encoding].test(f.url) || f.url.includes("..")) throw new Error(`the url ${JSON.stringify(f.url)} does not have the form of the encoding ${f.encoding}`);
    const st = fs.lstatSync(path.join(dir, f.url), { throwIfNoEntry: false });
    if (!st || !st.isFile()) throw new Error(`${f.url} is not a regular file`);
    if (st.size !== f.sent) throw new Error(`${f.url} has ${st.size} bytes, not ${f.sent}`);
  }
  return m;
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  try {
    console.log(check(process.argv[2]).go);
  } catch (e) {
    console.error(`checkmanifest.mjs: ${process.argv[2]}/manifest.json: ${e.message}`);
    process.exit(1);
  }
}
