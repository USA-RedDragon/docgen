// The kernel of the Go js/wasm processes of the playground. It runs on the main thread and holds the
// memfs, the descriptor tables, the pipes and the process table. Each process runs in its own Worker
// (proc.mjs) and calls the kernel synchronously: it posts a CALL message and blocks with Atomics.wait
// until the kernel writes the reply into a SharedArrayBuffer. A call that must block, such as a pipe
// read with no data or a wait for a child, gets a pending reply and completes later with a COMPLETE
// message. The kernel terminates the Worker of a process when the process exits or is killed.
import {
  S_IFREG, S_IFDIR, S_IFIFO, S_IFCHR, OPEN_FLAGS, O_ACCMODE, WRITE_FLAGS, ERRNO, errnoError, CALL_BUFFER,
  MESSAGE, SIG, SIGNAL_NAMES, KILLED,
} from "./abi.mjs";
import { normalizePath } from "./paths.mjs";

const textEncoder = new TextEncoder();
/** The tools of Go that read their arguments from a response file "@file". */
const RESPONSE_FILE_TOOLS = new Set(["compile", "link", "asm", "cgo", "cover", "pack"]);
/** The size of the arguments and environment above which such a tool gets a response file. */
const RESPONSE_FILE_MIN_SIZE = 6000;
/** The calls that can change the file at their first path argument, and for rename and link the second. */
const PATH_WRITE_CALLS = new Set(["open", "mkdir", "unlink", "rmdir", "rename", "truncate", "chmod", "utimes", "link", "symlink"]);
const PENDING = Symbol("pending");
/** The ppid of a process with no parent, a command of the page. */
const NO_PARENT = 1;
const FIRST_PID = 100;
/** The first descriptor after standard input, output and error. */
const FIRST_FREE_DESCRIPTOR = 3;
const STDOUT = 1;
const STDERR = 2;

const DURATION_UNITS_MS = { ns: 1e-6, us: 1e-3, "µs": 1e-3, ms: 1, s: 1000, m: 60000, h: 3600000 };

/** parseDuration parses a Go duration such as "2s", "1m30s" or "500ms" to milliseconds. It returns NaN for bad text. */
function parseDuration(text) {
  if (text === "0") return 0;
  const term = /([0-9]*\.?[0-9]+)(ns|us|µs|ms|s|m|h)/gy;
  let total = 0;
  let parsedLength = 0;
  let match;
  while ((match = term.exec(text))) {
    total += parseFloat(match[1]) * DURATION_UNITS_MS[match[2]];
    parsedLength = term.lastIndex;
  }
  return parsedLength === text.length && parsedLength > 0 ? total : NaN;
}

/** testTimeoutOf returns the value of the -test.timeout argument of argv, or "". */
function testTimeoutOf(argv) {
  const index = argv.findIndex(arg => /^--?test\.timeout(=|$)/.test(arg));
  if (index < 0) return "";
  const arg = argv[index];
  if (arg.includes("=")) return arg.slice(arg.indexOf("=") + 1);
  return argv[index + 1] || "";
}

/**
 * MemFS is the file system of the playground, in memory. An entry is a file or a directory:
 * {dir, children, data, size, ino, mtime, mode}. A directory has children, a Map of name to entry.
 * A file has data, whose first size bytes are the content.
 */
export class MemFS {
  constructor() {
    this.nextIno = 1;
    this.root = this.newEntry(true);
    this.devNull = this.newEntry(false);
    this.devNull.isCharDevice = true;
  }

  /** newEntry returns a new directory (dir true) or empty file that is in no directory yet. */
  newEntry(dir, mtime = Date.now()) {
    return {
      dir,
      children: dir ? new Map() : null,
      data: dir ? null : new Uint8Array(0),
      size: 0,
      ino: this.nextIno++,
      mtime,
      mode: dir ? 0o755 : 0o644,
    };
  }

  /** lookup returns the entry at the absolute path, or null. */
  lookup(path) {
    if (path === "/dev/null") return this.devNull;
    let entry = this.root;
    for (const name of path.split("/")) {
      if (!name) continue;
      if (!entry.dir) return null;
      entry = entry.children.get(name);
      if (!entry) return null;
    }
    return entry;
  }

  /** split returns the entry of the directory of path, or null, and the last name of path. */
  split(path) {
    const slash = path.lastIndexOf("/");
    return [this.lookup(path.slice(0, slash) || "/"), path.slice(slash + 1)];
  }

  /** mkdirAll makes the directory path and the directories above it, and returns its entry. */
  mkdirAll(path, mtime) {
    let entry = this.root;
    for (const name of path.split("/")) {
      if (!name) continue;
      let child = entry.children.get(name);
      if (!child) {
        child = this.newEntry(true, mtime);
        entry.children.set(name, child);
      }
      if (!child.dir) throw errnoError(ERRNO.ENOTDIR);
      entry = child;
    }
    return entry;
  }

  /** writeFile sets the content of the file path to data, with its directories, and returns its entry. */
  writeFile(path, data, mtime) {
    const slash = path.lastIndexOf("/");
    const dir = this.mkdirAll(path.slice(0, slash) || "/", mtime);
    const name = path.slice(slash + 1);
    const entry = dir.children.get(name) || this.newEntry(false, mtime);
    entry.data = data;
    entry.size = data.length;
    entry.mtime = mtime || Date.now();
    dir.children.set(name, entry);
    return entry;
  }

  /** readFile returns the content of the file path. */
  readFile(path) {
    const entry = this.lookup(path);
    if (!entry || entry.dir) throw errnoError(ERRNO.ENOENT);
    return entry.data.subarray(0, entry.size);
  }

  /** walk calls visit(path, entry) for the entry at path and for each entry under it, a directory before its children. */
  walk(path, visit, entry = this.lookup(path)) {
    visit(path, entry);
    if (!entry.dir) return;
    for (const [name, child] of entry.children) this.walk(path + "/" + name, visit, child);
  }
}

/** reserve grows the data of the file entry to hold at least size bytes. */
function reserve(entry, size) {
  if (entry.data.length >= size) return;
  const data = new Uint8Array(Math.max(size, entry.data.length * 2, 4096));
  data.set(entry.data.subarray(0, entry.size));
  entry.data = data;
}

/** Pipe holds the bytes that the write end of a pipe gave and the read end did not take yet. */
class Pipe {
  constructor() {
    this.chunks = [];
    this.writers = 0;
    this.readers = 0;
    /** The reads that wait for data: functions of the bytes, with the wanted length in want. */
    this.waitingReads = [];
  }

  /** take removes up to length bytes from the start of the pipe and returns them. */
  take(length) {
    const parts = [];
    let taken = 0;
    while (taken < length && this.chunks.length) {
      const chunk = this.chunks[0];
      const count = Math.min(chunk.length, length - taken);
      parts.push(chunk.subarray(0, count));
      taken += count;
      if (count === chunk.length) this.chunks.shift();
      else this.chunks[0] = chunk.subarray(count);
    }
    if (parts.length === 1) return parts[0];
    const bytes = new Uint8Array(taken);
    let offset = 0;
    for (const part of parts) {
      bytes.set(part, offset);
      offset += part.length;
    }
    return bytes;
  }

  /** wake completes the waiting reads while there is data, or with no data when no writer is left. */
  wake() {
    while (this.waitingReads.length && (this.chunks.length || this.writers === 0)) {
      const read = this.waitingReads.shift();
      read(this.take(read.want));
    }
  }
}

/**
 * A descriptor is an open file of a process. Several descriptor numbers of several processes can
 * share one, and references counts them. Its kind is one of these.
 */
const DESCRIPTOR = Object.freeze({
  FILE: "file",
  NULL: "null",
  /** A callback of the page that gets the bytes of each write. */
  SINK: "sink",
  PIPE_READ: "pipe-read",
  PIPE_WRITE: "pipe-write",
});

function newDescriptor(kind, fields = {}) {
  return { kind, references: 0, ...fields };
}

/**
 * Kernel runs processes on a MemFS. makeWorker() returns {post(message), onmessage, terminate()} for a
 * new process Worker. loadModule(path, entry) returns a Promise of the WebAssembly.Module of a program.
 * toolEnv, when set, maps a tool name to more environment variables, and onSpawn(process), when set,
 * is called for each new process.
 */
export class Kernel {
  /**
   * limitMs is the hard limit of the run time of any process, in milliseconds; 0 is no limit.
   * A process with a -test.timeout=D argument exits as a timed-out test at D.
   */
  constructor({ fs = new MemFS(), loadModule, makeWorker, limitMs = 0 }) {
    this.fs = fs;
    this.loadModule = loadModule;
    this.makeWorker = makeWorker;
    this.limitMs = limitMs;
    this.processes = new Map();
    this.nextPid = FIRST_PID;
    this.stats = { spawns: 0, maxMem: {}, live: 0, maxLive: 0 };
    /** The result of gorootImage, or undefined before its first call. */
    this.goroot = undefined;
  }

  newProcess(parent, cwd, descriptors) {
    const process = {
      pid: this.nextPid++,
      ppid: parent ? parent.pid : NO_PARENT,
      cwd,
      descriptors: new Map(),
      nextDescriptor: FIRST_FREE_DESCRIPTOR,
      exited: false,
      waiters: [],
    };
    descriptors.forEach((descriptor, fd) => {
      if (descriptor) this.share(process, fd, descriptor);
    });
    this.processes.set(process.pid, process);
    return process;
  }

  /** share gives the descriptor the number fd in process. */
  share(process, fd, descriptor) {
    descriptor.references++;
    if (descriptor.references === 1 && descriptor.kind === DESCRIPTOR.PIPE_WRITE) descriptor.pipe.writers++;
    if (descriptor.references === 1 && descriptor.kind === DESCRIPTOR.PIPE_READ) descriptor.pipe.readers++;
    process.descriptors.set(fd, descriptor);
  }

  /** addDescriptor gives the descriptor the next free number in process and returns the number. */
  addDescriptor(process, descriptor) {
    const fd = process.nextDescriptor++;
    this.share(process, fd, descriptor);
    return fd;
  }

  release(descriptor) {
    if (--descriptor.references > 0) return;
    if (descriptor.kind === DESCRIPTOR.PIPE_WRITE) {
      descriptor.pipe.writers--;
      descriptor.pipe.wake();
    }
    if (descriptor.kind === DESCRIPTOR.PIPE_READ) descriptor.pipe.readers--;
  }

  closeAll(process) {
    for (const descriptor of process.descriptors.values()) this.release(descriptor);
    process.descriptors.clear();
  }

  stat(entry) {
    const type = entry.isCharDevice ? S_IFCHR : entry.dir ? S_IFDIR : S_IFREG;
    return { ino: entry.ino, mode: type | entry.mode, size: entry.dir ? 4096 : entry.size, mtimeMs: entry.mtime };
  }

  /** write writes bytes to the descriptor, at position or at its own position when position is null. */
  write(descriptor, bytes, position) {
    switch (descriptor.kind) {
      case DESCRIPTOR.SINK:
        descriptor.sink(bytes.slice());
        return bytes.length;
      case DESCRIPTOR.NULL:
        return bytes.length;
      case DESCRIPTOR.PIPE_WRITE:
        if (descriptor.pipe.readers === 0) throw errnoError(ERRNO.EPIPE);
        if (bytes.length === 0) return 0;
        descriptor.pipe.chunks.push(bytes.slice());
        descriptor.pipe.wake();
        return bytes.length;
      case DESCRIPTOR.FILE: {
        const entry = descriptor.entry;
        let start = position;
        if (start == null) start = descriptor.append ? entry.size : descriptor.position;
        const end = start + bytes.length;
        reserve(entry, end);
        entry.data.set(bytes, start);
        entry.size = Math.max(entry.size, end);
        entry.mtime = Date.now();
        if (position == null) descriptor.position = end;
        return bytes.length;
      }
      default:
        throw errnoError(ERRNO.EBADF);
    }
  }

  /** writeStderr writes text to the standard error of process, when it has one. */
  writeStderr(process, text) {
    const descriptor = process.descriptors.get(STDERR);
    if (!descriptor) return;
    try {
      this.write(descriptor, textEncoder.encode(text));
    } catch {
      // A pipe with no reader fails with EPIPE, and the text is lost.
    }
  }

  /**
   * gorootImage returns the read-only image of /goroot that the Workers share, or null with no
   * /goroot: {buffer, index}. buffer is a SharedArrayBuffer with the content of each file, and index
   * is a list of [path, {offset, size, mtimeMs, ino, mode, names}], where names lists the children of
   * a directory and is null for a file.
   */
  gorootImage() {
    if (this.goroot !== undefined) return this.goroot;
    if (!this.fs.lookup("/goroot")) {
      this.goroot = null;
      return null;
    }
    let size = 0;
    this.fs.walk("/goroot", (path, entry) => {
      if (!entry.dir) size += entry.size;
    });
    const buffer = new SharedArrayBuffer(size);
    const bytes = new Uint8Array(buffer);
    const index = [];
    let offset = 0;
    this.fs.walk("/goroot", (path, entry) => {
      const stats = this.stat(entry);
      if (entry.dir) {
        const names = [...entry.children.keys()];
        index.push([path, { offset: 0, size: stats.size, mtimeMs: stats.mtimeMs, ino: stats.ino, mode: stats.mode, names }]);
        return;
      }
      bytes.set(entry.data.subarray(0, entry.size), offset);
      index.push([path, { offset, size: entry.size, mtimeMs: entry.mtime, ino: entry.ino, mode: stats.mode, names: null }]);
      offset += entry.size;
    });
    this.goroot = { buffer, index };
    return this.goroot;
  }

  /** writesGoroot reports whether the call name with args would change a file under /goroot. */
  writesGoroot(process, name, args) {
    if (!this.goroot || !PATH_WRITE_CALLS.has(name)) return false;
    const inGoroot = path => {
      if (typeof path !== "string") return false;
      const absolute = normalizePath(path, process.cwd);
      return absolute === "/goroot" || absolute.startsWith("/goroot/");
    };
    if (name === "open") return inGoroot(args[0]) && Boolean(args[1] & WRITE_FLAGS);
    return inGoroot(args[0]) || ((name === "rename" || name === "link") && inGoroot(args[1]));
  }

  /**
   * call runs the call name of process with args and data, the bytes of a write. It returns
   * {result, data}, or PENDING, and then complete(result, data) runs later. It throws an errno Error.
   */
  call(process, name, args, data, complete) {
    if (this.writesGoroot(process, name, args)) throw errnoError(ERRNO.EROFS);
    const fs = this.fs;
    const absolute = path => normalizePath(path, process.cwd);
    const descriptorOf = fd => {
      const descriptor = process.descriptors.get(fd);
      if (!descriptor) throw errnoError(ERRNO.EBADF);
      return descriptor;
    };
    const entryOf = path => {
      const entry = fs.lookup(absolute(path));
      if (!entry) throw errnoError(ERRNO.ENOENT);
      return entry;
    };
    switch (name) {
      case "writeSync":
      case "write": {
        const [fd, position] = args;
        if (process.runningTests && fd === STDOUT) this.scanTests(process, data);
        return { result: this.write(descriptorOf(fd), data, position) };
      }
      case "read":
        return this.read(descriptorOf(args[0]), args[1], args[2], complete);
      case "open":
        return { result: this.open(process, absolute(args[0]), args[1]) };
      case "close": {
        const descriptor = descriptorOf(args[0]);
        process.descriptors.delete(args[0]);
        this.release(descriptor);
        return { result: null };
      }
      case "pipe": {
        const pipe = new Pipe();
        const readEnd = this.addDescriptor(process, newDescriptor(DESCRIPTOR.PIPE_READ, { pipe }));
        const writeEnd = this.addDescriptor(process, newDescriptor(DESCRIPTOR.PIPE_WRITE, { pipe }));
        return { result: [readEnd, writeEnd] };
      }
      case "fstat": {
        const descriptor = descriptorOf(args[0]);
        if (descriptor.kind === DESCRIPTOR.FILE) return { result: this.stat(descriptor.entry) };
        if (descriptor.kind === DESCRIPTOR.NULL) return { result: this.stat(fs.devNull) };
        return { result: { ino: 0, mode: S_IFIFO | 0o600, size: 0, mtimeMs: 0 } };
      }
      case "stat":
      case "lstat":
        return { result: this.stat(entryOf(args[0])) };
      case "mkdir": {
        const path = absolute(args[0]);
        if (fs.lookup(path)) throw errnoError(ERRNO.EEXIST);
        const [dir, newName] = fs.split(path);
        if (!dir) throw errnoError(ERRNO.ENOENT);
        if (!dir.dir) throw errnoError(ERRNO.ENOTDIR);
        dir.children.set(newName, fs.newEntry(true));
        return { result: null };
      }
      case "readdir": {
        const entry = entryOf(args[0]);
        if (!entry.dir) throw errnoError(ERRNO.ENOTDIR);
        return { result: [...entry.children.keys()] };
      }
      case "unlink":
      case "rmdir": {
        const [dir, entryName] = fs.split(absolute(args[0]));
        const entry = dir && dir.dir && dir.children.get(entryName);
        if (!entry) throw errnoError(ERRNO.ENOENT);
        if (name === "unlink" && entry.dir) throw errnoError(ERRNO.EISDIR);
        if (name === "rmdir" && !entry.dir) throw errnoError(ERRNO.ENOTDIR);
        if (name === "rmdir" && entry.children.size) throw errnoError(ERRNO.ENOTEMPTY);
        dir.children.delete(entryName);
        return { result: null };
      }
      case "rename": {
        const [fromDir, fromName] = fs.split(absolute(args[0]));
        const entry = fromDir && fromDir.dir && fromDir.children.get(fromName);
        if (!entry) throw errnoError(ERRNO.ENOENT);
        const [toDir, toName] = fs.split(absolute(args[1]));
        if (!toDir || !toDir.dir) throw errnoError(ERRNO.ENOENT);
        const replaced = toDir.children.get(toName);
        if (replaced && replaced.dir && replaced.children.size) throw errnoError(ERRNO.ENOTEMPTY);
        fromDir.children.delete(fromName);
        toDir.children.set(toName, entry);
        return { result: null };
      }
      case "fsync":
      case "chown":
      case "fchown":
      case "lchown":
        return { result: null };
      case "ftruncate":
      case "truncate": {
        const entry = name === "ftruncate" ? descriptorOf(args[0]).entry : entryOf(args[0]);
        const size = args[1];
        if (!entry) throw errnoError(ERRNO.EINVAL);
        reserve(entry, size);
        if (size > entry.size) entry.data.fill(0, entry.size, size);
        entry.size = size;
        return { result: null };
      }
      case "chmod":
        entryOf(args[0]).mode = args[1] & 0o7777;
        return { result: null };
      case "fchmod": {
        const descriptor = descriptorOf(args[0]);
        if (descriptor.entry) descriptor.entry.mode = args[1] & 0o7777;
        return { result: null };
      }
      case "utimes":
        entryOf(args[0]).mtime = args[2] * 1000;
        return { result: null };
      case "readlink":
        throw errnoError(ERRNO.EINVAL);
      case "symlink":
        throw errnoError(ERRNO.ENOSYS);
      case "link": {
        const entry = entryOf(args[0]);
        if (entry.dir) throw errnoError(ERRNO.EPERM);
        const [dir, linkName] = fs.split(absolute(args[1]));
        if (!dir) throw errnoError(ERRNO.ENOENT);
        if (dir.children.has(linkName)) throw errnoError(ERRNO.EEXIST);
        dir.children.set(linkName, entry);
        return { result: null };
      }
      case "chdir": {
        const path = absolute(args[0]);
        const entry = fs.lookup(path);
        if (!entry) throw errnoError(ERRNO.ENOENT);
        if (!entry.dir) throw errnoError(ERRNO.ENOTDIR);
        process.cwd = path;
        return { result: path };
      }
      case "spawn": {
        const [argv0, argv, env, dir, fds] = args;
        try {
          const descriptors = fds.map(fd => (fd < 0 ? null : descriptorOf(fd)));
          const child = this.spawn(process, argv0, argv, env, dir ? absolute(dir) : process.cwd, descriptors);
          return { result: child.pid };
        } catch (error) {
          // goSpawn of the overlay returns the error as an object with a code, not as an Error.
          return { result: { code: error.code || ERRNO.ENOSYS, message: String(error.message) } };
        }
      }
      case "kill":
        return { result: this.kill(args[0], args[1]) };
      case "wait": {
        const child = this.processes.get(args[0]);
        if (!child) return { result: null };
        if (child.exited) {
          this.processes.delete(child.pid);
          return { result: child.code };
        }
        child.waiters.push(code => complete(code));
        return PENDING;
      }
      default:
        throw errnoError(ERRNO.ENOSYS, "kernel: no call " + name);
    }
  }

  /** read runs a read call of length bytes at position, or at the position of the descriptor when position is null. */
  read(descriptor, length, position, complete) {
    if (descriptor.kind === DESCRIPTOR.NULL || descriptor.kind === DESCRIPTOR.SINK) return { result: 0 };
    if (descriptor.kind === DESCRIPTOR.PIPE_READ) {
      const pipe = descriptor.pipe;
      if (pipe.chunks.length) {
        const bytes = pipe.take(length);
        return { result: bytes.length, data: bytes };
      }
      if (pipe.writers === 0) return { result: 0 };
      const waitingRead = bytes => complete(bytes.length, bytes);
      waitingRead.want = length;
      pipe.waitingReads.push(waitingRead);
      return PENDING;
    }
    if (descriptor.kind !== DESCRIPTOR.FILE) throw errnoError(ERRNO.EBADF);
    const entry = descriptor.entry;
    if (entry.dir) throw errnoError(ERRNO.EISDIR);
    const start = position != null ? position : descriptor.position;
    const count = Math.max(0, Math.min(length, entry.size - start, CALL_BUFFER.DATA_CAPACITY));
    if (position == null) descriptor.position += count;
    return { result: count, data: entry.data.subarray(start, start + count) };
  }

  /** open opens the absolute path with the open flags in process and returns the descriptor number. */
  open(process, path, flags) {
    if (path === "/dev/null") return this.addDescriptor(process, newDescriptor(DESCRIPTOR.NULL));
    let entry = this.fs.lookup(path);
    if (entry && flags & OPEN_FLAGS.O_CREAT && flags & OPEN_FLAGS.O_EXCL) throw errnoError(ERRNO.EEXIST);
    if (!entry) {
      if (!(flags & OPEN_FLAGS.O_CREAT)) throw errnoError(ERRNO.ENOENT);
      const [dir, name] = this.fs.split(path);
      if (!dir || !dir.dir) throw errnoError(ERRNO.ENOENT);
      entry = this.fs.newEntry(false);
      dir.children.set(name, entry);
    }
    if (entry.dir && flags & O_ACCMODE) throw errnoError(ERRNO.EISDIR);
    if (flags & OPEN_FLAGS.O_DIRECTORY && !entry.dir) throw errnoError(ERRNO.ENOTDIR);
    if (flags & OPEN_FLAGS.O_TRUNC) {
      entry.size = 0;
      entry.mtime = Date.now();
    }
    const append = Boolean(flags & OPEN_FLAGS.O_APPEND);
    return this.addDescriptor(process, newDescriptor(DESCRIPTOR.FILE, { entry, position: 0, append }));
  }

  /** kill kills process pid with a js/wasm signal number: it terminates the Worker at once. The wait status is KILLED + signal. */
  kill(pid, signal = SIG.KILL) {
    const process = this.processes.get(pid);
    if (!process || process.exited) return false;
    if (!SIGNAL_NAMES[signal]) signal = SIG.KILL;
    this.finish(process, KILLED + signal, null);
    return true;
  }

  /** killTree kills pid and each of its descendants, the children first. It returns the number of processes killed. */
  killTree(pid, signal = SIG.KILL) {
    let killed = 0;
    for (const process of [...this.processes.values()]) {
      if (process.ppid === pid && !process.exited) killed += this.killTree(process.pid, signal);
    }
    return killed + (this.kill(pid, signal) ? 1 : 0);
  }

  /** scanTests records the === RUN and --- PASS/FAIL/SKIP lines that the test binary process writes to standard output. */
  scanTests(process, bytes) {
    const text = process.testOutputTail + new TextDecoder().decode(bytes);
    const lines = text.split("\n");
    process.testOutputTail = lines.pop().slice(-512);
    for (const line of lines) {
      const started = /^=== (RUN|CONT)\s+(\S+)/.exec(line);
      if (started) {
        if (!process.runningTests.has(started[2])) process.runningTests.set(started[2], performance.now());
        continue;
      }
      const ended = /^\s*--- (PASS|FAIL|SKIP): (\S+)/.exec(line);
      if (ended) process.runningTests.delete(ended[2]);
    }
  }

  /**
   * testTimedOut ends the test binary process as package testing does when -test.timeout passes: the
   * panic text on standard error and exit status 2.
   */
  testTimedOut(process, timeout) {
    if (process.exited) return;
    let text = `panic: test timed out after ${timeout}`;
    if (process.runningTests.size) {
      const now = performance.now();
      const running = [...process.runningTests].map(([name, startedAt]) => `${name} (${Math.round((now - startedAt) / 1000)}s)`);
      text += "\n\trunning tests:" + running.sort().map(test => "\n\t\t" + test).join("");
    }
    text += `\n\n[kernel: pid ${process.pid} terminated at the -test.timeout deadline; no goroutine dump is available]\n`;
    this.writeStderr(process, text);
    this.finish(process, 2, null, "timeout");
  }

  /** overLimit kills process when it runs longer than limitMs. */
  overLimit(process) {
    if (process.exited) return;
    const limit = this.limitMs / 1000;
    this.writeStderr(process, `kernel: pid ${process.pid} (${process.tool}) killed: it ran past the process limit of ${limit}s\n`);
    this.finish(process, KILLED + SIG.KILL, null, "limit");
  }

  /** finish ends process with the wait status code. memoryBytes is the size of its memory, when known. */
  finish(process, code, memoryBytes, reason = "") {
    if (process.exited) return;
    process.exited = true;
    process.code = code;
    process.reason = reason;
    for (const timer of process.timers || []) clearTimeout(timer);
    if (process.worker) {
      process.worker.terminate();
      process.worker = null;
    }
    this.stats.live--;
    if (memoryBytes) this.stats.maxMem[process.tool] = Math.max(this.stats.maxMem[process.tool] || 0, memoryBytes);
    this.closeAll(process);
    // An exited process stays in the table as a zombie until a wait reaps it, as on Unix, so a wait
    // that comes after the exit still gets the status.
    const reaped = process.waiters.length > 0 || process.ppid === NO_PARENT;
    for (const waiter of process.waiters) waiter(code);
    process.waiters = [];
    process.resolve(code);
    if (reaped) this.processes.delete(process.pid);
  }

  /**
   * spawn starts a process in a new Worker and returns it. argv0 is a path in the memfs, and env is a
   * list of "name=value". process.done resolves with the wait status.
   */
  spawn(parent, argv0, argv, env, cwd, descriptors) {
    const path = normalizePath(argv0, parent ? parent.cwd : cwd);
    const entry = this.fs.lookup(path);
    if (!entry || entry.dir) throw errnoError(ERRNO.ENOENT);
    const process = this.newProcess(parent, cwd, descriptors);
    process.tool = path.split("/").pop();
    process.timers = [];
    const timeout = testTimeoutOf(argv);
    const timeoutMs = timeout ? parseDuration(timeout) : 0;
    if (timeoutMs > 0) {
      process.runningTests = new Map();
      process.testOutputTail = "";
      process.timers.push(setTimeout(() => this.testTimedOut(process, timeout), timeoutMs));
    }
    if (this.limitMs > 0) process.timers.push(setTimeout(() => this.overLimit(process), this.limitMs));
    if (this.onSpawn) this.onSpawn(process);
    process.done = new Promise(resolve => (process.resolve = resolve));
    this.stats.spawns++;
    this.stats.live++;
    this.stats.maxLive = Math.max(this.stats.maxLive, this.stats.live);
    const args = this.responseFileArgs(process, argv, env);
    this.startWorker(process, path, entry, [argv[0] || path, ...args], env, cwd);
    return process;
  }

  /**
   * responseFileArgs returns the arguments of process after argv[0]. For a tool that reads a response
   * file and has long arguments, it writes them to a file and returns ["@file"], as cmd/go does when
   * the command line is too long.
   */
  responseFileArgs(process, argv, env) {
    const args = argv.slice(1);
    const size = [...argv, ...env].reduce((total, text) => total + textEncoder.encode(text).length + 16, 0);
    if (size <= RESPONSE_FILE_MIN_SIZE || !RESPONSE_FILE_TOOLS.has(process.tool)) return args;
    const responseFile = `/tmp/rsp-${process.pid}.txt`;
    const quoted = args.map(arg => "'" + arg.replace(/'/g, "'\\''") + "'");
    this.fs.writeFile(responseFile, textEncoder.encode(quoted.join("\n")));
    return ["@" + responseFile];
  }

  /** startWorker loads the program at path, starts a Worker for process and answers its calls. */
  async startWorker(process, path, entry, argv, env, cwd) {
    let module;
    try {
      module = await this.loadModule(path, entry);
    } catch (error) {
      this.writeStderr(process, `kernel: ${path}: ${(error && error.stack) || error}\n`);
      return this.finish(process, 2);
    }
    if (process.exited) return;
    const buffer = new SharedArrayBuffer(CALL_BUFFER.SIZE);
    const state = new Int32Array(buffer, 0, 4);
    const bytes = new Uint8Array(buffer);
    const worker = this.makeWorker();
    process.worker = worker;
    const reply = (message, data) => {
      const json = textEncoder.encode(JSON.stringify(message));
      if (json.length > CALL_BUFFER.JSON_CAPACITY) throw new Error("kernel: reply too large");
      bytes.set(json, CALL_BUFFER.JSON_OFFSET);
      state[CALL_BUFFER.JSON_LENGTH] = json.length;
      if (data) {
        bytes.set(data, CALL_BUFFER.DATA_OFFSET);
        state[CALL_BUFFER.DATA_LENGTH] = data.length;
      } else {
        state[CALL_BUFFER.DATA_LENGTH] = -1;
      }
      Atomics.store(state, CALL_BUFFER.STATE, 1);
      Atomics.notify(state, CALL_BUFFER.STATE);
    };
    worker.onmessage = message => {
      if (process.exited) return;
      if (message.type === MESSAGE.CALL) {
        const complete = (result, data) => {
          if (process.exited || !process.worker) return;
          process.worker.post({ type: MESSAGE.COMPLETE, id: message.id, result, data: data && data.slice() });
        };
        let outcome;
        try {
          outcome = this.call(process, message.name, message.args, message.data, complete);
        } catch (error) {
          reply({ errno: error.code || ERRNO.EIO, message: String(error.message) });
          return;
        }
        if (outcome === PENDING) reply({ pendingId: message.id });
        else reply({ result: outcome.result }, outcome.data);
      } else if (message.type === MESSAGE.EXIT) {
        this.finish(process, message.code, message.memoryBytes);
      } else if (message.type === MESSAGE.CRASH) {
        this.writeStderr(process, `kernel: ${path}: ${message.text}\n`);
        this.finish(process, 2, message.memoryBytes);
      }
    };
    const envObject = {};
    for (const variable of env) {
      const equals = variable.indexOf("=");
      if (equals > 0) envObject[variable.slice(0, equals)] = variable.slice(equals + 1);
    }
    if (this.toolEnv && this.toolEnv[process.tool]) Object.assign(envObject, this.toolEnv[process.tool]);
    worker.post({
      type: MESSAGE.START,
      goroot: this.gorootImage(),
      module,
      buffer,
      pid: process.pid,
      ppid: process.ppid,
      cwd,
      argv,
      env: envObject,
    });
  }

  /**
   * run starts a command of the page, with standard output and error sent to the callbacks onOut and
   * onErr, and env an object of variables. It returns the process; process.done resolves with the
   * wait status.
   */
  run(argv0, argv, env, cwd, onOut, onErr) {
    const descriptors = [
      newDescriptor(DESCRIPTOR.NULL),
      newDescriptor(DESCRIPTOR.SINK, { sink: onOut }),
      newDescriptor(DESCRIPTOR.SINK, { sink: onErr }),
    ];
    const envList = Object.entries(env).map(([name, value]) => `${name}=${value}`);
    return this.spawn(null, argv0, argv, envList, cwd, descriptors);
  }
}
