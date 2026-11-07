package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Through the command: a level 1 skill whose fence the subset reader
// refuses still prints its name and every outside-profile rule.
func TestLevel1PrintsNameAndEveryRuleForABlankFence(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "c1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "SKILL.md", []byte("--- \nname: c1\ndescription: d\nversion: 3\n---\n"))
	code, out, errOut := do("verify", dir)
	if code != 3 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	for _, want := range []string{"skill: c1\n", "outside-profile SF-FRONTMATTER-MISSING", "outside-profile SF-KEY-UNKNOWN"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, out)
		}
	}
}
