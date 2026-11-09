// Builds web/bundles/<name>.bin: a GOROOT subset at /goroot, the sample modules at /work, and optionally a GOCACHE.
// Usage: node mkroot.mjs <out.bin> [--noroot] [--mods=<dir>,...] [--gocache=<dir>]; a module dir goes to /work/<base name>.
// Format: u32 header length, JSON header {files: [[path, off, len, mtimeMs]]}, then file bytes.
import fs from "node:fs";
import path from "node:path";
const P = path.join(path.dirname(new URL(import.meta.url).pathname), "..");
const walkAdd = (src, dst) => { for (const n of fs.readdirSync(src)) { const f = path.join(src, n); fs.statSync(f).isDirectory() ? walkAdd(f, dst + "/" + n) : add(dst + "/" + n, f); } };
const GOROOT = "/usr/lib/go";
const [out, ...flags] = process.argv.slice(2);
const opt = Object.fromEntries(flags.map(f => f.replace(/^--/, "").split("=")));
const OS = "aix android darwin dragonfly freebsd hurd illumos ios linux nacl netbsd openbsd plan9 solaris wasip1 windows zos".split(" ");
const ARCH = "386 amd64 arm arm64 loong64 mips mipsle mips64 mips64le ppc64 ppc64le riscv64 s390x sparc64 amd64p32 armbe arm64be mips64p32 mips64p32le ppc riscv s390 sparc".split(" ");
const other = new RegExp(`_(${OS.join("|")})(_(${ARCH.join("|")}|wasm))?\\.(go|s)$|_(${ARCH.join("|")})\\.(go|s)$`);
const files = [];
const add = (dst, src) => { const st = fs.statSync(src); files.push([dst, fs.readFileSync(src), Math.floor(st.mtimeMs)]); };
if (!("noroot" in opt)) for (const dir of fs.readFileSync(path.join(P, "stddirs.txt"), "utf8").split("\n").filter(Boolean)) {
  for (const n of fs.readdirSync(dir)) {
    const f = path.join(dir, n);
    if (!fs.statSync(f).isFile() || n.endsWith("_test.go") || other.test(n)) continue;
    add("/goroot" + f.slice(GOROOT.length), f);
  }
}
if (!("noroot" in opt)) for (const f of ["go.env", "VERSION", "src/go.mod", "src/vendor/modules.txt"]) add("/goroot/" + f, path.join(GOROOT, f));
if (!("noroot" in opt)) for (const n of fs.readdirSync(path.join(GOROOT, "pkg/include"))) add("/goroot/pkg/include/" + n, path.join(GOROOT, "pkg/include", n));
for (const m of (opt.mods || "").split(",").filter(Boolean)) walkAdd(path.join(P, m), `/work/${path.basename(m)}`);
if (opt.gocache) {
  const walk = d => { for (const n of fs.readdirSync(d)) { const f = path.join(d, n); fs.statSync(f).isDirectory() ? walk(f) : add("/gocache" + f.slice(opt.gocache.length), f); } };
  walk(opt.gocache);
}
let off = 0;
const head = { files: files.map(([p, d, m]) => { const r = [p, off, d.length, m]; off += d.length; return r; }) };
const hb = Buffer.from(JSON.stringify(head));
const lb = Buffer.alloc(4); lb.writeUInt32LE(hb.length);
fs.writeFileSync(out, Buffer.concat([lb, hb, ...files.map(f => f[1])]));
console.log(`${out}: ${files.length} files, ${off} bytes of content`);
