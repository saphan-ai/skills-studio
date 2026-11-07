package skillfmt

import (
	"reflect"
	"strings"
	"testing"
)

// A fence with trailing blanks is read by level 1 and refused by the subset
// reader as SF-FRONTMATTER-MISSING; the schema rules must still be evaluated on
// what level 1 read, so the outside-profile list is whole and the name is known.
func TestBlankFenceSkillNamesEveryProfileRule(t *testing.T) {
	for name, text := range map[string]string{
		"opening fence with a trailing blank": "--- \nname: c1\ndescription: d\nversion: 3\n---\n\n# Body\n",
		"closing fence with a trailing blank": "---\nname: c1\ndescription: d\nversion: 3\n--- \n\n# Body\n",
	} {
		t.Run(name, func(t *testing.T) {
			res, err := Verify(writeSkill(t, "c1", text))
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Level1) != 0 || res.Level != 1 {
				t.Fatalf("level %d, level 1 %+v; want valid Agent Skills at level 1", res.Level, res.Level1)
			}
			want := []string{RuleFrontMatterMissing, RuleKeyUnknown}
			if got := rulesOf(res.Violations); !reflect.DeepEqual(got, want) {
				t.Errorf("profile rules %v, want %v", got, want)
			}
			if res.Name != "c1" {
				t.Errorf("name %q, want c1", res.Name)
			}
		})
	}
}

// A metadata key must be a string, as the value must; a key YAML reads as
// an integer, a boolean or null is refused at level 1.
func TestMetadataKeyMustBeAString(t *testing.T) {
	for name, front := range map[string]string{
		"integer key": "metadata: {1: x}\n",
		"boolean key": "metadata:\n  true: x\n",
		"null key":    "metadata:\n  ~: x\n",
		"float key":   "metadata:\n  1.5: x\n",
	} {
		t.Run(name, func(t *testing.T) {
			res, _ := Verify(writeSkill(t, "demo", "---\nname: demo\ndescription: d\n"+front+"---\n"))
			if got := rulesOf(res.Level1); !reflect.DeepEqual(got, []string{RuleASMetadataValueType}) || res.Level != 0 {
				t.Errorf("level %d, level 1 rules %v, want level 0 with %s", res.Level, got, RuleASMetadataValueType)
			}
		})
	}
	for name, front := range map[string]string{
		"quoted integer key": "metadata:\n  \"1\": x\n",
		"plain word key":     "metadata:\n  a: x\n",
	} {
		t.Run(name, func(t *testing.T) {
			res, _ := Verify(writeSkill(t, "demo", "---\nname: demo\ndescription: d\n"+front+"---\n"))
			if len(res.Level1) != 0 {
				t.Errorf("level 1 refused: %+v", res.Level1)
			}
		})
	}
}

// Level 1 reads "unicode lowercase alphanumeric" as unicode
// lowercase letters and digits; level 2 keeps ASCII a-z 0-9 -.
func TestUnicodeNameIsLevel1AndNotLevel2(t *testing.T) {
	for _, n := range []string{"αβγ", "demo-٣"} {
		res, err := Verify(writeSkill(t, n, "---\nname: "+n+"\ndescription: d\n---\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Level1) != 0 || res.Level != 1 {
			t.Errorf("%q: level %d, level 1 %+v; want level 1", n, res.Level, res.Level1)
		}
		if !contains(rulesOf(res.Violations), RuleNamePattern) {
			t.Errorf("%q: profile rules %v lack %s", n, rulesOf(res.Violations), RuleNamePattern)
		}
	}
}

// The text rules with their exact lists (measured on this reader).
func TestTextRulesExactLists(t *testing.T) {
	for name, c := range map[string]struct {
		text string
		want []string
	}{
		"nul": {"---\nname: demo\x00\n---\n", []string{RuleFrontMatterSyntax, RuleTextEncoding}},
		// Invalid UTF-8 is normalized away on the profile side, so the schema rules
		// then read the replaced name; measured 2026-11-02.
		"invalid utf8": {"---\nname: \xff\n---\n", []string{RuleDescriptionRequired, RuleNameDirMismatch, RuleNamePattern, RuleTextEncoding}},
	} {
		res, _ := Verify(writeSkill(t, "demo", c.text))
		if got := rulesOf(res.Violations); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: profile rules %v, want %v", name, got, c.want)
		}
	}
}

// A null value is read as absent for every optional field too.
func TestNullOptionalFieldsAreAbsent(t *testing.T) {
	for _, f := range []string{"compatibility", "metadata", "allowed-tools", "license"} {
		res, _ := Verify(writeSkill(t, "demo", "---\nname: demo\ndescription: d\n"+f+":\n---\n"))
		if len(res.Level1) != 0 {
			t.Errorf("%s: null refused at level 1: %+v", f, res.Level1)
		}
	}
	res, _ := Verify(writeSkill(t, "demo", "---\nname: demo\ndescription:\n---\n"))
	if got := strings.Join(rulesOf(res.Level1), ","); got != RuleASDescRequired {
		t.Errorf("description null: level 1 rules %q, want %s", got, RuleASDescRequired)
	}
}

// A YAML merge key is a literal key; it is not applied.
func TestMergeKeyIsNotApplied(t *testing.T) {
	res, _ := Verify(writeSkill(t, "demo", "---\nname: demo\ndescription: d\nmetadata:\n  <<: {c: d}\n---\n"))
	if got := rulesOf(res.Level1); !reflect.DeepEqual(got, []string{RuleASMetadataValueType}) {
		t.Errorf("level 1 rules %v, want %s on the literal key <<", got, RuleASMetadataValueType)
	}
}
