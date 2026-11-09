// A session of the playground: a memfs with the Go toolchain, docgen and the bb shell and commands, and
// a kernel that runs them. The page and the Node scripts of the build both use it.
import { Kernel, MemFS } from "./kernel.mjs";
import { KILLED, SIGNAL_NAMES } from "./abi.mjs";
import { encodeBundle, decodeBundle } from "./bundle.mjs";

const TOOL_DIR = "/goroot/pkg/tool/js_wasm";
const TOOLS = ["compile", "link", "asm", "vet"];
/** The commands of the bb multi-call binary in /bin. */
const BB_COMMANDS = [
  "sh", "cat", "head", "tail", "grep", "sed", "wc", "ls", "echo", "printf", "tr", "cut", "sort", "uniq",
  "true", "false", "test", "[", "pwd", "basename", "dirname", "mkdir", "rm", "cp", "mv", "touch", "date",
];
/** The content of each program file in the memfs. The kernel runs the tool of its path instead. */
const PLACEHOLDER = new TextEncoder().encode("\0asm placeholder; the kernel maps this path to a module\n");
const ENV = {
  GOROOT: "/goroot",
  GOCACHE: "/gocache",
  GOPATH: "/gopath",
  HOME: "/home",
  TMPDIR: "/tmp",
  PATH: "/goroot/bin:/usr/local/bin:/bin",
  GOPROXY: "off",
  GOTOOLCHAIN: "local",
  GOTELEMETRY: "off",
  GOFLAGS: "",
  CGO_ENABLED: "0",
  GOENV: "off",
};

/**
 * loadBundle writes the files of a bundle into the memfs and returns their number. With an mtime
 * argument, each file gets that mtime in place of its own. The GOCACHE bundle needs the current time:
 * cmd/go deletes cache entries that are older than five days.
 */
export function loadBundle(fsys, bundle, mtime) {
  const { header, content } = decodeBundle(bundle);
  for (const [path, offset, length, fileMtime] of header.files) {
    fsys.writeFile(path, content.subarray(offset, offset + length).slice(), mtime || fileMtime);
  }
  return header.files.length;
}

/** dumpTree returns a bundle of the files under the directory dir of the memfs. */
export function dumpTree(fsys, dir) {
  const files = [];
  fsys.walk(dir, (path, entry) => {
    if (!entry.dir) files.push([path, entry.data.subarray(0, entry.size), entry.mtime]);
  });
  return encodeBundle({}, files);
}

/** describe returns the text of a wait status: "exit 1", or "killed by SIGKILL (137)". */
export function describe(code) {
  if (code < KILLED) return `exit ${code}`;
  const signal = code - KILLED;
  const [name, number] = SIGNAL_NAMES[signal] || ["signal " + signal, signal];
  return `killed by ${name} (${128 + number})`;
}

/**
 * createSession returns a new session: {fsys, kernel, start, exec}. loadTool(name) returns a Promise of
 * the WebAssembly.Module of go, compile, link, asm, vet, docgen or bb. makeWorker and limitMs go to
 * the Kernel.
 */
export function createSession({ loadTool, makeWorker, limitMs = 0 }) {
  const fsys = new MemFS();
  const tools = {};
  const toolOfPath = { "/goroot/bin/go": "go", "/usr/local/bin/docgen": "docgen" };
  for (const tool of TOOLS) toolOfPath[`${TOOL_DIR}/${tool}`] = tool;
  for (const command of BB_COMMANDS) toolOfPath[`/bin/${command}`] = "bb";
  const loadModule = async (path, entry) => {
    const tool = toolOfPath[path];
    if (!tool) return WebAssembly.compile(entry.data.subarray(0, entry.size));
    if (!tools[tool]) tools[tool] = loadTool(tool);
    return tools[tool];
  };
  const kernel = new Kernel({ fs: fsys, loadModule, makeWorker, limitMs });
  kernel.toolEnv = { compile: { GOGC: "50" }, link: { GOGC: "50" } };
  for (const path of Object.keys(toolOfPath)) fsys.writeFile(path, PLACEHOLDER, 0).mode = 0o755;
  for (const dir of ["/tmp", "/home", "/gopath", "/gocache"]) fsys.mkdirAll(dir);

  /**
   * start runs argv0 in cwd and sends its standard output and error to onOut and onErr. It returns
   * {proc, done}; done resolves with {code, ms, reason}.
   */
  function start(argv0, argv, cwd, extraEnv = {}, onOut = () => {}, onErr = () => {}) {
    const startedAt = performance.now();
    const proc = kernel.run(argv0, argv, { ...ENV, ...extraEnv }, cwd, onOut, onErr);
    const done = proc.done.then(code => ({ code, ms: performance.now() - startedAt, reason: proc.reason }));
    return { proc, done };
  }

  /** exec runs argv0 in cwd and resolves with {code, ms, reason, stdout, stderr}. */
  async function exec(argv0, argv, cwd, extraEnv = {}) {
    const textDecoder = new TextDecoder();
    let stdout = "";
    let stderr = "";
    const onOut = bytes => (stdout += textDecoder.decode(bytes));
    const onErr = bytes => (stderr += textDecoder.decode(bytes));
    const result = await start(argv0, argv, cwd, extraEnv, onOut, onErr).done;
    return { ...result, stdout, stderr };
  }

  return { fsys, kernel, exec, start };
}
