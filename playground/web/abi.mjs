// The values that the kernel (kernel.mjs), the process Worker (proc.mjs) and package syscall of Go's
// js/wasm port must agree on. The Go values come from GOROOT/src/syscall of the Go version in go.mod
// (go1.27.0); check them again after a change of that version.

/**
 * The file type bits of the mode of a stat result. syscall.Stat_t.Mode gets the mode as it is, so the
 * values must match the S_IF* constants of GOROOT/src/syscall/syscall_js.go.
 */
export const S_IFMT = 0o370000;
export const S_IFREG = 0o100000;
export const S_IFDIR = 0o040000;
export const S_IFCHR = 0o020000;
export const S_IFIFO = 0o010000;

/**
 * The open flags of fs.constants. syscall.Open in GOROOT/src/syscall/fs_js.go reads these names from
 * fs.constants and maps its own O_* flags to their values, so the names must be the ones it reads. The
 * values are those of Node on Linux, and the kernel reads the flags of an open call with them.
 */
export const OPEN_FLAGS = Object.freeze({
  O_RDONLY: 0,
  O_WRONLY: 1,
  O_RDWR: 2,
  O_CREAT: 64,
  O_EXCL: 128,
  O_TRUNC: 512,
  O_APPEND: 1024,
  O_DIRECTORY: 65536,
});
/** The bits of the access mode (O_RDONLY, O_WRONLY or O_RDWR) in open flags. */
export const O_ACCMODE = 3;
/** The open flags that can change a file. */
export const WRITE_FLAGS = O_ACCMODE | OPEN_FLAGS.O_CREAT | OPEN_FLAGS.O_TRUNC | OPEN_FLAGS.O_APPEND;

/**
 * The errno codes that the kernel and the Worker give as the code property of an Error. mapJSError in
 * GOROOT/src/syscall/fs_js.go looks up the code in errnoByCode of GOROOT/src/syscall/tables_js.go and
 * panics on a code that is not there, so each code must be a key of that map.
 */
export const ERRNO = Object.freeze({
  EBADF: "EBADF",
  EEXIST: "EEXIST",
  EINVAL: "EINVAL",
  EIO: "EIO",
  EISDIR: "EISDIR",
  ENOENT: "ENOENT",
  ENOSYS: "ENOSYS",
  ENOTDIR: "ENOTDIR",
  ENOTEMPTY: "ENOTEMPTY",
  EPERM: "EPERM",
  EPIPE: "EPIPE",
  EROFS: "EROFS",
});

/** errnoError returns an Error with the errno code and a message, by default the code. */
export function errnoError(code, message) {
  const error = new Error(message || code);
  error.code = code;
  return error;
}

/**
 * The fs calls that the Worker sends to the kernel unchanged: the name and each argument but the last,
 * the callback. fsCall in GOROOT/src/syscall/fs_js.go calls them by these names. The Worker has its own
 * code for the other calls: open, close, read, fstat, stat, lstat and readdir use the read-only GOROOT
 * image of the Worker, and write and pipe have their own arguments.
 */
export const FORWARDED_FS_CALLS = Object.freeze([
  "mkdir",
  "unlink",
  "rmdir",
  "rename",
  "fsync",
  "ftruncate",
  "truncate",
  "chmod",
  "fchmod",
  "chown",
  "fchown",
  "lchown",
  "utimes",
  "readlink",
  "link",
  "symlink",
]);

/**
 * The signal numbers of js/wasm: the Signal constants of GOROOT/src/syscall/syscall_js.go, which
 * start at SIGCHLD = 1.
 */
export const SIG = Object.freeze({ INT: 2, KILL: 3, TRAP: 4, QUIT: 5, TERM: 6 });
/** The Unix name and number of each js/wasm signal number, for the text that the page shows. */
export const SIGNAL_NAMES = Object.freeze({
  2: ["SIGINT", 2],
  3: ["SIGKILL", 9],
  4: ["SIGTRAP", 5],
  5: ["SIGQUIT", 3],
  6: ["SIGTERM", 15],
});
/**
 * A process killed by signal s has the wait status KILLED + s. The overlay of syscall.WaitStatus in
 * playground/build/mkoverlay.py decodes it.
 */
export const KILLED = 256;

/**
 * The layout of the SharedArrayBuffer of a synchronous call from a Worker to the kernel. The Worker
 * sets the Int32 at STATE to 0 and waits on it. The kernel writes the JSON of the reply at
 * JSON_OFFSET and its length at JSON_LENGTH, the data of a read at DATA_OFFSET and its length (or -1
 * for no data) at DATA_LENGTH, then sets STATE to 1 and wakes the Worker. STATE, JSON_LENGTH and
 * DATA_LENGTH are indexes of Int32 elements; the others are byte offsets and sizes.
 */
export const CALL_BUFFER = Object.freeze({
  STATE: 0,
  JSON_LENGTH: 1,
  DATA_LENGTH: 2,
  JSON_OFFSET: 16,
  JSON_CAPACITY: 1 << 20,
  DATA_OFFSET: (1 << 20) + 16,
  DATA_CAPACITY: 4 << 20,
  SIZE: (1 << 20) + 16 + (4 << 20),
});

/**
 * The types of the messages between the kernel and a Worker. START, COMPLETE: kernel to Worker. CALL,
 * EXIT, CRASH: Worker to kernel.
 */
export const MESSAGE = Object.freeze({
  START: "start",
  CALL: "call",
  COMPLETE: "complete",
  EXIT: "exit",
  CRASH: "crash",
});
