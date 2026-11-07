// Command skills checks skill directories against the skill format v1.
//
//	skills verify <dir> [--level 1|2] [--signature <file> --pubkey <file>]
//
// Two levels: 1 is valid Agent Skills, 2 is the Saphan profile (which includes
// level 1). Exit codes: 0 the level reached meets the one asked for (level 2
// when no --level is given), 1 refused with a named rule, 2 usage, 3 valid
// Agent Skills but outside the profile (no --level given).
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/saphan-ai/skills-studio/internal/skillfmt"
)

// exitLevel1Only is the exit code of a skill that is valid Agent Skills and
// outside the Saphan profile. It is not a refusal of the skill.
const exitLevel1Only = 3

const usage = `usage: skills verify <dir> [--level 1|2] [--signature <file> --pubkey <file>]
       skills check <dir> [--json] [--max-domains N]    (see: skills check -h)

  verify <dir>   read the skill directory at two levels and print the level
                 reached: level 1, valid Agent Skills (the rules of the Agent
                 Skills specification page); level 2, the Saphan profile (level 1
                 plus the rules of skill format v1). A level 1 skill is printed
                 with every profile rule that keeps it out of level 2. At level 2
                 the canonical hash is printed.
  --level        the lowest level that is acceptable: 1 or 2. Without it, the
                 exit code tells the three outcomes apart.
  --signature    an ed25519 signature over the canonical hash (level 2 only)
  --pubkey       the ed25519 public key that signature is checked against

  version        print the version this binary was built as

exit codes: 0 level 2 reached (or, with --level 1, level 1 or 2)
            1 refused with a named rule: not valid Agent Skills (or below --level)
            2 usage
            3 valid Agent Skills (level 1), outside the Saphan profile
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "version" || args[0] == "--version") {
		fmt.Fprintln(stdout, "skills "+version)
		return 0
	}
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if len(args) > 0 && args[0] == "check" {
		return runCheck(args[1:], stdout, stderr)
	}
	if len(args) == 0 || args[0] != "verify" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	var dir, sigPath, keyPath string
	want := 0 // --level: 0 reports the level reached; 1 or 2 is a floor
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		switch a := rest[i]; a {
		case "--level":
			if i+1 >= len(rest) {
				return usageError(stderr, "--level needs 1 or 2")
			}
			i++
			switch rest[i] {
			case "1":
				want = 1
			case "2":
				want = 2
			default:
				return usageError(stderr, "--level needs 1 or 2")
			}
		case "--signature", "--pubkey":
			if i+1 >= len(rest) {
				return usageError(stderr, a+" needs a file name")
			}
			i++
			if a == "--signature" {
				sigPath = rest[i]
			} else {
				keyPath = rest[i]
			}
		default:
			if len(a) > 0 && a[0] == '-' {
				return usageError(stderr, "unknown flag "+a)
			}
			if dir != "" {
				return usageError(stderr, "one skill directory at a time")
			}
			dir = a
		}
	}
	if dir == "" {
		return usageError(stderr, "verify needs a skill directory")
	}
	if (sigPath == "") != (keyPath == "") {
		return usageError(stderr, "--signature and --pubkey go together")
	}

	res, err := skillfmt.Verify(dir)
	if err != nil {
		return usageError(stderr, err.Error())
	}
	if res.Level == 0 || res.Level < want {
		// Not valid Agent Skills, or below the level --level asked for.
		for _, v := range res.Level1 {
			fmt.Fprintf(stderr, "refused %s: %s: %s\n", v.Rule, v.Path, v.Message)
		}
		for _, v := range res.Violations {
			fmt.Fprintf(stderr, "refused %s: %s: %s\n", v.Rule, v.Path, v.Message)
		}
		if len(res.NotEvaluated) > 0 {
			fmt.Fprintf(stderr, "not evaluated: %s (the file is too large to read whole)\n", strings.Join(res.NotEvaluated, " "))
		}
		return 1
	}
	if res.Level == 1 {
		fmt.Fprintf(stdout, "skill: %s\nlevel: 1 (valid Agent Skills; the Saphan profile is not met)\n", res.Name)
		for _, v := range res.Violations {
			fmt.Fprintf(stdout, "outside-profile %s: %s: %s\n", v.Rule, v.Path, v.Message)
		}
		if len(res.NotEvaluated) > 0 {
			fmt.Fprintf(stdout, "not-evaluated: %s (the file is too large to read whole)\n", strings.Join(res.NotEvaluated, " "))
		}
		fmt.Fprintf(stdout, "format: skill-format-v1 not met\n")
		if sigPath != "" {
			fmt.Fprintf(stdout, "signature: not checked (a signature is over the canonical hash, which exists at level 2 only)\n")
		}
		fmt.Fprintf(stdout, "remark: level 1 holds the skill to the rules of the Agent Skills page as read on %s and to nothing else; it says nothing about what the skill tells an agent to do\n", skillfmt.AgentSkillsPageRead)
		if want == 1 {
			return 0
		}
		return exitLevel1Only
	}

	signatureLine := "signature: not checked"
	if sigPath != "" {
		sig, err := os.ReadFile(sigPath)
		if err != nil {
			return usageError(stderr, err.Error())
		}
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return usageError(stderr, err.Error())
		}
		if v := skillfmt.CheckSignature(res.Hash, sig, key); v != nil {
			fmt.Fprintf(stderr, "refused %s: %s\n", v.Rule, v.Message)
			return 1
		}
		signatureLine = "signature: ed25519 valid over canonical-sha256 (nothing else is claimed)"
	}
	fmt.Fprintf(stdout, "skill: %s\nlevel: 2 (valid Agent Skills, and the Saphan profile is met)\ncanonical-sha256: %s\nformat: skill-format-v1 ok\n%s\n", res.Name, res.Hash, signatureLine)
	return 0
}

func usageError(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "skills: %s\n\n%s", msg, usage)
	return 2
}
