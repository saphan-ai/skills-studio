package skillfmt

import (
	"os"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func sortedRuleIDs(pred func(Rule) bool) []string {
	var out []string
	for _, r := range Rules {
		if pred(r) {
			out = append(out, r.ID)
		}
	}
	sort.Strings(out)
	return out
}

func sortedSet(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// N1: the census runs in both directions. The rule ids of the section 9 table
// and every x-rule of the schema are read from the published files and must
// equal the code's list; a rule present in only one place is red.
func TestPublishedRuleSetsEqualTheCodeList(t *testing.T) {
	specText, err := os.ReadFile(repoPath("spec", "SKILL_FORMAT_V1.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(specText)
	i := strings.Index(s, "## 9. Rules")
	j := strings.Index(s, "## 10. ")
	if i < 0 || j < i {
		t.Fatal("section 9 not found")
	}
	row := regexp.MustCompile("(?m)^\\| `([A-Za-z0-9-]+)` \\|")
	table := map[string]bool{}
	for _, m := range row.FindAllStringSubmatch(s[i:j], -1) {
		if table[m[1]] {
			t.Errorf("section 9 lists %s twice", m[1])
		}
		table[m[1]] = true
	}
	if got, want := sortedSet(table), sortedRuleIDs(func(Rule) bool { return true }); !reflect.DeepEqual(got, want) {
		t.Errorf("spec section 9 table rules %v differ from Rules %v", got, want)
	}

	schemaText, err := os.ReadFile(repoPath("spec", "skill-format-v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	annotated := map[string]bool{}
	for _, m := range regexp.MustCompile(`"x-rule"\s*:\s*"([^"]*)"`).FindAllStringSubmatch(string(schemaText), -1) {
		annotated[m[1]] = true
	}
	if got, want := sortedSet(annotated), sortedRuleIDs(func(r Rule) bool { return schemaRule(r.ID) }); !reflect.DeepEqual(got, want) {
		t.Errorf("schema x-rule values %v differ from the schema rules in Rules %v", got, want)
	}
}

// N3: SKILL.md is bounded. A file over the limit is refused by SF-SKILLMD-SIZE
// from its Lstat size, and the verifier asks for a constant multiple of the
// limit, never a multiple of the file.
func TestOversizedSkillMdIsRefusedWithBoundedAllocation(t *testing.T) {
	// Level 1 reads the head of the file, at most MaxLevel1PrefixBytes, so the
	// allowance is a constant multiple of that bound, whatever the file size.
	for _, size := range []int{MaxSkillMDBytes, 32 * MaxSkillMDBytes} {
		big := "---\n" + strings.Repeat("\n", size)
		dir := writeSkill(t, "demo", big)
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		res, err := Verify(dir)
		runtime.ReadMemStats(&after)
		if err != nil {
			t.Fatal(err)
		}
		if got := rulesOf(res.Violations); !reflect.DeepEqual(got, []string{RuleSkillMDSize}) || res.Hash != "" {
			t.Fatalf("refused by %v with hash %q, want only %s", got, res.Hash, RuleSkillMDSize)
		}
		if got := rulesOf(res.Level1); !reflect.DeepEqual(got, []string{RuleASUnread}) {
			t.Errorf("level 1 refused by %v, want only %s (front matter never closed within the bound)", got, RuleASUnread)
		}
		if grew := after.TotalAlloc - before.TotalAlloc; grew > uint64(8*MaxLevel1PrefixBytes) {
			t.Errorf("verifying a file of %d bytes allocated %d bytes, more than %d", size, grew, 8*MaxLevel1PrefixBytes)
		}
	}
}

func TestSkillMdAtTheLimitIsAccepted(t *testing.T) {
	head := "---\nname: demo\ndescription: d\n---\n"
	text := head + strings.Repeat("x", MaxSkillMDBytes-len(head))
	res, err := Verify(writeSkill(t, "demo", text))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed() {
		t.Fatalf("a file of exactly %d bytes was refused: %+v", MaxSkillMDBytes, res.Violations)
	}
}

// N3: the front matter is read up to the closing line, and the rest of the
// file is not split into lines.
func TestFrontMatterReadStopsAtTheClosingLine(t *testing.T) {
	text := []byte("---\nname: a\n---\n" + strings.Repeat("\n", MaxSkillMDBytes-16))
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	doc, v := ParseFrontMatter(text)
	runtime.ReadMemStats(&after)
	if v != nil || doc["name"] != "a" {
		t.Fatalf("got %v %+v", doc, v)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 64*1024 {
		t.Errorf("reading a 3-line front matter allocated %d bytes", grew)
	}
}
