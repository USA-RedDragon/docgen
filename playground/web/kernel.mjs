// Kernel for Go js/wasm programs, one Web Worker per process.
// The kernel runs on the main thread and holds the memfs, the fd tables, the pipes and the process table.
// A process calls the kernel with a synchronous RPC: it posts a request and waits with Atomics.wait on a
// SharedArrayBuffer that the kernel fills with the reply. A call that must block (a pipe read with no data,
// a wait for a child) gets a "pending" reply and completes later with a message.
// The Worker of a process is terminated when the process exits or is killed.
import {
  S_IFREG, S_IFDIR, S_IFIFO, S_IFCHR, OPEN_FLAGS as C, O_ACCMODE, WRITE_FLAGS, errnoError as err, CALL_BUFFER,
  MESSAGE, SIG, SIGNAL_NAMES as SIGNAME, KILLED,
} from "./abi.mjs";
const enc = new TextEncoder();
const RSP_TOOLS = new Set(["compile", "link", "asm", "cgo", "cover", "pack"]);
const PENDING = Symbol("pending");

/** parseDuration parses a Go duration such as "2s", "1m30s" or "500ms" to milliseconds. It returns NaN for bad text. */
export function parseDuration(t) {
  const u = { ns: 1e-6, us: 1e-3, "µs": 1e-3, ms: 1, s: 1000, m: 60000, h: 3600000 };
  if (t === "0") return 0;
  const re = /([0-9]*\.?[0-9]+)(ns|us|µs|ms|s|m|h)/gy;
  let ms = 0, m, n = 0;
  while ((m = re.exec(t))) { ms += parseFloat(m[1]) * u[m[2]]; n = re.lastIndex; }
  return n === t.length && n > 0 ? ms : NaN;
}
const WRITE_OPS = new Set(["open", "mkdir", "unlink", "rmdir", "rename", "truncate", "chmod", "utimes", "link", "symlink"]);

export function normPath(p, cwd = "/") {
  if (!p.startsWith("/")) p = cwd + "/" + p;
  const out = [];
  for (const s of p.split("/")) { if (!s || s === ".") continue; if (s === "..") out.pop(); else out.push(s); }
  return "/" + out.join("/");
}

export class MemFS {
  constructor() { this.ino = 1; this.root = this.mkent(true); this.dev = this.mkent(false); this.dev.chr = true; }
  mkent(dir, mtime = Date.now()) {
    return { dir, children: dir ? new Map() : null, data: dir ? null : new Uint8Array(0), size: 0, ino: this.ino++, mtime, mode: dir ? 0o755 : 0o644 };
  }
  lookup(p) {
    if (p === "/dev/null") return this.dev;
    let e = this.root;
    for (const s of p.split("/")) { if (!s) continue; if (!e.dir) return null; e = e.children.get(s); if (!e) return null; }
    return e;
  }
  split(p) { const i = p.lastIndexOf("/"); return [this.lookup(p.slice(0, i) || "/"), p.slice(i + 1)]; }
  mkdirAll(p, mtime) {
    let e = this.root;
    for (const s of p.split("/")) {
      if (!s) continue;
      let c = e.children.get(s);
      if (!c) { c = this.mkent(true, mtime); e.children.set(s, c); }
      if (!c.dir) throw err("ENOTDIR");
      e = c;
    }
    return e;
  }
  writeFile(p, data, mtime) {
    const i = p.lastIndexOf("/");
    const d = this.mkdirAll(p.slice(0, i) || "/", mtime);
    const name = p.slice(i + 1);
    const e = d.children.get(name) || this.mkent(false, mtime);
    e.data = data; e.size = data.length; e.mtime = mtime || Date.now();
    d.children.set(name, e);
    return e;
  }
  readFile(p) { const e = this.lookup(p); if (!e || e.dir) throw err("ENOENT"); return e.data.subarray(0, e.size); }
  stats() {
    let files = 0, bytes = 0;
    const walk = e => { for (const c of e.children.values()) { if (c.dir) walk(c); else { files++; bytes += c.size; } } };
    walk(this.root);
    return { files, bytes };
  }
}

function ensure(e, n) {
  if (e.data.length >= n) return;
  const d = new Uint8Array(Math.max(n, e.data.length * 2, 4096)); d.set(e.data.subarray(0, e.size)); e.data = d;
}

class Pipe {
  constructor() { this.chunks = []; this.writers = 0; this.readers = 0; this.waiters = []; }
  take(length) {
    const parts = []; let n = 0;
    while (n < length && this.chunks.length) {
      const c = this.chunks[0], k = Math.min(c.length, length - n);
      parts.push(c.subarray(0, k)); n += k;
      if (k === c.length) this.chunks.shift(); else this.chunks[0] = c.subarray(k);
    }
    if (parts.length === 1) return parts[0];
    const out = new Uint8Array(n); let o = 0;
    for (const p of parts) { out.set(p, o); o += p.length; }
    return out;
  }
  wake() {
    while (this.waiters.length && (this.chunks.length || this.writers === 0)) {
      const w = this.waiters.shift();
      w(this.take(w.want));
    }
  }
}

/**
 * makeWorker() returns {post(msg), onmessage, terminate()} for a new process Worker.
 * loadModule(path, entry) returns a Promise of a WebAssembly.Module.
 */
export class Kernel {
  /**
   * limitMs is the hard limit of the run time of any process, in milliseconds; 0 is no limit.
   * testTimeouts makes a process with a -test.timeout=D argument exit as a timed-out test at D.
   */
  constructor({ fs = new MemFS(), loadModule, makeWorker, log = () => {}, limitMs = 0, testTimeouts = true }) {
    this.fs = fs; this.loadModule = loadModule; this.makeWorker = makeWorker; this.log = log;
    this.limitMs = limitMs; this.testTimeouts = testTimeouts;
    this.procs = new Map(); this.nextPid = 100;
    this.stats = { spawns: 0, rpcs: 0, rpcMs: 0, maxMem: {}, byTool: {}, live: 0, maxLive: 0, ops: {} };
  }

  newProc(parent, cwd, descs) {
    const p = { pid: this.nextPid++, ppid: parent ? parent.pid : 1, cwd, fds: new Map(), nextFd: 3, exited: false, waiters: [] };
    descs.forEach((d, i) => { if (d) { d.refs++; this.onOpen(d); p.fds.set(i, d); } });
    this.procs.set(p.pid, p);
    return p;
  }
  onOpen(d) { if (d.type === "pw" && d.refs === 1) d.pipe.writers++; if (d.type === "pr" && d.refs === 1) d.pipe.readers++; }
  release(d) {
    if (--d.refs > 0) return;
    if (d.type === "pw") { d.pipe.writers--; d.pipe.wake(); }
    if (d.type === "pr") d.pipe.readers--;
  }
  addFd(p, d) { const fd = p.nextFd++; d.refs++; this.onOpen(d); p.fds.set(fd, d); return fd; }
  closeAll(p) { for (const d of p.fds.values()) this.release(d); p.fds.clear(); }
  sink(fn) { return { type: "sink", fn, refs: 0 }; }

  stat(e) {
    return { ino: e.ino, mode: (e.chr ? S_IFCHR : e.dir ? S_IFDIR : S_IFREG) | e.mode, size: e.dir ? 4096 : e.size, mtimeMs: e.mtime };
  }

  writeDesc(d, b, position) {
    switch (d.type) {
      case "sink": d.fn(b.slice()); return b.length;
      case "null": return b.length;
      case "pw":
        if (d.pipe.readers === 0) throw err("EPIPE");
        if (b.length === 0) return 0;
        d.pipe.chunks.push(b.slice()); d.pipe.wake(); return b.length;
      case "file": {
        const e = d.e, pos = position != null ? position : d.append ? e.size : d.pos;
        ensure(e, pos + b.length); e.data.set(b, pos); e.size = Math.max(e.size, pos + b.length); e.mtime = Date.now();
        if (position == null) d.pos = pos + b.length;
        return b.length;
      }
      default: throw err("EBADF");
    }
  }

  /** Returns the shared read-only image of /goroot: {buffer, index: [[path, {offset, size, mtimeMs, ino, mode, names}]]}; names is null for a file. */
  roImage() {
    if (this.ro !== undefined) return this.ro;
    const root = this.fs.lookup("/goroot");
    if (!root || this.noRO) return (this.ro = null);
    const index = []; let size = 0;
    const walk = (e, p) => { if (!e.dir) size += e.size; else for (const [n, c] of e.children) walk(c, p + "/" + n); };
    walk(root, "/goroot");
    const sab = new SharedArrayBuffer(size), u8 = new Uint8Array(sab);
    let off = 0;
    const add = (e, p) => {
      const st = this.stat(e);
      if (e.dir) { index.push([p, { offset: 0, size: st.size, mtimeMs: st.mtimeMs, ino: st.ino, mode: st.mode, names: [...e.children.keys()] }]); for (const [n, c] of e.children) add(c, p + "/" + n); }
      else { u8.set(e.data.subarray(0, e.size), off); index.push([p, { offset: off, size: e.size, mtimeMs: e.mtime, ino: e.ino, mode: st.mode, names: null }]); off += e.size; }
    };
    add(root, "/goroot");
    return (this.ro = { buffer: sab, index });
  }

  /** Runs one RPC of process p. Returns {res, data} or PENDING (then done(res, data) is called later). */
  op(p, name, a, din, done) {
    const M = this.fs, R = q => normPath(q, p.cwd);
    const getfd = fd => { const d = p.fds.get(fd); if (!d) throw err("EBADF"); return d; };
    if (this.ro && WRITE_OPS.has(name)) {
      const ro = q => typeof q === "string" && (R(q) === "/goroot" || R(q).startsWith("/goroot/"));
      if (name === "open" ? ro(a[0]) && (a[1] & WRITE_FLAGS) : ro(a[0]) || ((name === "rename" || name === "link") && ro(a[1]))) throw err("EROFS");
    }
    const ent = q => { const e = M.lookup(R(q)); if (!e) throw err("ENOENT"); return e; };
    switch (name) {
      case "writeSync": case "write":
        if (p.tests && a[0] === 1) this.scanTests(p, din);
        return { res: this.writeDesc(getfd(a[0]), din, a[1]) };
      case "read": {
        const [fd, length, position] = a;
        const d = getfd(fd);
        if (d.type === "null" || d.type === "sink") return { res: 0 };
        if (d.type === "pr") {
          const pp = d.pipe;
          if (pp.chunks.length) { const b = pp.take(length); return { res: b.length, data: b }; }
          if (pp.writers === 0) return { res: 0 };
          const w = b => done(b.length, b); w.want = length;
          pp.waiters.push(w);
          return PENDING;
        }
        if (d.type !== "file") throw err("EBADF");
        if (d.e.dir) throw err("EISDIR");
        const pos = position != null ? position : d.pos;
        const n = Math.max(0, Math.min(length, d.e.size - pos, CALL_BUFFER.DATA_CAPACITY));
        if (position == null) d.pos += n;
        return { res: n, data: d.e.data.subarray(pos, pos + n) };
      }
      case "open": {
        const [q0, flags] = a, q = R(q0);
        if (q === "/dev/null") return { res: this.addFd(p, { type: "null", refs: 0 }) };
        let e = M.lookup(q);
        if (e && (flags & C.O_CREAT) && (flags & C.O_EXCL)) throw err("EEXIST");
        if (!e) {
          if (!(flags & C.O_CREAT)) throw err("ENOENT");
          const [dir, nm] = M.split(q);
          if (!dir || !dir.dir) throw err("ENOENT");
          e = M.mkent(false); dir.children.set(nm, e);
        }
        if (e.dir && (flags & O_ACCMODE)) throw err("EISDIR");
        if ((flags & C.O_DIRECTORY) && !e.dir) throw err("ENOTDIR");
        if (flags & C.O_TRUNC) { e.size = 0; e.mtime = Date.now(); }
        return { res: this.addFd(p, { type: "file", e, pos: 0, append: !!(flags & C.O_APPEND), refs: 0 }) };
      }
      case "close": { const d = getfd(a[0]); p.fds.delete(a[0]); this.release(d); return { res: null }; }
      case "pipe": {
        const pp = new Pipe();
        return { res: [this.addFd(p, { type: "pr", pipe: pp, refs: 0 }), this.addFd(p, { type: "pw", pipe: pp, refs: 0 })] };
      }
      case "fstat": {
        const d = getfd(a[0]);
        return { res: d.type === "file" ? this.stat(d.e) : d.type === "null" ? this.stat(M.dev) : { ino: 0, mode: S_IFIFO | 0o600, size: 0, mtimeMs: 0 } };
      }
      case "stat": case "lstat": return { res: this.stat(ent(a[0])) };
      case "mkdir": {
        const q = R(a[0]); if (M.lookup(q)) throw err("EEXIST");
        const [dir, nm] = M.split(q); if (!dir) throw err("ENOENT"); if (!dir.dir) throw err("ENOTDIR");
        dir.children.set(nm, M.mkent(true)); return { res: null };
      }
      case "readdir": { const e = ent(a[0]); if (!e.dir) throw err("ENOTDIR"); return { res: [...e.children.keys()] }; }
      case "unlink": case "rmdir": {
        const [dir, nm] = M.split(R(a[0])); const e = dir && dir.dir && dir.children.get(nm);
        if (!e) throw err("ENOENT");
        if (name === "unlink" && e.dir) throw err("EISDIR");
        if (name === "rmdir") { if (!e.dir) throw err("ENOTDIR"); if (e.children.size) throw err("ENOTEMPTY"); }
        dir.children.delete(nm); return { res: null };
      }
      case "rename": {
        const [da, na] = M.split(R(a[0])); const e = da && da.dir && da.children.get(na); if (!e) throw err("ENOENT");
        const [db, nb] = M.split(R(a[1])); if (!db || !db.dir) throw err("ENOENT");
        const t = db.children.get(nb); if (t && t.dir && t.children.size) throw err("ENOTEMPTY");
        da.children.delete(na); db.children.set(nb, e); return { res: null };
      }
      case "fsync": case "chown": case "fchown": case "lchown": return { res: null };
      case "ftruncate": case "truncate": {
        const e = name === "ftruncate" ? getfd(a[0]).e : ent(a[0]); if (!e) throw err("EINVAL");
        ensure(e, a[1]); if (a[1] > e.size) e.data.fill(0, e.size, a[1]); e.size = a[1]; return { res: null };
      }
      case "chmod": ent(a[0]).mode = a[1] & 0o7777; return { res: null };
      case "fchmod": { const d = getfd(a[0]); if (d.e) d.e.mode = a[1] & 0o7777; return { res: null }; }
      case "utimes": ent(a[0]).mtime = a[2] * 1000; return { res: null };
      case "readlink": throw err("EINVAL");
      case "symlink": throw err("ENOSYS");
      case "link": {
        const e = ent(a[0]); if (e.dir) throw err("EPERM");
        const [db, nb] = M.split(R(a[1])); if (!db) throw err("ENOENT"); if (db.children.has(nb)) throw err("EEXIST");
        db.children.set(nb, e); return { res: null };
      }
      case "chdir": {
        const q = R(a[0]); const e = M.lookup(q); if (!e) throw err("ENOENT"); if (!e.dir) throw err("ENOTDIR");
        p.cwd = q; return { res: q };
      }
      case "spawn": {
        const [argv0, argv, env, dir, fds] = a;
        try {
          const descs = fds.map(fd => fd < 0 ? null : getfd(fd));
          return { res: this.spawn(p, argv0, argv, env, dir ? R(dir) : p.cwd, descs).pid };
        } catch (e) { return { res: { code: e.code || "ENOSYS", message: String(e.message) } }; }
      }
      case "kill": return { res: this.kill(a[0], a[1]) };
      case "wait": {
        const c = this.procs.get(a[0]);
        if (!c) return { res: null };
        if (c.exited) { this.procs.delete(c.pid); return { res: c.code }; }
        c.waiters.push(code => done(code));
        return PENDING;
      }
      default: throw err("ENOSYS", "kernel: no op " + name);
    }
  }

  /** Kills process pid with a js signal number: terminates its Worker at once. The wait status is KILLED + sig. */
  kill(pid, sig = SIG.KILL) {
    const c = this.procs.get(pid);
    if (!c || c.exited) return false;
    if (!SIGNAME[sig]) sig = SIG.KILL;
    this.log(`[pid ${pid}] killed by signal ${SIGNAME[sig][0]}`);
    this.finish(c, KILLED + sig, null, sig);
    return true;
  }

  /** Kills pid and each of its descendants, the children first. Returns the number of processes killed. */
  killTree(pid, sig = SIG.KILL) {
    let n = 0;
    for (const c of [...this.procs.values()]) if (c.ppid === pid && !c.exited) n += this.killTree(c.pid, sig);
    return n + (this.kill(pid, sig) ? 1 : 0);
  }

  /** scanTests records the === RUN and --- PASS/FAIL/SKIP lines of the test binary p from its standard output. */
  scanTests(p, b) {
    const text = p.testTail + new TextDecoder().decode(b);
    const lines = text.split("\n");
    p.testTail = lines.pop().slice(-512);
    for (const l of lines) {
      let m = /^=== (RUN|CONT)\s+(\S+)/.exec(l);
      if (m) { if (!p.tests.has(m[2])) p.tests.set(m[2], performance.now()); continue; }
      m = /^\s*--- (PASS|FAIL|SKIP): (\S+)/.exec(l);
      if (m) p.tests.delete(m[2]);
    }
  }

  /** testTimedOut ends the test binary p as the testing package does when -test.timeout passes: the panic text on standard error and exit status 2. */
  testTimedOut(p, text) {
    if (p.exited) return;
    let msg = `panic: test timed out after ${text}`;
    if (p.tests.size) {
      const now = performance.now();
      const list = [...p.tests].map(([name, t]) => `${name} (${Math.round((now - t) / 1000)}s)`).sort();
      msg += "\n\trunning tests:" + list.map(x => "\n\t\t" + x).join("");
    }
    msg += `\n\n[kernel: pid ${p.pid} terminated at the -test.timeout deadline; no goroutine dump is available]\n`;
    try { this.writeDesc(p.fds.get(2), enc.encode(msg)); } catch {}
    this.log(`[pid ${p.pid}] ${p.tool} test timeout ${text}`);
    this.finish(p, 2, null, 0, "timeout");
  }

  /** overLimit kills p when it runs longer than limitMs. */
  overLimit(p) {
    if (p.exited) return;
    try { this.writeDesc(p.fds.get(2), enc.encode(`kernel: pid ${p.pid} (${p.tool}) killed: it ran past the process limit of ${this.limitMs / 1000}s\n`)); } catch {}
    this.finish(p, KILLED + SIG.KILL, null, SIG.KILL, "limit");
  }

  finish(p, code, mem, signal = 0, reason = "") {
    if (p.exited) return;
    p.exited = true; p.code = code; p.signal = signal; p.reason = reason;
    for (const t of p.timers || []) clearTimeout(t);
    if (p.worker) { p.worker.terminate(); p.worker = null; }
    this.stats.live--;
    const ms = performance.now() - p.t0;
    const bt = this.stats.byTool[p.tool] || (this.stats.byTool[p.tool] = { n: 0, ms: 0 });
    bt.n++; bt.ms += ms;
    if (mem) this.stats.maxMem[p.tool] = Math.max(this.stats.maxMem[p.tool] || 0, mem);
    this.closeAll(p);
    this.log(`[pid ${p.pid}] ${p.tool} exit ${code} ${ms.toFixed(0)}ms mem ${mem ? (mem / 2 ** 20).toFixed(0) + "MiB" : "?"} rpcs ${p.rpcs}`);
    // An exited process stays in the table as a zombie until a wait reaps it, as on Unix, so a wait
    // that comes after the exit still gets the status.
    const reaped = p.waiters.length > 0 || p.ppid === 1;
    for (const w of p.waiters) w(code);
    p.waiters = [];
    p.resolve(code);
    if (reaped) this.procs.delete(p.pid);
    if (this.onExit) this.onExit(p, mem);
  }

  /** Starts a process in a new Worker. argv0 is a path in the memfs. p.done resolves with the exit code. */
  spawn(parent, argv0, argv, env, cwd, descs) {
    const path = normPath(argv0, parent ? parent.cwd : cwd);
    const e = this.fs.lookup(path);
    if (!e || e.dir) throw err("ENOENT");
    const p = this.newProc(parent, cwd, descs);
    p.tool = path.split("/").pop(); p.t0 = performance.now(); p.rpcs = 0; p.argv = argv; p.timers = [];
    if (this.testTimeouts) {
      const ta = argv.findIndex(x => /^--?test\.timeout(=|$)/.test(x));
      const text = ta < 0 ? "" : argv[ta].includes("=") ? argv[ta].slice(argv[ta].indexOf("=") + 1) : argv[ta + 1] || "";
      const ms = text ? parseDuration(text) : 0;
      if (ms > 0) { p.tests = new Map(); p.testTail = ""; p.timers.push(setTimeout(() => this.testTimedOut(p, text), ms)); }
    }
    if (this.limitMs > 0) p.timers.push(setTimeout(() => this.overLimit(p), this.limitMs));
    if (this.onSpawn) this.onSpawn(p);
    p.done = new Promise(r => (p.resolve = r));
    this.stats.spawns++; this.stats.live++; this.stats.maxLive = Math.max(this.stats.maxLive, this.stats.live);
    let args = argv.slice(1);
    const size = [...argv, ...env].reduce((n, s) => n + enc.encode(s).length + 16, 0);
    if (size > 6000 && RSP_TOOLS.has(p.tool)) {
      const rsp = `/tmp/rsp-${p.pid}.txt`;
      this.fs.writeFile(rsp, enc.encode(args.map(x => "'" + x.replace(/'/g, "'\\''") + "'").join("\n")));
      args = ["@" + rsp];
    }
    (async () => {
      let mod;
      try { mod = await this.loadModule(path, e); } catch (x) {
        try { this.writeDesc(p.fds.get(2), enc.encode(`kernel: ${path}: ${x && x.stack || x}\n`)); } catch {}
        return this.finish(p, 2);
      }
      if (p.exited) return;
      const sab = new SharedArrayBuffer(CALL_BUFFER.SIZE);
      const i32 = new Int32Array(sab, 0, 4), u8 = new Uint8Array(sab);
      const w = this.makeWorker();
      p.worker = w;
      const reply = (obj, data) => {
        const j = enc.encode(JSON.stringify(obj));
        if (j.length > CALL_BUFFER.JSON_CAPACITY) throw new Error("kernel: reply too large");
        u8.set(j, CALL_BUFFER.JSON_OFFSET); i32[CALL_BUFFER.JSON_LENGTH] = j.length;
        if (data) { u8.set(data, CALL_BUFFER.DATA_OFFSET); i32[CALL_BUFFER.DATA_LENGTH] = data.length; } else i32[CALL_BUFFER.DATA_LENGTH] = -1;
        Atomics.store(i32, CALL_BUFFER.STATE, 1); Atomics.notify(i32, CALL_BUFFER.STATE);
      };
      w.onmessage = m => {
        if (p.exited) return;
        const t0 = performance.now();
        if (m.type === MESSAGE.CALL) {
          p.rpcs++; this.stats.rpcs++; this.stats.ops[m.name] = (this.stats.ops[m.name] || 0) + 1;
          let r;
          try {
            r = this.op(p, m.name, m.args, m.data, (res, data) => { if (!p.exited && p.worker) p.worker.post({ type: MESSAGE.COMPLETE, id: m.id, result: res, data: data && data.slice() }); });
          } catch (x) { reply({ errno: x.code || "EIO", message: String(x.message) }); return; }
          if (r === PENDING) reply({ pendingId: m.id }); else reply({ result: r.res }, r.data);
          this.stats.rpcMs += performance.now() - t0;
        } else if (m.type === MESSAGE.EXIT) {
          const bt = this.stats.byTool[p.tool] || (this.stats.byTool[p.tool] = { n: 0, ms: 0 });
          bt.rpcN = (bt.rpcN || 0) + m.rpcCount; bt.rpcMs = (bt.rpcMs || 0) + m.rpcMs; bt.startMs = (bt.startMs || 0) + m.startMs;
          this.finish(p, m.code, m.memoryBytes);
        }
        else if (m.type === MESSAGE.CRASH) { try { this.writeDesc(p.fds.get(2), enc.encode(`kernel: ${path}: ${m.text}\n`)); } catch {} this.finish(p, 2, m.memoryBytes); }
      };
      const envObj = {};
      for (const kv of env) { const i = kv.indexOf("="); if (i > 0) envObj[kv.slice(0, i)] = kv.slice(i + 1); }
      if (this.toolEnv && this.toolEnv[p.tool]) Object.assign(envObj, this.toolEnv[p.tool]);
      w.post({ type: MESSAGE.START, goroot: this.roImage(), sentAt: performance.timeOrigin + performance.now(), module: mod, buffer: sab, pid: p.pid, ppid: p.ppid, cwd, argv: [argv[0] || path, ...args], env: envObj });
    })();
    return p;
  }

  /** Starts a top-level command with stdout and stderr sent to callbacks. Returns the process; p.done resolves with the wait status. */
  run(argv0, argv, env, cwd, onOut, onErr) {
    const descs = [{ type: "null", refs: 0 }, this.sink(onOut), this.sink(onErr)];
    const envArr = Object.entries(env).map(([a, b]) => `${a}=${b}`);
    return this.spawn(null, argv0, argv, envArr, cwd, descs);
  }
}
