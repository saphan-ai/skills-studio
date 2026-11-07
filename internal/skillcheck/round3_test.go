package skillcheck

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// More edge cases of skills check.

func unscannedAt(r Report, path string) *Finding {
	for i, f := range r.Findings {
		if f.Rule == RuleUnscanned && f.Path == path {
			return &r.Findings[i]
		}
	}
	return nil
}

// Front matter outside the subset is read by no rule, so it is a finding.
func TestFrontMatterOutsideTheSubsetIsAFinding(t *testing.T) {
	jsonEsc := bs + "u200B" + bs + "u202E"
	cases := []struct {
		name, front, control string
		wantRule             string // the rule that fires on the control
	}{
		{"P1 list beside escape",
			"description: \"Formats tables." + jsonEsc + "\"\ntags: [tables, markdown]\n",
			"description: \"Formats tables." + jsonEsc + "\"\ntags: \"tables\"\n", RuleHiddenText},
		{"P2 allowed-tools list",
			okDesc + "allowed-tools: [Bash, WebFetch]\n",
			okDesc + "allowed-tools: Bash WebFetch\n", RuleAllowedTools},
		{"P17 capital-U escape",
			"description: \"Formats tables." + bs + "U000E0041" + bs + "U000E0042\"\n",
			"description: \"Formats tables." + bs + "uDB40" + bs + "uDC41\"\n", RuleHiddenText},
	}
	for _, c := range cases {
		rep := mustCheck(t, skillWith(t, c.front, "x\n", nil))
		f := unscannedAt(rep, "SKILL.md")
		if f == nil {
			t.Errorf("%s: no SC-UNSCANNED at SKILL.md, the run would exit 0: %+v", c.name, rep.Findings)
			continue
		}
		if !strings.Contains(f.Message, "front matter") || !strings.Contains(f.Message, "decoded") {
			t.Errorf("%s: finding does not say the decoded values and front matter rules were not read: %q", c.name, f.Message)
		}
		ctl := mustCheck(t, skillWith(t, c.control, "x\n", nil))
		if unscannedAt(ctl, "SKILL.md") != nil {
			t.Errorf("%s control: SC-UNSCANNED appeared on readable front matter", c.name)
		}
		if !hasRule(ctl, c.wantRule) {
			t.Errorf("%s control: %s did not fire: %+v", c.name, c.wantRule, ctl.Findings)
		}
	}
}

func TestFrontMatterRemarkNamesAllFourRules(t *testing.T) {
	rep := mustCheck(t, skillWith(t, okDesc+"tags: [a, b]\n", "x\n", nil))
	var line string
	for _, r := range rep.Remarks {
		if strings.Contains(r, "front matter could not be read") {
			line = r
		}
	}
	if line == "" {
		t.Fatalf("no front matter remark: %v", rep.Remarks)
	}
	for _, r := range []string{RuleAllowedTools, RuleDescriptionOverrun, RuleHiddenText, RuleOverridePhrase} {
		if !strings.Contains(line, r) {
			t.Errorf("the remark does not name %s: %s", r, line)
		}
	}
}

// O1: a special entry (a FIFO) is a finding.
func TestFifoIsAFinding(t *testing.T) {
	dir := skillWith(t, okDesc, "x\n", nil)
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe"), 0o644); err != nil {
		t.Skipf("no FIFOs here: %v", err)
	}
	rep := mustCheck(t, dir)
	if unscannedAt(rep, "pipe") == nil {
		t.Errorf("no SC-UNSCANNED for the FIFO: %+v", rep.Findings)
	}
}

// O3: a hidden character in a path is found.
func TestHiddenCharacterInPathIsFound(t *testing.T) {
	name := "references/a" + string(rune(0x202E)) + "gnp.md"
	rep := mustCheck(t, skillWith(t, okDesc, "x\n", map[string]string{name: "x\n"}))
	if !hasRule(rep, RuleHiddenText) {
		t.Fatalf("a bidi control in a file name was not found: %+v", rep.Findings)
	}
	ctl := mustCheck(t, skillWith(t, okDesc, "x\n", map[string]string{"references/agnp.md": "x\n"}))
	if len(ctl.Findings) != 0 {
		t.Errorf("control: %+v", ctl.Findings)
	}
}
