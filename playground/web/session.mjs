// Shared by the Node harness and the browser page: loads bundles into the memfs and runs the real cmd/go,
// docgen and the bb shell and commands.
import { Kernel, MemFS, KILLED, SIGNAME } from "./kernel.mjs";
export const TOOLDIR = "/goroot/pkg/tool/js_wasm";
export const TOOLS = ["compile", "link", "asm", "vet"];
/** The names of the bb multi-call binary in /bin. */
export const BB = ["sh", "cat", "head", "tail", "grep", "sed", "wc", "ls", "echo", "printf", "tr", "cut", "sort", "uniq",
  "true", "false", "test", "[", "pwd", "basename", "dirname", "mkdir", "rm", "cp", "mv", "touch", "date"];
const dec = new TextDecoder();

/**
 * loadBundle writes the files of a bundle into the memfs. With a mtime argument, each file gets that
 * mtime in place of its own. The GOCACHE bundle needs the current time: cmd/go deletes cache
 * entries that are older than five days.
 */
export function loadBundle(fsys, blob, mtime) {
  const hl = new DataView(blob.buffer, blob.byteOffset).getUint32(0, true);
  const head = JSON.parse(dec.decode(blob.subarray(4, 4 + hl)));
  const base = 4 + hl;
  for (const [p, off, len, m] of head.files) fsys.writeFile(p, blob.subarray(base + off, base + off + len).slice(), mtime || m);
  return head.files.length;
}

/** describe returns the text of a wait status: "exit 1", or "killed by SIGKILL (137)". */
export function describe(code) {
  if (code >= KILLED) { const [name, n] = SIGNAME[code - KILLED] || ["signal " + (code - KILLED), code - KILLED]; return `killed by ${name} (${128 + n})`; }
  return `exit ${code}`;
}

/** loadTool(name) returns a Promise of a WebAssembly.Module for go, compile, link, asm, vet, docgen or bb. */
export function createSession({ loadTool, makeWorker, log = () => {}, binaries = {}, limitMs = 0 }) {
  const fsys = new MemFS();
  const tools = {};
  const map = { "/goroot/bin/go": "go", "/usr/local/bin/docgen": "docgen", ...binaries };
  for (const t of TOOLS) map[`${TOOLDIR}/${t}`] = t;
  for (const b of BB) map[`/bin/${b}`] = "bb";
  const loadModule = async (path, e) => {
    const m = map[path];
    if (m) return tools[m] || (tools[m] = loadTool(m));
    return WebAssembly.compile(e.data.subarray(0, e.size));
  };
  const kernel = new Kernel({ fs: fsys, loadModule, makeWorker, log, limitMs });
  const mark = new TextEncoder().encode("\0asm placeholder; the kernel maps this path to a module\n");
  for (const p of Object.keys(map)) fsys.writeFile(p, mark, 0).mode = 0o755;
  for (const d of ["/tmp", "/home", "/gopath", "/gocache"]) fsys.mkdirAll(d);
  const env = {
    GOROOT: "/goroot", GOCACHE: "/gocache", GOPATH: "/gopath", HOME: "/home", TMPDIR: "/tmp", PATH: "/goroot/bin:/usr/local/bin:/bin",
    GOPROXY: "off", GOTOOLCHAIN: "local", GOTELEMETRY: "off", GOFLAGS: "", CGO_ENABLED: "0", GOENV: "off",
  };
  /** start runs argv0 and streams its output to onOut and onErr. It returns {proc, done}; done resolves with {code, ms}. */
  function start(argv0, argv, cwd, extraEnv = {}, onOut = () => {}, onErr = () => {}) {
    const t0 = performance.now();
    const proc = kernel.run(argv0, argv, { ...env, ...extraEnv }, cwd, onOut, onErr);
    return { proc, done: proc.done.then(code => ({ code, ms: performance.now() - t0, reason: proc.reason })) };
  }
  async function exec(argv0, argv, cwd, extraEnv = {}) {
    let stdout = "", stderr = "";
    const r = await start(argv0, argv, cwd, extraEnv, b => { stdout += dec.decode(b); }, b => { stderr += dec.decode(b); }).done;
    return { ...r, stdout, stderr };
  }
  const go = (args, cwd = "/work/hello", extraEnv = {}) => exec("/goroot/bin/go", ["go", ...args], cwd, extraEnv);
  return { fsys, kernel, go, exec, start, env };
}

/** Serializes the memfs subtree at prefix in the bundle format. */
export function dumpTree(fsys, prefix) {
  const files = [], parts = [];
  let off = 0;
  const walk = (e, p) => {
    for (const [n, c] of e.children) {
      const q = p + "/" + n;
      if (c.dir) walk(c, q);
      else { const d = c.data.subarray(0, c.size); files.push([q, off, d.length, c.mtime]); parts.push(d); off += d.length; }
    }
  };
  walk(fsys.lookup(prefix), prefix);
  const hb = new TextEncoder().encode(JSON.stringify({ files }));
  const out = new Uint8Array(4 + hb.length + off);
  new DataView(out.buffer).setUint32(0, hb.length, true);
  out.set(hb, 4);
  let o = 4 + hb.length;
  for (const d of parts) { out.set(d, o); o += d.length; }
  return out;
}
