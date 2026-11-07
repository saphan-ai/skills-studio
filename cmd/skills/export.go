package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/saphan-ai/skills-studio/exportfmt"
)

const exportUsage = `usage: skills export <dir> --format <name> --out <dir>

  export <dir>  read the skill directory (it must be valid Agent Skills, level 1
                of skills verify) and write it in another tool's format
  --format      the format: ` + "%s" + `
  --out         the directory the files are written into, and nowhere else; it is
                created if absent; a file that already exists there is never
                overwritten

Every piece of the skill the format cannot carry is printed as one "loss:" line.
exit codes: 0 exported, 1 refused (the skill, the format or the output), 2 usage
`

func runExport(args []string, stdout, stderr io.Writer) int {
	return runExportWith(exportfmt.Default(), args, stdout, stderr)
}

func runExportWith(reg *exportfmt.Registry, args []string, stdout, stderr io.Writer) int {
	usage := fmt.Sprintf(exportUsage, strings.Join(reg.Names(), ", "))
	fail := func(msg string) int {
		fmt.Fprintf(stderr, "skills export: %s\n\n%s", msg, usage)
		return 2
	}
	var dir, format, out string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "-h", "--help":
			fmt.Fprint(stdout, usage)
			return 0
		case "--format", "--out":
			if i+1 >= len(args) {
				return fail(a + " needs a value")
			}
			i++
			if a == "--format" {
				format = args[i]
			} else {
				out = args[i]
			}
		default:
			if strings.HasPrefix(a, "-") {
				return fail("unknown flag " + a)
			}
			if dir != "" {
				return fail("one skill directory at a time")
			}
			dir = a
		}
	}
	switch {
	case dir == "":
		return fail("export needs a skill directory")
	case format == "":
		return fail("export needs --format; registered formats: " + strings.Join(reg.Names(), ", "))
	case out == "":
		return fail("export needs --out, the directory the files are written into")
	}
	if _, ok := reg.Get(format); !ok {
		fmt.Fprintf(stderr, "refused: %s\n", &exportfmt.UnknownFormatError{Name: format, Registered: reg.Names()})
		return 2
	}
	if inside(dir, out) {
		fmt.Fprintf(stderr, "refused: --out %s is the skill directory or inside it; an export writes beside nothing and into nothing of the skill\n", out)
		return 1
	}

	skill, err := exportfmt.Load(dir)
	if err != nil {
		var re *exportfmt.RefusedError
		if errors.As(err, &re) {
			for _, v := range re.Violations {
				fmt.Fprintf(stderr, "refused %s: %s: %s\n", v.Rule, v.Path, v.Message)
			}
			if re.Unread() {
				fmt.Fprint(stderr, "nothing was written: verify reads this skill; export does not read links or special files, or files over its size bounds\n")
			} else {
				fmt.Fprintf(stderr, "nothing was written: a skill that is not valid Agent Skills is not exported (see: skills verify %s --level 1)\n", dir)
			}
			return 1
		}
		fmt.Fprintf(stderr, "skills export: %v\n", err)
		return 2
	}
	res, err := reg.Export(format, skill)
	if err != nil {
		fmt.Fprintf(stderr, "refused: %v\nnothing was written\n", err)
		return 1
	}
	written, err := exportfmt.WriteFiles(out, res.Files)
	if err != nil {
		fmt.Fprintf(stderr, "refused: %v\n", err)
		if len(written) > 0 {
			fmt.Fprintf(stderr, "written before the failure: %s\n", strings.Join(written, " "))
		}
		return 1
	}
	fmt.Fprintf(stdout, "skill: %s\nformat: %s\nout: %s\n", skill.Name, format, out)
	for _, p := range written {
		fmt.Fprintf(stdout, "wrote: %s\n", p)
	}
	if len(res.Losses) == 0 {
		fmt.Fprintf(stdout, "losses: none\n")
	}
	for _, l := range res.Losses {
		fmt.Fprintf(stdout, "loss: %s\n", l)
	}
	fmt.Fprintf(stdout, "remark: a loss line names a piece of the skill the format cannot carry; \"losses: none\" says nothing was dropped from what this program read, not that the target tool will act on every field\n")
	return 0
}

// inside reports whether out is dir or lies within it, by absolute path.
func inside(dir, out string) bool {
	d, err1 := filepath.Abs(dir)
	o, err2 := filepath.Abs(out)
	if err1 != nil || err2 != nil {
		return false
	}
	if r, err := filepath.EvalSymlinks(d); err == nil {
		d = r
	}
	// out may not exist yet: resolve its nearest existing parent.
	probe, rest := o, ""
	for {
		if r, err := filepath.EvalSymlinks(probe); err == nil {
			o = filepath.Join(r, rest)
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			break
		}
		rest = filepath.Join(filepath.Base(probe), rest)
		probe = parent
	}
	rel, err := filepath.Rel(d, o)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
