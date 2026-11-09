"""Writes the overlay of the docgen playground: copies of the GOROOT files that
change for the process bridge of js/wasm, the new files of playground/overlay,
and overlay.json for go build -overlay. It also writes wasm_exec.js, the copy of
lib/wasm/wasm_exec.js that the playground workers load.

GOROOT is only read. Each edit is an exact string replacement that must match
once, so a Go version with other source stops the build with a message.
Usage: python3 -I mkoverlay.py GOROOT OUTDIR
"""
import json, os, sys

GOROOT, OUT = sys.argv[1], os.path.abspath(sys.argv[2])
SRC = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "overlay")
OV = os.path.join(OUT, "overlay")
out = {}


def patch(rel, edits):
    src = open(os.path.join(GOROOT, rel)).read()
    for old, new in edits:
        if src.count(old) != 1:
            sys.exit(f"{rel}: expected one match for {old!r}, got {src.count(old)}")
        src = src.replace(old, new)
    add(rel, src)


def add(rel, text):
    dst = os.path.join(OV, rel)
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    open(dst, "w").write(text)
    out[os.path.join(GOROOT, rel)] = dst


patch("src/cmd/go/internal/lockedfile/internal/filelock/filelock_other.go", [
    ('import (\n\t"errors"\n\t"io/fs"\n)\n', 'import (\n\t"errors"\n\t"io/fs"\n\t"runtime"\n)\n'),
    ("func lock(f File, lt lockType) error {\n",
     "func lock(f File, lt lockType) error {\n\tif runtime.GOOS == \"js\" {\n\t\treturn nil\n\t}\n"),
    ("func unlock(f File) error {\n",
     "func unlock(f File) error {\n\tif runtime.GOOS == \"js\" {\n\t\treturn nil\n\t}\n"),
])

patch("src/syscall/syscall_js.go", [
    # The kernel reports an exit as its status (0-255) and a kill as 256 + the js signal number.
    ("func (w WaitStatus) Exited() bool       { return false }\nfunc (w WaitStatus) ExitStatus() int    { return 0 }\n"
     "func (w WaitStatus) Signaled() bool     { return false }\nfunc (w WaitStatus) Signal() Signal     { return 0 }\n",
     "func (w WaitStatus) Exited() bool       { return w < 256 }\n"
     "func (w WaitStatus) ExitStatus() int {\n\tif w >= 256 {\n\t\treturn -1\n\t}\n\treturn int(w)\n}\n"
     "func (w WaitStatus) Signaled() bool { return w >= 256 }\n"
     "func (w WaitStatus) Signal() Signal {\n\tif w < 256 {\n\t\treturn -1\n\t}\n\treturn Signal(w - 256)\n}\n"),
    ("var signals = [...]string{}\n",
     "var signals = [...]string{SIGCHLD: \"child exited\", SIGINT: \"interrupt\", SIGKILL: \"killed\", SIGTRAP: \"trace/breakpoint trap\", SIGQUIT: \"quit\", SIGTERM: \"terminated\"}\n"),
    ("func Kill(pid int, signum Signal) error { return ENOSYS }\n", ""),
    ("func StartProcess(argv0 string, argv []string, attr *ProcAttr) (pid int, handle uintptr, err error) {\n\treturn 0, 0, ENOSYS\n}\n", ""),
    ("func Wait4(pid int, wstatus *WaitStatus, options int, rusage *Rusage) (wpid int, err error) {\n\treturn 0, ENOSYS\n}\n", ""),
])

patch("src/syscall/fs_js.go", [
    ("func Pipe(fd []int) error {\n\treturn ENOSYS\n}\n", ""),
])

add("src/syscall/proc_js.go", open(os.path.join(SRC, "proc_js.go")).read())

patch("src/os/pipe_wasm.go", [
    ("\t// Neither GOOS=js nor GOOS=wasip1 have pipes.\n\treturn nil, nil, NewSyscallError(\"pipe\", syscall.ENOSYS)\n",
     "\tvar p [2]int\n\tif e := syscall.Pipe(p[:]); e != nil {\n\t\treturn nil, nil, NewSyscallError(\"pipe\", e)\n\t}\n"
     "\treturn newFile(p[0], \"|0\", kindPipe, false), newFile(p[1], \"|1\", kindPipe, false), nil\n"),
])

patch("src/os/exec/lp_wasm.go", [
    ('import (\n\t"errors"\n)\n', 'import (\n\t"errors"\n\t"runtime"\n)\n'),
    ("\t// Wasm can not execute processes, so act as if there are no executables at all.\n",
     "\tif runtime.GOOS == \"js\" {\n\t\treturn jsLookPath(file)\n\t}\n"
     "\t// Wasm can not execute processes, so act as if there are no executables at all.\n"),
])
add("src/os/exec/lp_js.go", open(os.path.join(SRC, "lp_js.go")).read())

json.dump({"Replace": out}, open(os.path.join(OUT, "overlay.json"), "w"), indent=1)

# The kernel gives each worker its own fs object, so wasm_exec.js reads fs and
# the global object from this.global in place of globalThis.
js = open(os.path.join(GOROOT, "lib/wasm/wasm_exec.js")).read()
for old, new in [
    ("\t\t\tthis.env = {};\n", "\t\t\tthis.global = globalThis;\n\t\t\tthis.env = {};\n"),
    ("\t\t\t\t\t\tfs.writeSync(fd, new Uint8Array(this._inst.exports.mem.buffer, p, n));", "\t\t\t\t\t\tthis.global.fs.writeSync(fd, new Uint8Array(this._inst.exports.mem.buffer, p, n));"),
    ("\t\t\t\tglobalThis,\n", "\t\t\t\tthis.global,\n"),
    ("\t\t\t\t[globalThis, 5],", "\t\t\t\t[this.global, 5],"),
]:
    if js.count(old) != 1:
        sys.exit(f"wasm_exec.js: expected one match for {old!r}, got {js.count(old)}")
    js = js.replace(old, new)
open(os.path.join(OUT, "wasm_exec.js"), "w").write(js)
