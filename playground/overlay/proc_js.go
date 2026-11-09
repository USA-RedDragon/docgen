// Process support for js/wasm in the docgen playground. The build adds this
// file to package syscall with -overlay.

//go:build js && wasm

package syscall

import "syscall/js"

// The host page supplies goSpawn, goWait, goKill and fs.pipe.
// goSpawn(argv0, argv, env, dir, fds) returns a pid, or an object with an
// errno-style "code" property. goWait(pid) returns a Promise of the exit code.

func jsStrings(s []string) []any {
	a := make([]any, len(s))
	for i, v := range s {
		a[i] = v
	}
	return a
}

func Pipe(fd []int) (err error) {
	if len(fd) != 2 {
		return EINVAL
	}
	defer recoverErr(&err)
	r := jsFS.Call("pipe")
	fd[0], fd[1] = r.Index(0).Int(), r.Index(1).Int()
	filesMu.Lock()
	files[fd[0]] = &jsFile{}
	files[fd[1]] = &jsFile{}
	filesMu.Unlock()
	return nil
}

func StartProcess(argv0 string, argv []string, attr *ProcAttr) (pid int, handle uintptr, err error) {
	spawn := js.Global().Get("goSpawn")
	if spawn.Type() != js.TypeFunction {
		return 0, 0, ENOSYS
	}
	defer recoverErr(&err)
	fds := make([]any, len(attr.Files))
	for i, f := range attr.Files {
		if f > 1<<30 {
			fds[i] = -1
		} else {
			fds[i] = int(f)
		}
	}
	r := spawn.Invoke(argv0, jsStrings(argv), jsStrings(attr.Env), attr.Dir, fds)
	if r.Type() != js.TypeNumber {
		return 0, 0, mapJSError(r)
	}
	return r.Int(), 0, nil
}

func Wait4(pid int, wstatus *WaitStatus, options int, rusage *Rusage) (wpid int, err error) {
	c := make(chan int, 1)
	f := js.FuncOf(func(this js.Value, args []js.Value) any {
		c <- args[0].Int()
		return nil
	})
	defer f.Release()
	p := js.Global().Call("goWait", pid)
	if p.Type() != js.TypeObject {
		return 0, ECHILD
	}
	p.Call("then", f)
	code := <-c
	if wstatus != nil {
		*wstatus = WaitStatus(code)
	}
	return pid, nil
}

func Kill(pid int, signum Signal) error {
	k := js.Global().Get("goKill")
	if k.Type() != js.TypeFunction {
		return ENOSYS
	}
	if !k.Invoke(pid, int(signum)).Bool() {
		return ESRCH
	}
	return nil
}
