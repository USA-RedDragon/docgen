package gen

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/USA-RedDragon/docgen/internal/gomod"
)

// copyRoot copies the directory src to dst, with no .git entry, no entry that is not a file, directory or symbolic
// link, and not the directory session, which holds dst. Each file and directory
// of the copy is writable, since the run updates it. A symbolic link outside src
// is an error. root is the name of src in the error.
func copyRoot(ctx context.Context, src, dst, session, root string) error {
	src = canonical(src)
	session = canonical(session)
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil || rel == "." {
			return err
		}
		if d.Name() == ".git" || p == session {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		to := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return copyDir(d, to)
		case d.Type()&fs.ModeSymlink != 0:
			if !gomod.Inside(src, p) {
				return fmt.Errorf("%s is a symbolic link outside root=%s", filepath.ToSlash(rel), root)
			}
			return copyLink(p, to)
		case d.Type().IsRegular():
			return copyFile(d, p, to)
		}
		return nil
	})
}

func copyDir(d fs.DirEntry, to string) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	return os.Mkdir(to, info.Mode().Perm()|0o700)
}

// copyLink makes a link at to with the target of the link at p, as a relative
// path. The directories of to match those of p, so the link stays in the copy.
func copyLink(p, to string) error {
	target, err := filepath.Rel(filepath.Dir(p), canonical(p))
	if err != nil {
		return err
	}
	return os.Symlink(target, to)
}

func copyFile(d fs.DirEntry, p, to string) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	in, err := os.Open(p)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm()|0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
