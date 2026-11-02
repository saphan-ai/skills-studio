// Command skills checks skill directories against the skill format v1.
//
//	skills verify <dir> [--signature <file> --pubkey <file>]
//
// Exit codes: 0 the skill passes, 1 refused with a named rule, 2 usage.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/saphan-ai/skills-studio/internal/skillfmt"
)

const usage = `usage: skills verify <dir> [--signature <file> --pubkey <file>]
       skills check <dir> [--json] [--max-domains N]    (see: skills check -h)

  verify <dir>   check the skill directory against skill format v1 and print
                 its canonical hash
  --signature    an ed25519 signature over the canonical hash
  --pubkey       the ed25519 public key that signature is checked against

exit codes: 0 pass, 1 refused with a named rule, 2 usage
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
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
	rest := args[1:]
	for i := 0; i < len(rest); i++ {
		switch a := rest[i]; a {
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
	if !res.Passed() {
		for _, v := range res.Violations {
			fmt.Fprintf(stderr, "refused %s: %s: %s\n", v.Rule, v.Path, v.Message)
		}
		return 1
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
	fmt.Fprintf(stdout, "skill: %s\ncanonical-sha256: %s\nformat: skill-format-v1 ok\n%s\n", res.Name, res.Hash, signatureLine)
	return 0
}

func usageError(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "skills: %s\n\n%s", msg, usage)
	return 2
}
