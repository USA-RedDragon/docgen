"""Copies u-root command main packages into bb/ucmd/<name> as importable packages.

Each copy has package u<name>, func Main in place of main, and a flag set of its
own in place of flag.CommandLine, so that one binary can hold all of them.
Usage: python3 -I vendor-uroot.py <u-root dir> <out dir> <name>...

After a change of the u-root version in go.mod, run it again from playground/bb:

  python3 -I vendor-uroot.py "$(go list -m -f "{{.Dir}}" github.com/u-root/u-root)" ucmd $(cd ucmd && ls -d */ | tr -d /)

It also copies the LICENSE file of u-root into the out dir.

build.sh stops when ucmd differs from its output.
"""
import os, re, shutil, sys

src, out, names = sys.argv[1], sys.argv[2], sys.argv[3:]
os.makedirs(out, exist_ok=True)
shutil.copyfile(os.path.join(src, "LICENSE"), os.path.join(out, "LICENSE"))
FUNCS = "Parse|Bool|Int|String|Duration|BoolVar|IntVar|StringVar|DurationVar|Int64|Int64Var|Uint|UintVar|Uint64|Uint64Var|Float64|Float64Var|Var|Func|BoolFunc|TextVar|Args|Arg|NArg|NFlag|Usage|PrintDefaults|Lookup|Set|Visit|VisitAll|CommandLine|Parsed"
for n in names:
    d = os.path.join(src, "cmds/core", n)
    od = os.path.join(out, n)
    os.makedirs(od, exist_ok=True)
    pkg = "u" + n
    for f in sorted(os.listdir(d)):
        if not f.endswith(".go") or f.endswith("_test.go"):
            continue
        s = open(os.path.join(d, f)).read()
        s = re.sub(r"(?m)^package main\b", "package " + pkg, s, count=1)
        s = re.sub(r"(?m)^func main\(\)", "func Main()", s)
        s = re.sub(r"\bflag\.(" + FUNCS + r")\b", r"cmdFlags.\1", s)
        s = s.replace("cmdFlags.Parse()", "cmdFlags.Parse(os.Args[1:])")
        s = s.replace("cmdFlags.CommandLine", "cmdFlags")
        if "cmdFlags." in s and not re.search(r'"os"', s):
            s = re.sub(r"(?m)^import \(", 'import (\n\t"os"', s, count=1)
        if re.search(r'\t"flag"\n|import "flag"', s) and not re.search(r"\bflag\.", s):
            s += "\nvar _ = flag.ErrHelp\n"
        open(os.path.join(od, f), "w").write(s)
    open(os.path.join(od, "zz_flags.go"), "w").write(
        f'package {pkg}\n\nimport (\n\t"flag"\n\t"os"\n)\n\nvar cmdFlags = flag.NewFlagSet(os.Args[0], flag.ExitOnError)\n\nvar _ = flag.ErrHelp\n')

# js/wasm has no settimeofday: date can print the time but not set it.
dd = os.path.join(out, "date")
if os.path.isdir(dd):
    f = os.path.join(dd, "date_unix.go")
    s = open(f).read().replace("//go:build !plan9 && !windows\n", "//go:build !plan9 && !windows && !js\n")
    open(f, "w").write(s)
    open(os.path.join(dd, "date_js.go"), "w").write(
        "//go:build js\n\npackage udate\n\nimport (\n\t\"errors\"\n\t\"time\"\n)\n\n"
        "func setDate(d string, z *time.Location, clocksource Clock) error {\n\treturn errors.New(\"setting the date is not supported on js/wasm\")\n}\n")
