package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestExportWritesOnlyIntoOut(t *testing.T) {
	out := filepath.Join(t.TempDir(), "o")
	code, stdout, stderr := runCLI("export", "../../examples/valid/with-references", "--format", "agents-md", "--out", out)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(out, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	// The pieces AGENTS.md cannot carry are named, one line each.
	for _, want := range []string{"loss: assets/: ", "loss: references/: ", "loss: canonical-hash: ", "remark: "} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
	// Nothing next to the skill.
	if _, err := os.Stat("../../examples/valid/with-references/AGENTS.md"); err == nil {
		t.Error("a file was written into the skill directory")
	}
}

// A skill that verify rates valid but holds a symbolic link is refused by
// export for export's own reason, and the refusal does not call it invalid
// Agent Skills nor send the operator to a verify run that passes it.
func TestExportRefusalOfAnUnreadFileDoesNotBlameTheFormat(t *testing.T) {
	skill := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: demo\ndescription: A demo skill.\n---\nBody\n"
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("SKILL.md", filepath.Join(skill, "link.md")); err != nil {
		t.Skipf("cannot make a symbolic link here: %v", err)
	}
	if code, _, stderr := runCLI("verify", skill, "--level", "1"); code != 0 {
		t.Fatalf("the premise fails: verify --level 1 exits %d: %s", code, stderr)
	}
	out := t.TempDir()
	code, _, stderr := runCLI("export", skill, "--format", "agents-md", "--out", out)
	if code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, stderr)
	}
	if !strings.Contains(stderr, "EXPORT-UNREAD") || !strings.Contains(stderr, "export does not read links or special files") {
		t.Errorf("the refusal does not say what export could not read:\n%s", stderr)
	}
	for _, bad := range []string{"not valid Agent Skills", "skills verify"} {
		if strings.Contains(stderr, bad) {
			t.Errorf("the refusal contains %q, which a passing verify run contradicts:\n%s", bad, stderr)
		}
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Errorf("something was written: %v", entries)
	}
}

func TestExportLosslessAgentSkillsNamesNoLoss(t *testing.T) {
	out := t.TempDir()
	code, stdout, stderr := runCLI("export", "../../examples/agentskills/changelog-helper", "--format", "agentskills", "--out", out)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "losses: none") || strings.Contains(stdout, "loss: ") {
		t.Errorf("a lossless export printed a loss:\n%s", stdout)
	}
	want, _ := os.ReadFile("../../examples/agentskills/changelog-helper/scripts/count.sh")
	got, err := os.ReadFile(filepath.Join(out, "changelog-helper/scripts/count.sh"))
	if err != nil || !bytes.Equal(got, want) {
		t.Errorf("scripts/count.sh was not carried: %v", err)
	}
	src, _ := os.Stat("../../examples/agentskills/changelog-helper/scripts/count.sh")
	dst, _ := os.Stat(filepath.Join(out, "changelog-helper/scripts/count.sh"))
	if src == nil || dst == nil || src.Mode()&0o100 != dst.Mode()&0o100 {
		t.Errorf("the execute bit of scripts/count.sh differs: source %v, export %v", src, dst)
	}
}

func TestExportLevel1SkillOutsideTheProfileIsExported(t *testing.T) {
	// The agentskills example is valid Agent Skills and outside the profile.
	code, stdout, stderr := runCLI("export", "../../examples/agentskills/changelog-helper", "--format", "cursor-rules", "--out", t.TempDir())
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if strings.Contains(stdout, "canonical-hash") {
		t.Errorf("a level 1 skill has no canonical hash to lose:\n%s", stdout)
	}
	if !strings.Contains(stdout, "loss: scripts/: ") {
		t.Errorf("the scripts/ loss is not named:\n%s", stdout)
	}
}

func TestExportRefusesInvalidSkillByRuleName(t *testing.T) {
	out := filepath.Join(t.TempDir(), "never")
	code, _, stderr := runCLI("export", "../../examples/invalid/sf-name-pattern/Bad_Name", "--format", "agentskills", "--out", out)
	if code != 1 || !strings.Contains(stderr, "refused AS-NAME-CHARS") {
		t.Fatalf("exit %d, stderr %q; want 1 naming AS-NAME-CHARS", code, stderr)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("--out was created for a refused skill")
	}
}

func TestExportUnknownFormatListsRegisteredNames(t *testing.T) {
	code, _, stderr := runCLI("export", "../../examples/valid/minimal", "--format", "word", "--out", t.TempDir())
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	for _, n := range []string{"agentskills", "cursor-rules", "agents-md", "copilot-instructions"} {
		if !strings.Contains(stderr, n) {
			t.Errorf("stderr does not list %s: %s", n, stderr)
		}
	}
}

func TestExportOutInsideSkillRefused(t *testing.T) {
	src := t.TempDir()
	skill := filepath.Join(src, "my-skill")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: my-skill\ndescription: d\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{skill, filepath.Join(skill, "out"), filepath.Join(skill, "a", "b")} {
		if code, _, stderr := runCLI("export", skill, "--format", "agents-md", "--out", out); code != 1 || !strings.Contains(stderr, "inside it") {
			t.Errorf("--out %s: exit %d, stderr %q; want a refusal", out, code, stderr)
		}
	}
	entries, _ := os.ReadDir(skill)
	if len(entries) != 1 {
		t.Errorf("the skill directory gained files: %v", entries)
	}
	// A sibling directory is fine.
	if code, _, stderr := runCLI("export", skill, "--format", "agents-md", "--out", filepath.Join(src, "my-skill-out")); code != 0 {
		t.Errorf("a sibling --out was refused: %s", stderr)
	}
}

func TestExportNeverOverwrites(t *testing.T) {
	out := t.TempDir()
	target := filepath.Join(out, "AGENTS.md")
	if err := os.WriteFile(target, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runCLI("export", "../../examples/valid/minimal", "--format", "agents-md", "--out", out)
	if code != 1 || !strings.Contains(stderr, "already exists") {
		t.Fatalf("exit %d, stderr %q; want 1 naming the existing file", code, stderr)
	}
	if got, _ := os.ReadFile(target); string(got) != "mine\n" {
		t.Errorf("the existing file was changed: %q", got)
	}
}

func TestExportUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"export"},
		{"export", "../../examples/valid/minimal"},
		{"export", "../../examples/valid/minimal", "--format", "agents-md"},
		{"export", "../../examples/valid/minimal", "--out", "x"},
		{"export", "../../examples/valid/minimal", "--format"},
		{"export", "a", "b", "--format", "agents-md", "--out", "x"},
		{"export", "--bogus"},
	} {
		if code, _, _ := runCLI(args...); code != 2 {
			t.Errorf("%v: exit %d, want 2", args, code)
		}
	}
}
