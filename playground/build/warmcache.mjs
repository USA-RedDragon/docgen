// Runs commands of the playground under Node, with the kernel of the page and one worker_threads
// Worker for each process, and writes the GOCACHE that they fill as a bundle.
// Usage: node warmcache.mjs BINDIR WASM_EXEC OUT BUNDLE... -- STEP...
//   BINDIR holds <tool>.wasm for go, compile, link, asm, vet, docgen and bb.
//   A step is "exec|DIR|ARGV0 ARG..." or "sh|DIR|shell command". The environment variable EXTRA
//   adds variables to the environment of each step, as JSON.
// A step that ends with a status other than its expected status (exit 0, or "!" before the step
// for a nonzero status) stops the script with exit 1.
import fs from "node:fs";
import path from "node:path";
import { Worker } from "node:worker_threads";

const web = path.join(path.dirname(new URL(import.meta.url).pathname), "..", "web");
const { createSession, loadBundle, dumpTree, describe } = await import(path.join(web, "session.mjs"));
const argv = process.argv.slice(2);
const sep = argv.indexOf("--");
const [bindir, wasmExec, out, ...bundles] = argv.slice(0, sep);
const steps = argv.slice(sep + 1);
const makeWorker = () => {
  const w = new Worker(path.join(web, "proc.js"), { workerData: { wasmExec: path.resolve(wasmExec) } });
  const o = { post: m => w.postMessage(m), terminate: () => w.terminate(), onmessage: null };
  w.on("message", m => o.onmessage && o.onmessage(m));
  w.on("error", e => console.error("worker error", e));
  return o;
};
const s = createSession({ loadTool: async n => WebAssembly.compile(fs.readFileSync(path.join(bindir, n + ".wasm"))), makeWorker });
s.kernel.toolEnv = { compile: { GOGC: "50" }, link: { GOGC: "50" } };
for (const b of bundles) console.log(`loaded ${b}: ${loadBundle(s.fsys, new Uint8Array(fs.readFileSync(b)))} files`);
const extra = process.env.EXTRA ? JSON.parse(process.env.EXTRA) : {};
let failed = false;
for (let st of steps) {
  const fail = st.startsWith("!");
  if (fail) st = st.slice(1);
  const [kind, dir, ...rest] = st.split("|");
  const cmd = rest.join("|");
  const r = kind === "sh" ? await s.exec("/bin/sh", ["sh", "-c", cmd], dir, extra) : await s.exec(cmd.split(" ")[0], cmd.split(" "), dir, extra);
  console.log(`== ${cmd}: ${describe(r.code)} in ${(r.ms / 1000).toFixed(2)} s`);
  if (r.stdout) console.log(r.stdout.trimEnd());
  if (r.stderr) console.log(r.stderr.slice(-4000).trimEnd());
  if ((r.code !== 0) !== fail) { console.log(`want ${fail ? "a nonzero status" : "exit 0"}`); failed = true; }
}
const b = dumpTree(s.fsys, "/gocache");
fs.writeFileSync(out, b);
console.log(`wrote ${out}: ${b.length} bytes`);
process.exit(failed ? 1 : 0);
