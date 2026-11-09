// Command unzstd decodes zstd data in a browser Worker. It sets the global
// function zstdDecode(Uint8Array, size) that returns a Uint8Array, and then calls the
// global function zstdReady. The decoder
// is a copy of the internal/zstd package of the Go standard library.
package main

import (
	"bytes"
	"io"
	"syscall/js"

	"playground/unzstd/zstd"
)

func main() {
	js.Global().Set("zstdDecode", js.FuncOf(func(this js.Value, args []js.Value) any {
		in := make([]byte, args[0].Get("length").Int())
		js.CopyBytesToGo(in, args[0])
		size := 0
		if len(args) > 1 && args[1].Type() == js.TypeNumber {
			size = args[1].Int()
		}
		out := bytes.NewBuffer(make([]byte, 0, size))
		if _, err := io.Copy(out, zstd.NewReader(bytes.NewReader(in))); err != nil {
			return js.Global().Get("Error").New("zstd: " + err.Error())
		}
		u := js.Global().Get("Uint8Array").New(out.Len())
		js.CopyBytesToJS(u, out.Bytes())
		return u
	}))
	js.Global().Call("zstdReady")
	select {}
}
