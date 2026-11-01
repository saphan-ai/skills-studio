package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/saphan-ai/skills-studio/internal/skillcheck"
)

var checkUsage = fmt.Sprintf(`usage: skills check <dir> [--json] [--max-domains N]

  check <dir>    run the static checks over the skill directory; no model, no
                 network. Each finding names a rule, a file:line, a severity
                 (refuse or flag) and one sentence. Every run that checks ends
                 with remark: lines saying what a text scan cannot see. An entry
                 the text rules could not read is itself a finding (SC-UNSCANNED).
  --json         print one JSON object instead of text (the remarks are its
                 "remarks" array)
  --max-domains  the most comma-separated alternatives one description sentence
                 may list before it is flagged (default 10)
  --help         print this text; it takes no other argument

limits: a file is read up to %d MiB, a tree up to %d entries and %d levels deep,
and %d MiB in all; a tree past a limit is not checked.

exit codes: 0 no finding, 1 at least one refuse, 3 only flags, 2 usage or a tree
over a limit: no check ran, so nothing was found and nothing was cleared (the
remarks are printed to stderr)
`, skillcheck.MaxFileBytes>>20, skillcheck.MaxEntries, skillcheck.MaxDepth, skillcheck.MaxTotalBytes>>20)

// runCheck implements "skills check". Findings go to stdout: they are the
// product of the command, not a diagnostic about it.
func runCheck(args []string, stdout, stderr io.Writer) int {
	var dir string
	asJSON, help, other := false, false, false
	cfg := skillcheck.Config{}
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "-h", "--help":
			help = true
		case "--json":
			asJSON, other = true, true
		case "--max-domains":
			other = true
			if i+1 >= len(args) {
				return checkUsageError(stderr, a+" needs a number")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				return checkUsageError(stderr, a+" needs a whole number of at least 1")
			}
			cfg.MaxDomains = n
		default:
			if len(a) > 0 && a[0] == '-' {
				return checkUsageError(stderr, "unknown flag "+a)
			}
			if dir != "" {
				return checkUsageError(stderr, "one skill directory at a time")
			}
			dir = a
			other = true
		}
	}
	if help {
		// Help prints the usage and checks nothing, so it takes no directory:
		// a command line that names one and asks for help would otherwise
		// look like a clean check.
		if other {
			return checkUsageError(stderr, "--help takes no other argument; it checks nothing")
		}
		fmt.Fprint(stdout, checkUsage)
		return 0
	}
	if dir == "" {
		return checkUsageError(stderr, "check needs a skill directory")
	}
	rep, err := skillcheck.Check(dir, cfg)
	if err != nil {
		return checkUsageError(stderr, err.Error())
	}

	if asJSON {
		if rep.Findings == nil {
			rep.Findings = []skillcheck.Finding{}
		}
		out, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "skills: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "%s\n", out)
	} else {
		for _, f := range rep.Findings {
			fmt.Fprintf(stdout, "%s %s %s:%d: %s\n", f.Severity, f.Rule, f.Path, f.Line, f.Message)
		}
		fmt.Fprintf(stdout, "checked %d text file(s): %d finding(s)\n", rep.Files, len(rep.Findings))
		for _, r := range rep.Remarks {
			fmt.Fprintf(stdout, "remark: %s\n", r)
		}
	}
	switch {
	case rep.Refused():
		return 1
	case len(rep.Findings) > 0:
		return 3
	}
	return 0
}

// checkUsageError prints why, the usage and the remarks. Exit 2 means no check
// ran, and the remarks say so, so a script that reads only stderr is not left
// believing a clean run.
func checkUsageError(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "skills: %s\n\n%s\n", msg, checkUsage)
	fmt.Fprintln(stderr, "remark: no check ran, so no finding was made and nothing was cleared; this is not a clean result")
	for _, r := range skillcheck.FixedRemarks {
		fmt.Fprintf(stderr, "remark: %s\n", r)
	}
	return 2
}
