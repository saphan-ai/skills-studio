package skillfmt

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// agentSkillsExamples is each skill under examples/agentskills and the
// profile rules that keep it out of level 2: all of them, not the first.
var agentSkillsExamples = map[string][]string{
	"changelog-helper": {RuleFileScript, RuleFileUnlisted, RuleFrontMatterSyntax, RuleKeyUnknown},
	"csv-column-stats": {RuleFileScript},
	"αβγ-δεζ":          {RuleFrontMatterSyntax, RuleNamePattern},
	"commit-reviewer":  {RuleFileUnlisted, RuleFrontMatterSyntax, RuleKeyUnknown},
}

func TestAgentSkillsExamplesReachLevel1WithExactlyTheirProfileRules(t *testing.T) {
	onDisk := subdirs(t, repoPath("examples", "agentskills"))
	if len(onDisk) < 3 {
		t.Fatalf("expected at least 3 examples, found %v", onDisk)
	}
	for _, n := range onDisk {
		if _, ok := agentSkillsExamples[n]; !ok {
			t.Errorf("examples/agentskills/%s has no expectation in the test", n)
		}
	}
	for n, want := range agentSkillsExamples {
		t.Run(n, func(t *testing.T) {
			res, err := Verify(repoPath("examples", "agentskills", n))
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Level1) != 0 {
				t.Fatalf("not valid Agent Skills: %+v", res.Level1)
			}
			if res.Level != 1 || res.Passed() || res.Hash != "" {
				t.Fatalf("level %d, hash %q; want level 1 and no hash", res.Level, res.Hash)
			}
			if got := rulesOf(res.Violations); !reflect.DeepEqual(got, want) {
				t.Fatalf("profile rules broken %v, want exactly %v", got, want)
			}
		})
	}
}

func TestExistingValidExamplesReachLevel2(t *testing.T) {
	for _, n := range subdirs(t, repoPath("examples", "valid")) {
		res, err := Verify(repoPath("examples", "valid", n))
		if err != nil {
			t.Fatal(err)
		}
		if res.Level != 2 || len(res.Level1) != 0 || len(res.Violations) != 0 || res.Hash == "" {
			t.Errorf("%s: level %d, level 1 %+v, profile %+v", n, res.Level, res.Level1, res.Violations)
		}
	}
}

// Which of the invalid examples are still valid Agent Skills. The profile rule
// each is refused by is held by TestInvalidExamplesRefusedByExactlyTheirRule.
func TestInvalidExamplesLevel(t *testing.T) {
	level1 := map[string]bool{
		"sf-file-script": true, "sf-file-unlisted": true, "sf-key-unknown": true,
		"sf-saphan-default": true, "sf-saphan-origin": true, "sf-text-encoding": true,
		"sf-skillmd-size": true, "sf-frontmatter-syntax": true,
		// license is not typed by the Agent Skills page: a number there is not a level 1 fault.
		"sf-field-type": true,
	}
	for _, ruleDir := range subdirs(t, repoPath("examples", "invalid")) {
		kids := subdirs(t, repoPath("examples", "invalid", ruleDir))
		res, err := Verify(repoPath("examples", "invalid", ruleDir, kids[0]))
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if level1[ruleDir] {
			want = 1
		}
		if res.Level != want {
			t.Errorf("%s: level %d, want %d (level 1 refusals %+v)", ruleDir, res.Level, want, res.Level1)
		}
	}
}

// level1Cases mutate exactly one field of a valid skill (law #27: the negative
// control names the field it mutates). A nil rule means the skill is valid
// Agent Skills.
func TestLevel1Rules(t *testing.T) {
	long := func(n int) string { return strings.Repeat("é", n) }
	cases := []struct {
		name, dir, front string
		rule             string // "" = valid Agent Skills
	}{
		{"baseline", "demo", "name: demo\ndescription: d\n", ""},
		// name
		{"name absent", "demo", "description: d\n", RuleASNameRequired},
		{"name null is absent", "demo", "name:\ndescription: d\n", RuleASNameRequired},
		{"name a number", "demo", "name: 12\ndescription: d\n", RuleASFieldType},
		{"name empty", "demo", "name: \"\"\ndescription: d\n", RuleASNameDirMismatch + "," + RuleASNameLength},
		{"name 64 characters", long(64), "name: " + long(64) + "\ndescription: d\n", ""},
		{"name 65 characters", long(65), "name: " + long(65) + "\ndescription: d\n", RuleASNameLength},
		{"name upper case", "Demo", "name: Demo\ndescription: d\n", RuleASNameChars},
		{"name underscore", "de_mo", "name: de_mo\ndescription: d\n", RuleASNameChars},
		{"name with space", "de mo", "name: \"de mo\"\ndescription: d\n", RuleASNameChars},
		{"name unicode lowercase", "αβγ", "name: αβγ\ndescription: d\n", ""},
		{"name digits", "7up", "name: \"7up\"\ndescription: d\n", ""},
		{"name starts with hyphen", "-demo", "name: \"-demo\"\ndescription: d\n", RuleASNameHyphen},
		{"name ends with hyphen", "demo-", "name: demo-\ndescription: d\n", RuleASNameHyphen},
		{"name double hyphen", "de--mo", "name: de--mo\ndescription: d\n", RuleASNameHyphen},
		{"name differs from directory", "other", "name: demo\ndescription: d\n", RuleASNameDirMismatch},
		// description
		{"description absent", "demo", "name: demo\n", RuleASDescRequired},
		{"description a list", "demo", "name: demo\ndescription: [a, b]\n", RuleASFieldType},
		{"description a date is a string", "demo", "name: demo\ndescription: 2026-11-02\n", ""},
		{"description yes is a string", "demo", "name: demo\ndescription: yes\n", ""},
		{"description true is a boolean", "demo", "name: demo\ndescription: true\n", RuleASFieldType},
		{"description empty", "demo", "name: demo\ndescription: \"\"\n", RuleASDescLength},
		{"description 1024 characters", "demo", "name: demo\ndescription: " + long(1024) + "\n", ""},
		{"description 1025 characters", "demo", "name: demo\ndescription: " + long(1025) + "\n", RuleASDescLength},
		{"description folded over lines", "demo", "name: demo\ndescription: >-\n  one\n  two\n", ""},
		{"description literal block", "demo", "name: demo\ndescription: |\n  one\n  two\n", ""},
		// optional fields
		{"license any text", "demo", "name: demo\ndescription: d\nlicense: " + long(5000) + "\n", ""},
		{"license a number is not typed by the page", "demo", "name: demo\ndescription: d\nlicense: 3\n", ""},
		{"compatibility 500", "demo", "name: demo\ndescription: d\ncompatibility: " + long(500) + "\n", ""},
		{"compatibility 501", "demo", "name: demo\ndescription: d\ncompatibility: " + long(501) + "\n", RuleASCompatLength},
		{"compatibility empty", "demo", "name: demo\ndescription: d\ncompatibility: \"\"\n", RuleASCompatLength},
		{"compatibility a number", "demo", "name: demo\ndescription: d\ncompatibility: 3\n", RuleASFieldType},
		{"allowed-tools a string", "demo", "name: demo\ndescription: d\nallowed-tools: Read Grep\n", ""},
		{"allowed-tools a list", "demo", "name: demo\ndescription: d\nallowed-tools: [Read, Grep]\n", RuleASFieldType},
		{"metadata strings", "demo", "name: demo\ndescription: d\nmetadata:\n  a: b\n  v: \"1\"\n", ""},
		{"metadata not a map", "demo", "name: demo\ndescription: d\nmetadata: x\n", RuleASMetadataType},
		{"metadata a list", "demo", "name: demo\ndescription: d\nmetadata: [a]\n", RuleASMetadataType},
		{"metadata value a number", "demo", "name: demo\ndescription: d\nmetadata:\n  v: 1\n", RuleASMetadataValueType},
		{"metadata value nested", "demo", "name: demo\ndescription: d\nmetadata:\n  v:\n    w: x\n", RuleASMetadataValueType},
		{"metadata flow map", "demo", "name: demo\ndescription: d\nmetadata: {a: b}\n", ""},
		// the choice for unknown top-level keys: the page is silent, level 1 accepts
		{"unknown key", "demo", "name: demo\ndescription: d\nversion: 3\nargument-hint: x\n", ""},
		// YAML as a full reader reads it
		{"comment, anchor and alias", "demo", "# top\nname: demo\ndescription: &d text\nmetadata:\n  a: *d\n", ""},
		{"duplicate key", "demo", "name: demo\nname: demo\ndescription: d\n", RuleASFrontMatterYAML},
		{"not YAML", "demo", "name: demo\ndescription: d\n  bad: indent\n", RuleASFrontMatterYAML},
		{"a list at top", "demo", "- a\n- b\n", RuleASFrontMatterYAML},
		{"a scalar at top", "demo", "just text\n", RuleASFrontMatterYAML},
		{"empty front matter", "demo", "", RuleASDescRequired + "," + RuleASNameRequired},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := Verify(writeSkill(t, c.dir, "---\n"+c.front+"---\n\n# Body\n"))
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Join(rulesOf(res.Level1), ","); got != c.rule {
				t.Errorf("level 1 rules %q, want %q: %+v", got, c.rule, res.Level1)
			}
			if c.rule == "" && res.Level < 1 {
				t.Errorf("level %d, want at least 1", res.Level)
			}
			if c.rule != "" && res.Level != 0 {
				t.Errorf("level %d, want 0", res.Level)
			}
		})
	}
}

func TestLevel1ReadsWhatAYAMLReaderReads(t *testing.T) {
	dir := writeSkill(t, "demo", "\xef\xbb\xbf---\r\nname: demo\r\ndescription: >-\r\n  one\r\n  two\r\n---\r\nbody\r\n")
	res, err := Verify(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Level != 1 {
		t.Fatalf("level %d: level 1 %+v profile %+v", res.Level, res.Level1, res.Violations)
	}
	if got := rulesOf(res.Violations); !reflect.DeepEqual(got, []string{RuleFrontMatterSyntax, RuleTextEncoding}) {
		t.Errorf("profile rules %v", got)
	}
}

func TestLevel1SkillMdProblems(t *testing.T) {
	mk := func(files map[string]string) string {
		dir := filepath.Join(t.TempDir(), "demo")
		for p, text := range files {
			full := filepath.Join(dir, p)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	cases := []struct {
		name string
		dir  string
		rule string
	}{
		{"no SKILL.md", mk(map[string]string{"references/a.md": "x"}), RuleASSkillMDMissing},
		{"no front matter", mk(map[string]string{"SKILL.md": "# Just text\n"}), RuleASFrontMatterMissing},
		{"front matter not closed", mk(map[string]string{"SKILL.md": "---\nname: demo\ndescription: d\n"}), RuleASFrontMatterMissing},
		{"header not closed", mk(map[string]string{"SKILL.md": HeaderOpen + "\nx\n"}), RuleASFrontMatterMissing},
		{"invalid UTF-8 in front matter", mk(map[string]string{"SKILL.md": "---\nname: demo\ndescription: \xff\n---\n"}), RuleASFrontMatterYAML},
	}
	for _, c := range cases {
		res, err := Verify(c.dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := rulesOf(res.Level1); !reflect.DeepEqual(got, []string{c.rule}) || res.Level != 0 {
			t.Errorf("%s: level %d, level 1 rules %v, want [%s]", c.name, res.Level, got, c.rule)
		}
	}
}

func TestLevel1SkillsWithOtherFilesAndDirectories(t *testing.T) {
	dir := writeSkill(t, "demo", skillText(""))
	for p, text := range map[string]string{"scripts/run.sh": "#!/bin/sh\n", "LICENSE.txt": "terms\n", "notes/x.md": "x\n"} {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Verify(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Level != 1 || len(res.Level1) != 0 {
		t.Fatalf("level %d, level 1 %+v", res.Level, res.Level1)
	}
	// every unlisted entry is named, not the first one
	var paths []string
	for _, v := range res.Violations {
		paths = append(paths, v.Rule+" "+v.Path)
	}
	want := []string{RuleFileScript + " scripts", RuleFileUnlisted + " LICENSE.txt", RuleFileUnlisted + " notes"}
	if !reflect.DeepEqual(sortedCopy(paths), sortedCopy(want)) {
		t.Errorf("profile violations %v, want %v", paths, want)
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func TestLevel1FrontMatterOverTheBoundIsUnreadNotValid(t *testing.T) {
	big := "---\nname: demo\ndescription: d\nmetadata:\n" + strings.Repeat("  k: v\n", MaxLevel1PrefixBytes) + "---\n"
	res, err := Verify(writeSkill(t, "demo", big))
	if err != nil {
		t.Fatal(err)
	}
	if got := rulesOf(res.Level1); !reflect.DeepEqual(got, []string{RuleASUnread}) || res.Level != 0 {
		t.Errorf("level %d, level 1 rules %v, want [%s]", res.Level, got, RuleASUnread)
	}
}

func TestLevel1AliasExpansionIsBounded(t *testing.T) {
	var b strings.Builder
	b.WriteString("---\nname: demo\ndescription: d\nx0: &a0 [z, z, z, z, z, z, z, z, z, z]\n")
	prev := "a0"
	for i := 1; i <= 8; i++ {
		cur := "a" + string(rune('0'+i))
		b.WriteString("x" + string(rune('0'+i)) + ": &" + cur + " [")
		b.WriteString(strings.TrimRight(strings.Repeat("*"+prev+", ", 10), ", "))
		b.WriteString("]\n")
		prev = cur
	}
	b.WriteString("---\n")
	res, err := Verify(writeSkill(t, "demo", b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if got := rulesOf(res.Level1); !reflect.DeepEqual(got, []string{RuleASFrontMatterYAML}) {
		t.Errorf("level 1 rules %v, want [%s]", got, RuleASFrontMatterYAML)
	}
}

func TestSymlinkedSkillMdIsNotReadThrough(t *testing.T) {
	dir := writeSkill(t, "demo", skillText(""))
	real := filepath.Join(dir, "SKILL.md")
	moved := filepath.Join(t.TempDir(), "real.md")
	if err := os.Rename(real, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, real); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	res, err := Verify(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := rulesOf(res.Level1); !reflect.DeepEqual(got, []string{RuleASUnread}) || res.Level != 0 {
		t.Errorf("level %d, level 1 rules %v, want [%s]", res.Level, got, RuleASUnread)
	}
}

func TestOversizedSkillMdListsTheRulesItCannotEvaluate(t *testing.T) {
	head := "---\nname: demo\ndescription: d\nversion: 3\n---\n"
	res, err := Verify(writeSkill(t, "demo", head+strings.Repeat("x", MaxSkillMDBytes)))
	if err != nil {
		t.Fatal(err)
	}
	if res.Level != 1 {
		t.Fatalf("level %d: %+v %+v", res.Level, res.Level1, res.Violations)
	}
	if got := rulesOf(res.Violations); !reflect.DeepEqual(got, []string{RuleKeyUnknown, RuleSkillMDSize}) {
		t.Errorf("profile rules %v", got)
	}
	if want := []string{RuleTextEncoding, RuleHeaderMalformed, RuleFrontMatterSyntax}; !reflect.DeepEqual(res.NotEvaluated, want) {
		t.Errorf("not evaluated %v, want %v", res.NotEvaluated, want)
	}
}

// The census: every level 1 rule is listed once, and the specification names
// each one in its level 1 section.
func TestEveryLevel1RuleIsSpecified(t *testing.T) {
	specText, err := os.ReadFile(repoPath("spec", "SKILL_FORMAT_V1.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(specText)
	i := strings.Index(s, "## 13. ")
	if i < 0 {
		t.Fatal("section 13 not found")
	}
	row := regexp.MustCompile("(?m)^\\| `(AS-[A-Z0-9-]+)` \\|")
	table := map[string]bool{}
	for _, m := range row.FindAllStringSubmatch(s[i:], -1) {
		if table[m[1]] {
			t.Errorf("section 13 lists %s twice", m[1])
		}
		table[m[1]] = true
	}
	code := map[string]bool{}
	for _, r := range Level1Rules {
		if code[r.ID] {
			t.Errorf("rule %s listed twice", r.ID)
		}
		code[r.ID] = true
	}
	if !reflect.DeepEqual(sortedSet(table), sortedSet(code)) {
		t.Errorf("spec section 13 rules %v differ from Level1Rules %v", sortedSet(table), sortedSet(code))
	}
}
