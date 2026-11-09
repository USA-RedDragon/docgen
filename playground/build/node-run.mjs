// Runs the browser session code under Node (worker_threads, one Worker per process).
// Usage: node node-run.mjs <bindir> <bundle.bin>... -- [steps...]
//   step = "dir|go args" | "write|file|content" | "exec|dir|argv0 args" | "sh|dir|shell command" ; env EXTRA='{"K":"V"}' adds env.
import fs from "node:fs";
import path from "node:path";
import { Worker } from "node:worker_threads";
const P = path.join(path.dirname(new URL(import.meta.url).pathname), "..");
const { createSession, loadBundle, dumpTree, describe } = await import(path.join(P, "web/session.mjs"));
const argv = process.argv.slice(2);
const sep = argv.indexOf("--");
const [bindir, ...bundles] = sep < 0 ? argv : argv.slice(0, sep);
const steps = sep < 0 ? [] : argv.slice(sep + 1);
const wasmExec = path.join(P, "web/wasm_exec.js");
const makeWorker = () => {
  const w = new Worker(path.join(P, "web/proc.js"), { workerData: { wasmExec } });
  const o = { post: m => w.postMessage(m), terminate: () => w.terminate(), onmessage: null };
  w.on("message", m => o.onmessage && o.onmessage(m));
  w.on("error", e => console.error("worker error", e));
  return o;
};
const s = createSession({
  loadTool: async n => WebAssembly.compile(fs.readFileSync(path.join(bindir, n + ".wasm"))),
  makeWorker, log: m => process.env.KLOG && console.error(m),
  limitMs: +(process.env.LIMITMS || 0),
});
if (process.env.DEBUGPIPE) {
  const wd = s.kernel.writeDesc.bind(s.kernel);
  s.kernel.writeDesc = (d, b, pos) => { if (d.type === "pw") console.error(`pw write pipe ${d.pipe.id ??= Math.random().toString(36).slice(2, 6)} ${b.length} readers=${d.pipe.readers} writers=${d.pipe.writers} :: ${new TextDecoder().decode(b.slice(0, 60)).replace(/\n/g, "|")}`); return wd(d, b, pos); };
  const rel = s.kernel.release.bind(s.kernel);
  s.kernel.release = d => { if (d.type === "pr" && d.refs === 1) console.error(`release pr of pipe ${d.pipe.id ??= Math.random().toString(36).slice(2, 6)}`); return rel(d); };
  const op = s.kernel.op.bind(s.kernel);
  s.kernel.op = (p, name, a, din, done) => { const before = name === "close" ? p.fds.get(a[0]) : null; const r = op(p, name, a, din, done); if (before && before.pipe) console.error(`pid ${p.pid} ${p.tool} close fd ${a[0]} ${before.type} ${before.pipe.id ??= Math.random().toString(36).slice(2, 6)} refs=${before.refs}`); if (name === "read" && p.fds.get(a[0])?.type === "pr") console.error(`pid ${p.pid} ${p.tool} read fd ${a[0]} -> ${typeof r === "symbol" ? "PENDING" : r.res}`); const pid_ = fd => { const d = p.fds.get(fd); return d ? d.type + ":" + (d.pipe ? (d.pipe.id ??= Math.random().toString(36).slice(2, 6)) + "/r" + d.pipe.readers + "w" + d.pipe.writers : "") : "none"; };
    if (name === "pipe") console.error(`pid ${p.pid} ${p.tool} pipe -> ${r.res} ${r.res.map(pid_)}`);
    if (name === "spawn") console.error(`pid ${p.pid} ${p.tool} spawn fds ${a[4]} ${a[4].map(pid_)} -> ${r.res}`);
    if (name === "close" && a[0] < 3) console.error(`pid ${p.pid} ${p.tool} close ${a[0]}`); return r; };
}
for (const b of bundles) console.log(`loaded ${b}: ${loadBundle(s.fsys, new Uint8Array(fs.readFileSync(b)))} files`);
const extra = process.env.EXTRA ? JSON.parse(process.env.EXTRA) : {};
let peak = 0;
const iv = setInterval(() => { peak = Math.max(peak, process.memoryUsage().rss); }, 200);
for (const st of steps) {
  if (st.startsWith("write|")) { const [, f, c] = st.split("|"); s.fsys.writeFile(f, new TextEncoder().encode(c.replace(/\\n/g, "\n"))); continue; }
  if (st.startsWith("cat|")) { console.log(new TextDecoder().decode(s.fsys.readFile(st.slice(4)))); continue; }
  let r, label;
  if (st.startsWith("sh|")) {
    const [, dir, ...rest] = st.split("|"); const cmd = rest.join("|"); label = "sh -c " + cmd;
    r = await s.exec("/bin/sh", ["sh", "-c", cmd], dir, extra);
  } else if (st.startsWith("exec|")) {
    const [, dir, cmd] = st.split("|"); label = cmd;
    const a = cmd.split(" ");
    r = await s.exec(a[0], a, dir, extra);
  } else {
    const [dir, cmd] = st.split("|"); label = "go " + cmd;
    r = await s.go(cmd.split(" "), dir, extra);
  }
  console.log(`== ${label}: ${describe(r.code)} in ${(r.ms / 1000).toFixed(2)}s`);
  if (r.stdout) console.log(r.stdout.trimEnd());
  if (r.stderr) console.log("stderr:", r.stderr.slice(-4000).trimEnd());
}
clearInterval(iv);
if (process.env.DUMPCACHE) { const b = dumpTree(s.fsys, "/gocache"); fs.writeFileSync(process.env.DUMPCACHE, b); console.log("wrote", process.env.DUMPCACHE, b.length, "bytes"); }
if (process.env.DUMPFILE) for (const f of process.env.DUMPFILE.split(",")) { const [src, dst] = f.split("="); fs.writeFileSync(dst, s.fsys.readFile(src)); }
const { ops, ...rest } = s.kernel.stats;
console.log("stats", JSON.stringify(rest));
console.log("ops", JSON.stringify(ops));
console.log("memfs", JSON.stringify(s.fsys.stats()), "peak rss MiB", Math.round(Math.max(peak, process.memoryUsage().rss) / 2 ** 20));
process.exit(0);
