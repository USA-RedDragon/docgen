// Runs commands of the playground under Node, with the kernel of the page and one worker_threads
// Worker for each process, and writes the GOCACHE that they fill as a bundle.
// Usage: node warmcache.mjs BINDIR WASM_EXEC OUT BUNDLE... -- STEP...
//   BINDIR holds <tool>.wasm for go, compile, link, asm, vet, docgen and bb.
//   A step is "exec|DIR|ARGV0 ARG..." or "sh|DIR|shell command". The environment variable EXTRA
//   adds variables to the environment of each step, as JSON.
// When a step ends with a status other than its expected status (exit 0, or "!" before the step
// for a nonzero status), the script runs the other steps and then exits 1.
import fs from "node:fs";
import path from "node:path";
import { Worker } from "node:worker_threads";

const web = path.join(path.dirname(new URL(import.meta.url).pathname), "..", "web");
const { createSession, loadBundle, dumpTree, describe } = await import(path.join(web, "session.mjs"));
const args = process.argv.slice(2);
const separator = args.indexOf("--");
const [binDir, wasmExec, out, ...bundles] = args.slice(0, separator);
const steps = args.slice(separator + 1);

function makeWorker() {
  const worker = new Worker(path.join(web, "proc.mjs"), { workerData: { wasmExec: path.resolve(wasmExec) } });
  const handle = { post: message => worker.postMessage(message), terminate: () => worker.terminate(), onmessage: null };
  worker.on("message", message => handle.onmessage && handle.onmessage(message));
  worker.on("error", error => console.error("worker error", error));
  return handle;
}

const loadTool = async name => WebAssembly.compile(fs.readFileSync(path.join(binDir, name + ".wasm")));
const session = createSession({ loadTool, makeWorker });
for (const bundle of bundles) {
  const count = loadBundle(session.fsys, new Uint8Array(fs.readFileSync(bundle)));
  console.log(`loaded ${bundle}: ${count} files`);
}
const extraEnv = process.env.EXTRA ? JSON.parse(process.env.EXTRA) : {};
let failed = false;
for (let step of steps) {
  const wantFailure = step.startsWith("!");
  if (wantFailure) step = step.slice(1);
  const [kind, dir, ...rest] = step.split("|");
  const command = rest.join("|");
  let result;
  if (kind === "sh") {
    result = await session.exec("/bin/sh", ["sh", "-c", command], dir, extraEnv);
  } else {
    const argv = command.split(" ");
    result = await session.exec(argv[0], argv, dir, extraEnv);
  }
  console.log(`== ${command}: ${describe(result.code)} in ${(result.ms / 1000).toFixed(2)} s`);
  if (result.stdout) console.log(result.stdout.trimEnd());
  if (result.stderr) console.log(result.stderr.slice(-4000).trimEnd());
  if ((result.code !== 0) !== wantFailure) {
    console.log(`want ${wantFailure ? "a nonzero status" : "exit 0"}`);
    failed = true;
  }
}
const gocache = dumpTree(session.fsys, "/gocache");
fs.writeFileSync(out, gocache);
console.log(`wrote ${out}: ${gocache.length} bytes`);
process.exit(failed ? 1 : 0);
