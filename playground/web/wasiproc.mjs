// The WASI process type of the kernel: a wasm32-wasip1 program, such as the Rust helper of docgen,
// in the Worker of proc.mjs. @bjorn3/browser_wasi_shim (the bundle wasi-shim.js) turns each WASI
// call of the program into a call of an Fd object, and the Fd classes here send the call to the
// kernel with the synchronous call of proc.mjs, so the program sees the memfs and the descriptors
// of the kernel as a Go process does. The shim is used as it is; the classes here replace three of
// its parts:
//   - path_filestat_set_times and fd_filestat_set_times succeed. cargo and std set times on files,
//     and the in-memory classes of the shim return ENOTSUP.
//   - path_readlink fails with EINVAL, as for a file that is not a link, since the memfs has no links.
//   - Filestat.write_bytes of the shim (0.4.2) writes atim, mtim and ctim at the offsets 38, 46 and
//     52, not 40, 48 and 56, so the times of a stat are wrong. KernelFilestat writes them right.
// path_rename of the shim moves an inode between its own directory objects, so runWasi replaces
// that one import with a rename call of the kernel.
import { S_IFMT, S_IFDIR, S_IFREG, S_IFCHR, OPEN_FLAGS, CALL_BUFFER } from "./abi.mjs";
import { normalizePath } from "./paths.mjs";

const textDecoder = new TextDecoder();
/** The import module of the WASI calls, and the one of the Go js/wasm calls. */
const WASI_MODULE = "wasi_snapshot_preview1";
const GO_MODULE = "gojs";
const STDIN = 0;
const STDERR = 2;
const NS_PER_MS = 1000000n;

/** isWasi reports whether the WebAssembly.Module imports WASI calls and no Go js/wasm calls. */
export function isWasi(module) {
  const names = new Set(WebAssembly.Module.imports(module).map(entry => entry.module));
  return names.has(WASI_MODULE) && !names.has(GO_MODULE);
}

/**
 * runWasi runs the WASI module with argv and env, an object of variables, and resolves with
 * {code, memoryBytes}. The program gets the directory "/" as descriptor 3, its only preopened
 * directory, so its working directory is "/" and a relative path is relative to "/". shim is the module of wasi-shim.js. call(name, args, data, block) is the
 * synchronous call of proc.mjs; with block set, a call that cannot finish at once, such as a read
 * of an empty pipe, returns its result when it finishes. It rejects with the trap of a program that
 * stops with one, such as a Rust panic, which aborts.
 */
export async function runWasi(shim, { module, argv, env, call }) {
  const { WASI, Fd, wasi } = shim;
  const errnos = {
    EBADF: wasi.ERRNO_BADF,
    EEXIST: wasi.ERRNO_EXIST,
    EINVAL: wasi.ERRNO_INVAL,
    EIO: wasi.ERRNO_IO,
    EISDIR: wasi.ERRNO_ISDIR,
    ENOENT: wasi.ERRNO_NOENT,
    ENOSYS: wasi.ERRNO_NOSYS,
    ENOTDIR: wasi.ERRNO_NOTDIR,
    ENOTEMPTY: wasi.ERRNO_NOTEMPTY,
    EPERM: wasi.ERRNO_PERM,
    EPIPE: wasi.ERRNO_PIPE,
    EROFS: wasi.ERRNO_ROFS,
  };
  const errnoOf = error => errnos[error && error.code] || wasi.ERRNO_IO;

  /**
   * attempt runs body and returns its result with ret ERRNO_SUCCESS, or for an errno Error of the
   * kernel {ret: the WASI errno, ...failed}.
   */
  const attempt = (body, failed = {}) => {
    try {
      return { ret: wasi.ERRNO_SUCCESS, ...body() };
    } catch (error) {
      return { ...failed, ret: errnoOf(error) };
    }
  };
  /** status runs body and returns ERRNO_SUCCESS, or the WASI errno of its errno Error. */
  const status = body => attempt(() => (body(), {})).ret;

  class KernelFilestat extends wasi.Filestat {
    write_bytes(view, ptr) {
      view.setBigUint64(ptr, this.dev, true);
      view.setBigUint64(ptr + 8, this.ino, true);
      view.setUint8(ptr + 16, this.filetype);
      view.setBigUint64(ptr + 24, this.nlink, true);
      view.setBigUint64(ptr + 32, this.size, true);
      view.setBigUint64(ptr + 40, this.atim, true);
      view.setBigUint64(ptr + 48, this.mtim, true);
      view.setBigUint64(ptr + 56, this.ctim, true);
    }
  }

  const fileType = mode => {
    switch (mode & S_IFMT) {
      case S_IFDIR:
        return wasi.FILETYPE_DIRECTORY;
      case S_IFREG:
        return wasi.FILETYPE_REGULAR_FILE;
      case S_IFCHR:
        return wasi.FILETYPE_CHARACTER_DEVICE;
      default:
        return wasi.FILETYPE_UNKNOWN;
    }
  };
  /** filestat returns the Filestat of a stat result of the kernel. */
  const filestat = stats => {
    const result = new KernelFilestat(BigInt(stats.ino), fileType(stats.mode), BigInt(stats.size));
    result.nlink = 1n;
    result.mtim = BigInt(Math.round(stats.mtimeMs)) * NS_PER_MS;
    result.atim = result.mtim;
    result.ctim = result.mtim;
    return result;
  };
  const statPath = path => call("stat", [path]).result;
  /** readLength limits a read to the data area of the call buffer. */
  const readLength = size => Math.min(size, CALL_BUFFER.DATA_CAPACITY);

  /**
   * KernelFile is a descriptor of the kernel. A file that the program opens keeps its own offset and
   * reads and writes at it. Standard input, output and error are shared with other processes, so they
   * use the offset of the kernel, and a read of a pipe with no data waits for data.
   */
  class KernelFile extends Fd {
    constructor(descriptor, { seekable = false, append = false } = {}) {
      super();
      this.descriptor = descriptor;
      this.seekable = seekable;
      this.append = append;
      this.offset = 0n;
    }

    fd_fdstat_get() {
      return attempt(
        () => {
          const stats = call("fstat", [this.descriptor]).result;
          return { fdstat: new wasi.Fdstat(fileType(stats.mode), this.append ? wasi.FDFLAGS_APPEND : 0) };
        },
        { fdstat: null },
      );
    }

    fd_filestat_get() {
      return attempt(() => ({ filestat: filestat(call("fstat", [this.descriptor]).result) }), { filestat: null });
    }

    fd_filestat_set_size(size) {
      return status(() => call("ftruncate", [this.descriptor, Number(size)]));
    }

    fd_filestat_set_times() {
      return wasi.ERRNO_SUCCESS;
    }

    readAt(size, position) {
      const reply = call("read", [this.descriptor, readLength(size), position], undefined, position == null);
      return reply.result ? reply.data.slice() : new Uint8Array(0);
    }

    fd_read(size) {
      return attempt(
        () => {
          const data = this.readAt(size, this.seekable ? Number(this.offset) : null);
          if (this.seekable) this.offset += BigInt(data.length);
          return { data };
        },
        { data: new Uint8Array(0) },
      );
    }

    fd_pread(size, offset) {
      if (!this.seekable) return { ret: wasi.ERRNO_SPIPE, data: new Uint8Array(0) };
      return attempt(() => ({ data: this.readAt(size, Number(offset)) }), { data: new Uint8Array(0) });
    }

    fd_write(data) {
      return attempt(
        () => {
          const position = this.seekable && !this.append ? Number(this.offset) : null;
          const nwritten = call("write", [this.descriptor, position], data).result;
          if (this.seekable && !this.append) this.offset += BigInt(nwritten);
          return { nwritten };
        },
        { nwritten: 0 },
      );
    }

    fd_pwrite(data, offset) {
      if (!this.seekable) return { ret: wasi.ERRNO_SPIPE, nwritten: 0 };
      return attempt(() => ({ nwritten: call("write", [this.descriptor, Number(offset)], data).result }), {
        nwritten: 0,
      });
    }

    fd_seek(offset, whence) {
      if (!this.seekable) return { ret: wasi.ERRNO_SPIPE, offset: 0n };
      return attempt(
        () => {
          let base = 0n;
          if (whence === wasi.WHENCE_CUR) base = this.offset;
          if (whence === wasi.WHENCE_END) base = BigInt(call("fstat", [this.descriptor]).result.size);
          if (base + offset < 0n) return { ret: wasi.ERRNO_INVAL, offset: this.offset };
          this.offset = base + offset;
          return { offset: this.offset };
        },
        { offset: 0n },
      );
    }

    fd_tell() {
      if (!this.seekable) return { ret: wasi.ERRNO_SPIPE, offset: 0n };
      return { ret: wasi.ERRNO_SUCCESS, offset: this.offset };
    }

    fd_close() {
      return status(() => call("close", [this.descriptor]));
    }
  }

  /**
   * KernelDirectory is a directory of the memfs by its absolute path. The directory "/" is the one
   * preopened directory, so wasi-libc resolves each absolute path in it.
   */
  class KernelDirectory extends Fd {
    constructor(path, preopenName = null) {
      super();
      this.path = path;
      this.preopenName = preopenName;
      this.names = null;
    }

    resolve(path) {
      return normalizePath(path, this.path);
    }

    fd_prestat_get() {
      if (this.preopenName == null) return { ret: wasi.ERRNO_BADF, prestat: null };
      return { ret: wasi.ERRNO_SUCCESS, prestat: wasi.Prestat.dir(this.preopenName) };
    }

    fd_fdstat_get() {
      return { ret: wasi.ERRNO_SUCCESS, fdstat: new wasi.Fdstat(wasi.FILETYPE_DIRECTORY, 0) };
    }

    fd_filestat_get() {
      return attempt(() => ({ filestat: filestat(statPath(this.path)) }), { filestat: null });
    }

    fd_filestat_set_times() {
      return wasi.ERRNO_SUCCESS;
    }

    fd_readdir_single(cookie) {
      return attempt(
        () => {
          if (cookie === 0n || !this.names) this.names = call("readdir", [this.path]).result;
          if (cookie >= BigInt(this.names.length)) return { dirent: null };
          const name = this.names[Number(cookie)];
          const stats = statPath(this.path + "/" + name);
          return { dirent: new wasi.Dirent(cookie + 1n, BigInt(stats.ino), name, fileType(stats.mode)) };
        },
        { dirent: null },
      );
    }

    path_open(dirflags, path, oflags, rightsBase, rightsInheriting, fdflags) {
      return attempt(
        () => {
          const absolute = this.resolve(path);
          let stats = null;
          try {
            stats = statPath(absolute);
          } catch {
            stats = null;
          }
          if (stats && (stats.mode & S_IFMT) === S_IFDIR) {
            if (oflags & wasi.OFLAGS_CREAT && oflags & wasi.OFLAGS_EXCL) return { ret: wasi.ERRNO_EXIST, fd_obj: null };
            if (oflags & wasi.OFLAGS_TRUNC) return { ret: wasi.ERRNO_ISDIR, fd_obj: null };
            return { fd_obj: new KernelDirectory(absolute) };
          }
          if (oflags & wasi.OFLAGS_DIRECTORY) return { ret: stats ? wasi.ERRNO_NOTDIR : wasi.ERRNO_NOENT, fd_obj: null };
          const read = (BigInt(rightsBase) & BigInt(wasi.RIGHTS_FD_READ)) !== 0n;
          const write = (BigInt(rightsBase) & BigInt(wasi.RIGHTS_FD_WRITE)) !== 0n;
          const append = (fdflags & wasi.FDFLAGS_APPEND) !== 0;
          let flags = OPEN_FLAGS.O_RDONLY;
          if (write || append) flags = read ? OPEN_FLAGS.O_RDWR : OPEN_FLAGS.O_WRONLY;
          if (oflags & wasi.OFLAGS_CREAT) flags |= OPEN_FLAGS.O_CREAT;
          if (oflags & wasi.OFLAGS_EXCL) flags |= OPEN_FLAGS.O_EXCL;
          if (oflags & wasi.OFLAGS_TRUNC) flags |= OPEN_FLAGS.O_TRUNC;
          if (append) flags |= OPEN_FLAGS.O_APPEND;
          const descriptor = call("open", [absolute, flags, 0o644]).result;
          return { fd_obj: new KernelFile(descriptor, { seekable: true, append }) };
        },
        { fd_obj: null },
      );
    }

    path_filestat_get(flags, path) {
      return attempt(() => ({ filestat: filestat(statPath(this.resolve(path))) }), { filestat: null });
    }

    path_filestat_set_times(flags, path, atim, mtim, fstFlags) {
      return status(() => {
        const absolute = this.resolve(path);
        statPath(absolute);
        let mtimeMs = null;
        if (fstFlags & wasi.FSTFLAGS_MTIM_NOW) mtimeMs = Date.now();
        else if (fstFlags & wasi.FSTFLAGS_MTIM) mtimeMs = Number(mtim / NS_PER_MS);
        if (mtimeMs != null) call("utimes", [absolute, mtimeMs / 1000, mtimeMs / 1000]);
      });
    }

    path_readlink() {
      return { ret: wasi.ERRNO_INVAL, data: null };
    }

    path_create_directory(path) {
      return status(() => call("mkdir", [this.resolve(path), 0o755]));
    }

    path_remove_directory(path) {
      return status(() => call("rmdir", [this.resolve(path)]));
    }

    path_unlink_file(path) {
      return status(() => call("unlink", [this.resolve(path)]));
    }
  }

  const fds = [
    new KernelFile(STDIN),
    new KernelFile(STDIN + 1),
    new KernelFile(STDERR),
    new KernelDirectory("/", "/"),
  ];
  const envList = Object.entries(env).map(([name, value]) => `${name}=${value}`);
  const runner = new WASI(argv, envList, fds, { debug: false });
  runner.wasiImport.path_rename = (fd, oldPtr, oldLength, newFd, newPtr, newLength) => {
    const from = runner.fds[fd];
    const to = runner.fds[newFd];
    if (!(from instanceof KernelDirectory) || !(to instanceof KernelDirectory)) return wasi.ERRNO_BADF;
    const memory = new Uint8Array(runner.inst.exports.memory.buffer);
    const oldPath = textDecoder.decode(memory.slice(oldPtr, oldPtr + oldLength));
    const newPath = textDecoder.decode(memory.slice(newPtr, newPtr + newLength));
    return status(() => call("rename", [from.resolve(oldPath), to.resolve(newPath)]));
  };
  const instance = await WebAssembly.instantiate(module, { [WASI_MODULE]: runner.wasiImport });
  const code = runner.start(instance);
  return { code, memoryBytes: instance.exports.memory.buffer.byteLength };
}
