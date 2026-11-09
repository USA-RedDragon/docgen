// Writes a bundle of files for the memfs of the playground.
// Usage: node mkbundle.mjs OUT [--goroot=DIR --stddirs=FILE] [--mods=DIR,...] [--prefix=/work]
//   --goroot and --stddirs add the Go source of each directory in the list at /goroot, without
//   tests and files of other systems, and the GOROOT files that cmd/go reads.
//   --mods adds each directory at <prefix>/<base name of the directory>.
// The format is the one of web/bundle.mjs. Each file has the mtime 2000-01-01, so the bundle is
// reproducible.
import fs from "node:fs";
import path from "node:path";
import { encodeBundle } from "../web/bundle.mjs";

const [out, ...flags] = process.argv.slice(2);
const opt = Object.fromEntries(
  flags.map(f => {
    const [k, ...v] = f.replace(/^--/, "").split("=");
    return [k, v.join("=")];
  }),
);
const OS =
  "aix android darwin dragonfly freebsd hurd illumos ios linux nacl netbsd openbsd plan9 solaris wasip1 windows zos".split(
    " ",
  );
const ARCH =
  "386 amd64 arm arm64 loong64 mips mipsle mips64 mips64le ppc64 ppc64le riscv64 s390x sparc64 amd64p32 armbe arm64be mips64p32 mips64p32le ppc riscv s390 sparc".split(
    " ",
  );
const other = new RegExp(`_(${OS.join("|")})(_(${ARCH.join("|")}|wasm))?\\.(go|s)$|_(${ARCH.join("|")})\\.(go|s)$`);
const MTIME = Date.UTC(2000, 0, 1);
const files = [];
const add = (dst, src) => files.push([dst, fs.readFileSync(src)]);
const walkAdd = (src, dst) => {
  for (const n of fs.readdirSync(src).sort()) {
    const f = path.join(src, n);
    if (fs.statSync(f).isDirectory()) walkAdd(f, dst + "/" + n);
    else add(dst + "/" + n, f);
  }
};
if (opt.goroot) {
  const root = path.resolve(opt.goroot);
  for (const rel of fs.readFileSync(opt.stddirs, "utf8").split("\n").filter(Boolean)) {
    const dir = path.join(root, rel);
    for (const n of fs.readdirSync(dir).sort()) {
      const f = path.join(dir, n);
      if (!fs.statSync(f).isFile() || n.endsWith("_test.go") || other.test(n)) continue;
      add(`/goroot/${rel}/${n}`, f);
    }
  }
  for (const f of ["go.env", "VERSION", "src/go.mod", "src/vendor/modules.txt"])
    add("/goroot/" + f, path.join(root, f));
  for (const n of fs.readdirSync(path.join(root, "pkg/include")).sort())
    add("/goroot/pkg/include/" + n, path.join(root, "pkg/include", n));
}
for (const m of (opt.mods || "").split(",").filter(Boolean)) walkAdd(m, `${opt.prefix || "/work"}/${path.basename(m)}`);
fs.writeFileSync(
  out,
  encodeBundle(
    {},
    files.map(([dst, data]) => [dst, data, MTIME]),
  ),
);
const contentLength = files.reduce((total, [, data]) => total + data.length, 0);
console.log(`${out}: ${files.length} files, ${contentLength} bytes of content`);
