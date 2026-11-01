package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/saphan-ai/skills-studio/internal/skillcheck"
)

const checkUsage = `usage: skills check <dir> [--json] [--max-domains N]

  check <dir>    run the static checks over the skill directory; no model, no
                 network. Each finding names a rule, a file:line, a severity
                 (refuse or flag) and one sentence. Every run ends with remark:
                 lines saying what a text scan cannot see.
  --json         print one JSON object instead of text (the remarks are its
                 "remarks" array)
  --max-domains  the most comma-separated alternatives one description sentence
                 may list before it is flagged (default 10)

exit codes: 0 no finding, 1 at least one refuse, 3 only flags, 2 usage
`

// runCheck implements "skills check". Findings go to stdout: they are the
// product of the command, not a diagnostic about it.
func runCheck(args []string, stdout, stderr io.Writer) int {
	var dir string
	asJSON := false
	cfg := skillcheck.Config{}
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "-h", "--help":
			fmt.Fprint(stdout, checkUsage)
			return 0
		case "--json":
			asJSON = true
		case "--max-domains":
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
		}
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

func checkUsageError(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "skills: %s\n\n%s", msg, checkUsage)
	return 2
}
