package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saphan-ai/skills-studio/internal/skillcheck"
)

func canary(rule string) string {
	return filepath.Join("..", "..", "internal", "skillcheck", "testdata", "canaries", rule)
}

func runCmd(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestCheckExitCodes(t *testing.T) {
	clean := filepath.Join("..", "..", "examples", "valid", "minimal")
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"no finding", []string{"check", clean}, 0},
		{"refuse", []string{"check", canary("sc-url-fetch")}, 1},
		{"refuse beats flag", []string{"check", canary("sc-hidden-text")}, 1},
		{"only flags", []string{"check", canary("sc-url")}, 3},
		{"no directory", []string{"check"}, 2},
		{"two directories", []string{"check", clean, clean}, 2},
		{"unknown flag", []string{"check", "--nope", clean}, 2},
		{"missing directory", []string{"check", filepath.Join(t.TempDir(), "absent")}, 2},
		{"bad number", []string{"check", clean, "--max-domains", "x"}, 2},
		{"zero limit", []string{"check", clean, "--max-domains", "0"}, 2},
		{"flag without value", []string{"check", clean, "--max-domains"}, 2},
	}
	for _, c := range cases {
		if got, _, _ := runCmd(c.args...); got != c.want {
			t.Errorf("%s: exit %d, want %d", c.name, got, c.want)
		}
	}
}

func TestCheckTextEndsWithEveryRemark(t *testing.T) {
	for _, dir := range []string{filepath.Join("..", "..", "examples", "valid", "minimal"), canary("sc-url")} {
		_, out, _ := runCmd("check", dir)
		lines := strings.Split(strings.TrimSpace(out), "\n")
		var remarks []string
		for _, l := range lines {
			if strings.HasPrefix(l, "remark: ") {
				remarks = append(remarks, strings.TrimPrefix(l, "remark: "))
			}
		}
		for _, want := range skillcheck.FixedRemarks {
			found := false
			for _, got := range remarks {
				found = found || got == want
			}
			if !found {
				t.Errorf("%s: output lacks the remark %q", dir, want)
			}
		}
		if !strings.HasPrefix(lines[len(lines)-1], "remark: ") {
			t.Errorf("%s: the output does not end with a remark line: %q", dir, lines[len(lines)-1])
		}
	}
}

func TestCheckFindingLineShape(t *testing.T) {
	_, out, _ := runCmd("check", canary("sc-credential"))
	if !strings.Contains(out, "flag SC-CREDENTIAL SKILL.md:10: ") {
		t.Errorf("output lacks a finding line of the form severity rule file:line: sentence:\n%s", out)
	}
}

func TestCheckJSON(t *testing.T) {
	code, out, _ := runCmd("check", "--json", canary("sc-url-fetch"))
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	var rep skillcheck.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("not one JSON object: %v\n%s", err, out)
	}
	if len(rep.Findings) != 1 || rep.Findings[0].Rule != skillcheck.RuleURLFetch || rep.Findings[0].Severity != skillcheck.Refuse {
		t.Errorf("findings %+v", rep.Findings)
	}
	if len(rep.Remarks) < len(skillcheck.FixedRemarks) {
		t.Errorf("remarks %v", rep.Remarks)
	}
	// A clean run is an empty list, never null.
	_, out, _ = runCmd("check", "--json", filepath.Join("..", "..", "examples", "valid", "minimal"))
	if !strings.Contains(out, `"findings": []`) {
		t.Errorf("a clean run does not print an empty findings list:\n%s", out)
	}
}

func TestCheckMaxDomainsFlagReachesTheRule(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "valid", "read-files")
	if code, _, _ := runCmd("check", dir); code != 0 {
		t.Fatalf("read-files under the default limit: exit %d", code)
	}
	if code, out, _ := runCmd("check", dir, "--max-domains", "2"); code != 3 || !strings.Contains(out, "SC-DESCRIPTION-OVERCLAIM") {
		t.Errorf("read-files under a limit of 2: exit %d\n%s", code, out)
	}
}

func TestCheckHelp(t *testing.T) {
	if code, out, _ := runCmd("check", "-h"); code != 0 || !strings.Contains(out, "skills check <dir>") {
		t.Errorf("exit %d\n%s", code, out)
	}
}
