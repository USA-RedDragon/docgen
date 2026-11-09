"""Copies u-root command main packages into bb/ucmd/<name> as importable packages.

Each copy has package u<name>, func Main in place of main, and a flag set of its
own in place of flag.CommandLine, so that one binary can hold all of them.
Usage: python3 -I vendor-uroot.py <u-root dir> <out dir> <name>...

After a change of the u-root version in go.mod, run it again from playground/bb:

  python3 -I vendor-uroot.py "$(go list -m -f "{{.Dir}}" github.com/u-root/u-root)" ucmd $(cd ucmd && ls -d */ | tr -d /)

It also copies the LICENSE file of u-root into the out dir.

build.sh stops when ucmd differs from its output.
"""
import os
import re
import shutil
import sys
from pathlib import Path

DATE_JS_GO = """\
//go:build js

package udate

import (
	"errors"
	"time"
)

func setDate(d string, z *time.Location, clocksource Clock) error {
	return errors.New("setting the date is not supported on js/wasm")
}
"""

src, out, names = sys.argv[1], sys.argv[2], sys.argv[3:]
os.makedirs(out, exist_ok=True)

shutil.copyfile(os.path.join(src, "LICENSE"), os.path.join(out, "LICENSE"))

FUNCS = "Parse|Bool|Int|String|Duration|BoolVar|IntVar|StringVar|DurationVar|Int64|Int64Var|Uint|UintVar|Uint64|Uint64Var|Float64|Float64Var|Var|Func|BoolFunc|TextVar|Args|Arg|NArg|NFlag|Usage|PrintDefaults|Lookup|Set|Visit|VisitAll|CommandLine|Parsed"
for n in names:
    d = Path(src, "cmds/core", n)
    od = Path(out, n)
    od.mkdir(exist_ok=True)
    pkg = "u" + n
    for f in sorted(d.iterdir()):
        if not f.name.endswith(".go") or f.name.endswith("_test.go"):
            continue
        s = f.read_text()
        s = re.sub(r"(?m)^package main\b", "package " + pkg, s, count=1)
        s = re.sub(r"(?m)^func main\(\)", "func Main()", s)
        s = re.sub(r"\bflag\.(" + FUNCS + r")\b", r"cmdFlags.\1", s)
        s = s.replace("cmdFlags.Parse()", "cmdFlags.Parse(os.Args[1:])")
        s = s.replace("cmdFlags.CommandLine", "cmdFlags")
        if "cmdFlags." in s and not re.search(r'"os"', s):
            s = re.sub(r"(?m)^import \(", 'import (\n\t"os"', s, count=1)
        if re.search(r'\t"flag"\n|import "flag"', s) and not re.search(r"\bflag\.", s):
            s += "\nvar _ = flag.ErrHelp\n"
        Path(od, f.name).write_text(s)
    Path(od, "zz_flags.go").write_text(
        f'package {pkg}\n\nimport (\n\t"flag"\n\t"os"\n)\n\nvar cmdFlags = flag.NewFlagSet(os.Args[0], flag.ExitOnError)\n\nvar _ = flag.ErrHelp\n'
    )

# js/wasm has no settimeofday: date can print the time but not set it.
dd = Path(out, "date")
if dd.is_dir():
    f = Path(dd, "date_unix.go")
    f.write_text(
        f.read_text().replace(
            "//go:build !plan9 && !windows\n",
            "//go:build !plan9 && !windows && !js\n"
        )
    )
    Path(dd, "date_js.go").write_text(DATE_JS_GO)
