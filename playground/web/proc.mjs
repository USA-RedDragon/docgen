// The Worker of one process: it runs a Go js/wasm program. The fs, process and spawn calls of the
// program go to the kernel on the main thread as synchronous calls (see callKernel). The module runs
// as a browser module Worker and as a Node worker_threads Worker.
import {
  S_IFMT, S_IFDIR, OPEN_FLAGS, WRITE_FLAGS, ERRNO, errnoError, FORWARDED_FS_CALLS, CALL_BUFFER, MESSAGE,
} from "./abi.mjs";

const textDecoder = new TextDecoder();
/** The descriptors of the GOROOT image start here, far above the descriptors of the kernel. */
const FIRST_LOCAL_DESCRIPTOR = 1 << 20;

let postToKernel = null;
let wasmExecLoaded = null;
let callState = null;
let callBytes = null;
let nextCallId = 1;
/** The callbacks of the calls that complete later, by call id. */
const pendingCalls = new Map();

let pid = 0;
let ppid = 0;
let cwd = "/";

// The read-only GOROOT image that all processes share: the open, stat, read, readdir and close calls
// of /goroot paths run in the Worker with no call to the kernel. The kernel refuses a write under
// /goroot with EROFS, so the image and the memfs stay the same.
let gorootIndex = null;
let gorootData = null;
let nextLocalDescriptor = FIRST_LOCAL_DESCRIPTOR;
const localDescriptors = new Map();

/**
 * callKernel sends the call name with args and data to the kernel and blocks until the reply. It
 * returns {result, data}, or {pendingId} for a call that completes later with a COMPLETE message. It
 * throws an Error with the errno code of a failed call.
 */
function callKernel(name, args, data) {
  Atomics.store(callState, CALL_BUFFER.STATE, 0);
  postToKernel({ type: MESSAGE.CALL, id: nextCallId++, name, args, data });
  // The syscalls of Go are synchronous, so the Worker blocks until the kernel writes the reply and
  // sets the state to 1. The kernel runs on the main thread, which cannot block, so only the Worker
  // waits.
  Atomics.wait(callState, CALL_BUFFER.STATE, 0);
  const jsonLength = callState[CALL_BUFFER.JSON_LENGTH];
  const dataLength = callState[CALL_BUFFER.DATA_LENGTH];
  // TextDecoder does not decode a view of a SharedArrayBuffer, so the JSON is copied first.
  const json = callBytes.slice(CALL_BUFFER.JSON_OFFSET, CALL_BUFFER.JSON_OFFSET + jsonLength);
  const reply = JSON.parse(textDecoder.decode(json));
  if (reply.errno) throw errnoError(reply.errno, reply.message);
  if (dataLength >= 0) {
    reply.data = callBytes.subarray(CALL_BUFFER.DATA_OFFSET, CALL_BUFFER.DATA_OFFSET + dataLength);
  }
  return reply;
}

/** callLater calls callback with args after the current fs call returns, as Node does. */
function callLater(callback, ...args) {
  queueMicrotask(() => callback(...args));
}

/** toNodeStats adds the fields and the isDirectory method of a Node fs.Stats to a stat result. */
function toNodeStats(stats) {
  stats.isDirectory = () => (stats.mode & S_IFMT) === S_IFDIR;
  stats.dev = 1;
  stats.nlink = 1;
  stats.uid = 0;
  stats.gid = 0;
  stats.rdev = 0;
  stats.blksize = 4096;
  stats.blocks = Math.ceil(stats.size / 512);
  stats.atimeMs = stats.mtimeMs;
  stats.ctimeMs = stats.mtimeMs;
  return stats;
}

/**
 * forwardCall returns an fs method that sends the call name to the kernel with each argument but the
 * last, the callback, and gives the callback the result, changed by convertResult.
 */
function forwardCall(name, convertResult = result => result) {
  return (...args) => {
    const callback = args.pop();
    let result;
    try {
      result = convertResult(callKernel(name, args).result);
    } catch (error) {
      return callLater(callback, error);
    }
    callLater(callback, null, result);
  };
}

function normalizePath(path) {
  if (!path.startsWith("/")) path = cwd + "/" + path;
  const parts = [];
  for (const part of path.split("/")) {
    if (!part || part === ".") continue;
    if (part === "..") parts.pop();
    else parts.push(part);
  }
  return "/" + parts.join("/");
}

/** gorootPath returns the absolute path of path when it is in the GOROOT image, else null. */
function gorootPath(path) {
  if (!gorootIndex) return null;
  const absolute = normalizePath(path);
  return absolute === "/goroot" || absolute.startsWith("/goroot/") ? absolute : null;
}

function gorootStats(entry) {
  return toNodeStats({ ino: entry.ino, mode: entry.mode, size: entry.size, mtimeMs: entry.mtimeMs });
}

const forwardOpen = forwardCall("open");
const forwardClose = forwardCall("close");
const forwardFstat = forwardCall("fstat", toNodeStats);
const forwardStat = forwardCall("stat", toNodeStats);
const forwardLstat = forwardCall("lstat", toNodeStats);
const forwardReaddir = forwardCall("readdir");

/** statPath runs stat or lstat. The memfs has no symbolic links, so both give the same result. */
function statPath(path, callback, forward) {
  const absolute = gorootPath(path);
  if (!absolute) return forward(path, callback);
  const entry = gorootIndex.get(absolute);
  if (!entry) return callLater(callback, errnoError(ERRNO.ENOENT));
  callLater(callback, null, gorootStats(entry));
}

/** The Node fs object that GOROOT/src/syscall/fs_js.go calls. */
const fs = {
  constants: OPEN_FLAGS,
  writeSync(fd, buffer) {
    return callKernel("writeSync", [fd, null], buffer.slice()).result;
  },
  write(fd, buffer, offset, length, position, callback) {
    let written;
    try {
      written = callKernel("write", [fd, position], buffer.slice(offset, offset + length)).result;
    } catch (error) {
      return callLater(callback, error);
    }
    callLater(callback, null, written);
  },
  read(fd, buffer, offset, length, position, callback) {
    const local = localDescriptors.get(fd);
    if (local) {
      const start = position != null ? position : local.position;
      const count = Math.max(0, Math.min(length, local.entry.size - start));
      const from = local.entry.offset + start;
      buffer.set(gorootData.subarray(from, from + count), offset);
      if (position == null) local.position += count;
      return callLater(callback, null, count);
    }
    let reply;
    try {
      reply = callKernel("read", [fd, length, position]);
    } catch (error) {
      return callLater(callback, error);
    }
    if (reply.pendingId) {
      pendingCalls.set(reply.pendingId, (count, data) => {
        if (count) buffer.set(data, offset);
        callback(null, count);
      });
      return;
    }
    if (reply.result) buffer.set(reply.data, offset);
    callLater(callback, null, reply.result);
  },
  pipe() {
    return callKernel("pipe", []).result;
  },
  open(path, flags, mode, callback) {
    const absolute = gorootPath(path);
    if (!absolute || flags & WRITE_FLAGS) return forwardOpen(path, flags, mode, callback);
    const entry = gorootIndex.get(absolute);
    if (!entry) return callLater(callback, errnoError(ERRNO.ENOENT));
    if (flags & OPEN_FLAGS.O_DIRECTORY && !entry.names) return callLater(callback, errnoError(ERRNO.ENOTDIR));
    const fd = nextLocalDescriptor++;
    localDescriptors.set(fd, { entry, position: 0 });
    callLater(callback, null, fd);
  },
  close(fd, callback) {
    if (localDescriptors.delete(fd)) return callLater(callback, null, null);
    forwardClose(fd, callback);
  },
  fstat(fd, callback) {
    const local = localDescriptors.get(fd);
    if (!local) return forwardFstat(fd, callback);
    callLater(callback, null, gorootStats(local.entry));
  },
  stat(path, callback) {
    statPath(path, callback, forwardStat);
  },
  lstat(path, callback) {
    statPath(path, callback, forwardLstat);
  },
  readdir(path, callback) {
    const absolute = gorootPath(path);
    if (!absolute) return forwardReaddir(path, callback);
    const entry = gorootIndex.get(absolute);
    if (!entry) return callLater(callback, errnoError(ERRNO.ENOENT));
    if (!entry.names) return callLater(callback, errnoError(ERRNO.ENOTDIR));
    callLater(callback, null, entry.names.slice());
  },
};
for (const name of FORWARDED_FS_CALLS) fs[name] = forwardCall(name);

/** The Node process object that package syscall reads. */
const processApi = {
  get pid() {
    return pid;
  },
  get ppid() {
    return ppid;
  },
  getuid: () => -1,
  getgid: () => -1,
  geteuid: () => -1,
  getegid: () => -1,
  getgroups: () => [],
  umask: () => 0o022,
  cwd: () => cwd,
  chdir: dir => {
    cwd = callKernel("chdir", [dir]).result;
  },
};

/** The globals of the Go program: fs, process and path of Node, and the calls of playground/overlay/proc_js.go.txt. */
const goGlobals = {
  fs,
  process: processApi,
  path: {
    resolve: (...paths) => paths.reduce((resolved, path) => (path.startsWith("/") ? path : resolved + "/" + path), cwd),
  },
  goSpawn: (argv0, argv, env, dir, fds) => {
    if (fds.some(fd => localDescriptors.has(fd))) return { code: ERRNO.EBADF, message: "read-only fd" };
    return callKernel("spawn", [argv0, argv, env, dir, fds]).result;
  },
  goKill: (childPid, signal) => callKernel("kill", [childPid, signal]).result,
  goWait: childPid => {
    let reply;
    try {
      reply = callKernel("wait", [childPid]);
    } catch {
      return null;
    }
    if (reply.result === null && !reply.pendingId) return null;
    return new Promise(resolve => {
      if (reply.pendingId) pendingCalls.set(reply.pendingId, resolve);
      else resolve(reply.result);
    });
  },
};
const goGlobal = new Proxy(globalThis, {
  get: (target, key) => (key in goGlobals ? goGlobals[key] : Reflect.get(target, key)),
});

/** start runs the program of a START message, then posts EXIT or CRASH to the kernel. */
async function start(message) {
  callState = new Int32Array(message.buffer, 0, 4);
  callBytes = new Uint8Array(message.buffer);
  pid = message.pid;
  ppid = message.ppid;
  cwd = message.cwd;
  if (message.goroot) {
    gorootIndex = new Map(message.goroot.index);
    gorootData = new Uint8Array(message.goroot.buffer);
  }
  let exitCode = 0;
  let instance = null;
  try {
    await wasmExecLoaded;
    const go = new Go();
    go.global = goGlobal;
    go.argv = message.argv;
    go.env = message.env;
    go.exit = code => {
      exitCode = code;
    };
    instance = await WebAssembly.instantiate(message.module, go.importObject);
    await go.run(instance);
    postToKernel({
      type: MESSAGE.EXIT,
      code: exitCode,
      memoryBytes: instance.exports.mem.buffer.byteLength,
    });
  } catch (error) {
    postToKernel({
      type: MESSAGE.CRASH,
      text: String((error && error.stack) || error),
      memoryBytes: instance ? instance.exports.mem.buffer.byteLength : 0,
    });
  }
}

function handleMessage(message) {
  if (message.type === MESSAGE.START) return start(message);
  if (message.type !== MESSAGE.COMPLETE) return;
  const complete = pendingCalls.get(message.id);
  pendingCalls.delete(message.id);
  if (complete) complete(message.result, message.data);
}

// The kernel can post START while this module waits at an await. A browser Worker loses a message that
// arrives when it has no handler, so the handler is set before the first await. The parentPort of Node
// keeps the messages until it has a listener.
if (typeof globalThis.WorkerGlobalScope === "undefined") {
  const { parentPort, workerData } = await import("node:worker_threads");
  const { pathToFileURL } = await import("node:url");
  postToKernel = message => parentPort.postMessage(message);
  parentPort.on("message", handleMessage);
  wasmExecLoaded = import(pathToFileURL(workerData.wasmExec).href);
} else {
  postToKernel = message => self.postMessage(message);
  self.onmessage = event => handleMessage(event.data);
  wasmExecLoaded = import("./wasm_exec.js");
}
