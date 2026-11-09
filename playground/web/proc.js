// One process: runs a Go js/wasm module in this Worker. Each fs, process and spawn call is a
// synchronous RPC to the kernel on the main thread (postMessage, then Atomics.wait on a SharedArrayBuffer).
// Works as a browser Worker and as a Node worker_threads Worker.
"use strict";
(() => {
  const isNode = typeof importScripts !== "function";
  let post, onmsg;
  if (isNode) {
    const wt = require("node:worker_threads");
    require(wt.workerData.wasmExec);
    post = m => wt.parentPort.postMessage(m);
    onmsg = f => wt.parentPort.on("message", f);
  } else {
    importScripts("wasm_exec.js");
    post = m => postMessage(m);
    onmsg = f => { self.onmessage = e => f(e.data); };
  }
  const JSON_OFF = 16, DATA_OFF = (1 << 20) + 16;
  const dec = new TextDecoder();
  const S_IFMT = 0o170000, S_IFDIR = 0o040000;
  const C = { O_RDONLY: 0, O_WRONLY: 1, O_RDWR: 2, O_CREAT: 64, O_EXCL: 128, O_TRUNC: 512, O_APPEND: 1024, O_DIRECTORY: 65536 };
  let i32, u8, nextId = 1, inst = null;
  const pending = new Map();
  const mkerr = (code, m) => { const e = new Error(m || code); e.code = code; return e; };
  // call returns {r, data} or {p: id}; it throws a coded Error.
  let rpcN = 0, rpcMs = 0;
  function call(op, a, d) {
    const t0 = performance.now(); rpcN++;
    Atomics.store(i32, 0, 0);
    const id = nextId++;
    post({ t: "s", id, op, a, d });
    Atomics.wait(i32, 0, 0);
    const jl = i32[1], dl = i32[2];
    const jb = new Uint8Array(jl); jb.set(u8.subarray(JSON_OFF, JSON_OFF + jl));
    const o = JSON.parse(dec.decode(jb));
    if (o.e) throw mkerr(o.e, o.m);
    if (dl >= 0) o.data = u8.subarray(DATA_OFF, DATA_OFF + dl);
    rpcMs += performance.now() - t0;
    return o;
  }
  const later = (cb, ...r) => queueMicrotask(() => cb(...r));
  const st = s => { s.isDirectory = () => (s.mode & S_IFMT) === S_IFDIR; s.dev = 1; s.nlink = 1; s.uid = 0; s.gid = 0; s.rdev = 0; s.blksize = 4096; s.blocks = Math.ceil(s.size / 512); s.atimeMs = s.ctimeMs = s.mtimeMs; return s; };
  const simple = (op, conv = x => x) => (...a) => { const cb = a.pop(); let r; try { r = conv(call(op, a).r); } catch (e) { return later(cb, e); } later(cb, null, r); };
  let cwd = "/";
  // Read-only GOROOT image shared by all processes: open, stat, read, readdir and close of /goroot paths
  // run in the Worker with no RPC. The kernel refuses writes under /goroot, so the image stays exact.
  let ro = null, roData = null, nextLocal = 1 << 20;
  const localFds = new Map();
  const norm = p => {
    if (!p.startsWith("/")) p = cwd + "/" + p;
    const out = [];
    for (const x of p.split("/")) { if (!x || x === ".") continue; if (x === "..") out.pop(); else out.push(x); }
    return "/" + out.join("/");
  };
  const roPath = p => { if (!ro) return null; const q = norm(p); return q === "/goroot" || q.startsWith("/goroot/") ? q : null; };
  const roStat = e => st({ ino: e[3], mode: e[4], size: e[1], mtimeMs: e[2] });
  const enoent = () => mkerr("ENOENT");
  const fs = {
    constants: C,
    writeSync(fd, b) { return call("writeSync", [fd, null], b.slice()).r; },
    write(fd, buf, offset, length, position, cb) {
      let r; try { r = call("write", [fd, position], buf.slice(offset, offset + length)).r; } catch (e) { return later(cb, e); }
      later(cb, null, r);
    },
    read(fd, buf, offset, length, position, cb) {
      const lf = localFds.get(fd);
      if (lf) {
        const e = lf.e, pos = position != null ? position : lf.pos;
        const n = Math.max(0, Math.min(length, e[1] - pos));
        buf.set(roData.subarray(e[0] + pos, e[0] + pos + n), offset);
        if (position == null) lf.pos += n;
        return later(cb, null, n);
      }
      let o; try { o = call("read", [fd, length, position]); } catch (e) { return later(cb, e); }
      if (o.p) { pending.set(o.p, (n, data) => { if (n) buf.set(data, offset); cb(null, n); }); return; }
      if (o.r) buf.set(o.data, offset);
      later(cb, null, o.r);
    },
    pipe() { return call("pipe", []).r; },
    open: (path, flags, mode, cb) => {
      const q = roPath(path);
      if (q && !(flags & (3 | C.O_CREAT | C.O_TRUNC | C.O_APPEND))) {
        const e = ro.get(q);
        if (!e) return later(cb, enoent());
        if ((flags & C.O_DIRECTORY) && !e[5]) return later(cb, mkerr("ENOTDIR"));
        const fd = nextLocal++; localFds.set(fd, { e, pos: 0 }); return later(cb, null, fd);
      }
      simpleOpen(path, flags, mode, cb);
    },
    close: (fd, cb) => { if (localFds.delete(fd)) return later(cb, null, null); simpleClose(fd, cb); },
    fstat: (fd, cb) => { const lf = localFds.get(fd); if (lf) return later(cb, null, roStat(lf.e)); simpleFstat(fd, cb); },
    stat: (path, cb) => { const q = roPath(path); if (q) { const e = ro.get(q); return e ? later(cb, null, roStat(e)) : later(cb, enoent()); } simpleStat(path, cb); },
    lstat: (path, cb) => { const q = roPath(path); if (q) { const e = ro.get(q); return e ? later(cb, null, roStat(e)) : later(cb, enoent()); } simpleLstat(path, cb); },
    readdir: (path, cb) => {
      const q = roPath(path);
      if (q) { const e = ro.get(q); if (!e) return later(cb, enoent()); if (!e[5]) return later(cb, mkerr("ENOTDIR")); return later(cb, null, e[5].slice()); }
      simpleReaddir(path, cb);
    },
    mkdir: simple("mkdir"), unlink: simple("unlink"), rmdir: simple("rmdir"), rename: simple("rename"),
    fsync: simple("fsync"), ftruncate: simple("ftruncate"), truncate: simple("truncate"), chmod: simple("chmod"), fchmod: simple("fchmod"),
    chown: simple("chown"), fchown: simple("fchown"), lchown: simple("lchown"), utimes: simple("utimes"), readlink: simple("readlink"),
    link: simple("link"), symlink: simple("symlink"),
  };
  const simpleOpen = simple("open"), simpleClose = simple("close"), simpleFstat = simple("fstat", st), simpleStat = simple("stat", st),
    simpleLstat = simple("lstat", st), simpleReaddir = simple("readdir");
  let pid = 0, ppid = 0;
  const proc = {
    get pid() { return pid; }, get ppid() { return ppid; },
    getuid: () => -1, getgid: () => -1, geteuid: () => -1, getegid: () => -1, getgroups: () => [], umask: () => 0o022,
    cwd: () => cwd,
    chdir: d => { cwd = call("chdir", [d]).r; },
  };
  const api = {
    fs, process: proc,
    path: { resolve: (...a) => a.reduce((acc, s) => s.startsWith("/") ? s : acc + "/" + s, cwd) },
    goSpawn: (argv0, argv, env, dir, fds) => fds.some(f => localFds.has(f)) ? { code: "EBADF", message: "read-only fd" } : call("spawn", [argv0, argv, env, dir, fds]).r,
    goKill: (p, sig) => call("kill", [p, sig]).r,
    goWait: p => {
      let o; try { o = call("wait", [p]); } catch { return null; }
      if (o.r === null && !o.p) return null;
      return new Promise(res => { if (o.p) pending.set(o.p, res); else res(o.r); });
    },
  };
  const g = new Proxy(globalThis, { get: (t, k) => k in api ? api[k] : Reflect.get(t, k) });
  onmsg(async m => {
    if (m.t === "r") { const f = pending.get(m.id); pending.delete(m.id); if (f) f(m.res, m.data); return; }
    if (m.t !== "start") return;
    i32 = new Int32Array(m.sab, 0, 4); u8 = new Uint8Array(m.sab);
    pid = m.pid; ppid = m.ppid; cwd = m.cwd;
    if (m.ro) { ro = new Map(m.ro.index); roData = new Uint8Array(m.ro.sab); }
    let code = 0, tMsg = m.t0 || 0, tStart = 0;
    try {
      const go = new Go();
      go.global = g; go.argv = m.argv; go.env = m.env;
      go.exit = c => { code = c; };
      inst = await WebAssembly.instantiate(m.mod, go.importObject);
      tStart = performance.timeOrigin + performance.now();
      await go.run(inst);
      post({ t: "x", code, mem: inst.exports.mem.buffer.byteLength, rpcN, rpcMs, startMs: tStart - tMsg });
    } catch (x) {
      post({ t: "err", msg: String(x && x.stack || x), mem: inst ? inst.exports.mem.buffer.byteLength : 0 });
    }
  });
})();
