// Command zstd compresses the file SRC into the file DST with the best level
// of github.com/klauspost/compress/zstd. build.sh uses it when the zstd
// command is not on PATH.
//
// Usage: go run . SRC DST
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: zstd SRC DST")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "zstd:", err)
		os.Exit(1)
	}
}

func run(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	w, err := zstd.NewWriter(out, zstd.WithEncoderLevel(zstd.SpeedBestCompression), zstd.WithWindowSize(1<<23))
	if err != nil {
		out.Close()
		return err
	}
	if _, err := io.Copy(w, in); err != nil {
		out.Close()
		return err
	}
	if err := w.Close(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
