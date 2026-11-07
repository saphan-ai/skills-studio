package skillfmt

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/saphan-ai/skills-studio/spec"
)

const root = "../.."

func repoPath(parts ...string) string {
	return filepath.Join(append([]string{root}, parts...)...)
}

func rulesOf(vs []Violation) []string {
	seen := map[string]bool{}
	for _, v := range vs {
		seen[v.Rule] = true
	}
	var out []string
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

func subdirs(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// writeSkill makes <tmp>/<name>/SKILL.md with the given text and returns the directory.
func writeSkill(t *testing.T, name, text string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func skillText(extra string) string {
	return "---\nname: demo\ndescription: A demo skill.\n" + extra + "---\n\n# Demo\n"
}

func TestValidExamplesPass(t *testing.T) {
	names := subdirs(t, repoPath("examples", "valid"))
	if len(names) < 6 {
		t.Fatalf("expected at least 6 valid examples, found %v", names)
	}
	for _, n := range names {
		t.Run(n, func(t *testing.T) {
			res, err := Verify(repoPath("examples", "valid", n))
			if err != nil {
				t.Fatal(err)
			}
			if !res.Passed() {
				t.Fatalf("refused: %+v", res.Violations)
			}
		})
	}
}

func TestInvalidExamplesRefusedByExactlyTheirRule(t *testing.T) {
	for _, ruleDir := range subdirs(t, repoPath("examples", "invalid")) {
		want := strings.ToUpper(ruleDir)
		kids := subdirs(t, repoPath("examples", "invalid", ruleDir))
		if len(kids) != 1 {
			t.Fatalf("%s: want exactly one skill directory, found %v", ruleDir, kids)
		}
		t.Run(ruleDir, func(t *testing.T) {
			res, err := Verify(repoPath("examples", "invalid", ruleDir, kids[0]))
			if err != nil {
				t.Fatal(err)
			}
			if res.Passed() || res.Hash != "" {
				t.Fatalf("accepted; hash %q", res.Hash)
			}
			if got := rulesOf(res.Violations); !reflect.DeepEqual(got, []string{want}) {
				t.Fatalf("refused by %v, want exactly [%s]: %+v", got, want, res.Violations)
			}
		})
	}
}

// The census holds the rule list, the examples, the schema annotations and the
// specification text to one another: a rule added to one and not the others
// turns this test red.
func TestEveryRuleIsExemplifiedAnnotatedAndSpecified(t *testing.T) {
	ids := map[string]Rule{}
	for _, r := range Rules {
		if _, dup := ids[r.ID]; dup {
			t.Fatalf("rule %s listed twice", r.ID)
		}
		ids[r.ID] = r
	}

	onDisk := map[string]bool{}
	for _, d := range subdirs(t, repoPath("examples", "invalid")) {
		onDisk[strings.ToUpper(d)] = true
	}
	for id, r := range ids {
		if r.Example && !onDisk[id] {
			t.Errorf("rule %s has no examples/invalid/%s", id, strings.ToLower(id))
		}
		if !r.Example && onDisk[id] {
			t.Errorf("rule %s is listed without an example but one exists", id)
		}
	}
	for d := range onDisk {
		if _, ok := ids[d]; !ok {
			t.Errorf("examples/invalid/%s names no rule", strings.ToLower(d))
		}
	}

	specText, err := os.ReadFile(repoPath("spec", "SKILL_FORMAT_V1.md"))
	if err != nil {
		t.Fatal(err)
	}
	for id := range ids {
		if !strings.Contains(string(specText), "`"+id+"`") {
			t.Errorf("rule %s is not in spec/SKILL_FORMAT_V1.md", id)
		}
	}

	schemaText, err := os.ReadFile(repoPath("spec", "skill-format-v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range Rules {
		inSchema := strings.Contains(string(schemaText), `"`+r.ID+`"`)
		inCode := schemaRule(r.ID)
		if inSchema != inCode {
			t.Errorf("rule %s: in schema=%v, expected schema rule=%v", r.ID, inSchema, inCode)
		}
	}
}

// schemaRule lists the rules that live in the JSON Schema; the rest are file,
// text, header, front matter, directory-name and signature rules in code.
func schemaRule(id string) bool {
	switch id {
	case RuleKeyUnknown, RuleNameRequired, RuleNamePattern, RuleNameLength, RuleDescriptionRequired,
		RuleDescriptionLength, RuleFieldType, RuleCompatibilityLength, RuleMetadataType,
		RuleMetadataValueType, RuleSaphanOrigin, RuleSaphanDefault:
		return true
	}
	return false
}

func TestSchemaNeverReportsAnUnattributedFailure(t *testing.T) {
	for _, extra := range []string{"", "version: 1\n", "license: 3\n", "metadata: x\n", "metadata:\n  a: 3\n"} {
		dir := writeSkill(t, "demo", skillText(extra))
		res, err := Verify(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range res.Violations {
			if v.Rule == RuleSchemaUnattributed {
				t.Errorf("%q: unattributed failure %+v", extra, v)
			}
		}
	}
}

func TestBoundaryLengths(t *testing.T) {
	cases := []struct {
		field string
		n     int
		rule  string
	}{
		{"description", 1024, ""}, {"description", 1025, RuleDescriptionLength},
		{"compatibility", 500, ""}, {"compatibility", 501, RuleCompatibilityLength},
	}
	for _, c := range cases {
		text := "---\nname: demo\ndescription: d\n---\n"
		if c.field == "description" {
			text = "---\nname: demo\ndescription: " + strings.Repeat("é", c.n) + "\n---\n"
		} else {
			text = "---\nname: demo\ndescription: d\ncompatibility: " + strings.Repeat("é", c.n) + "\n---\n"
		}
		res, err := Verify(writeSkill(t, "demo", text))
		if err != nil {
			t.Fatal(err)
		}
		want := []string(nil)
		if c.rule != "" {
			want = []string{c.rule}
		}
		if got := rulesOf(res.Violations); !reflect.DeepEqual(got, want) {
			t.Errorf("%s of %d characters: refused by %v, want %v", c.field, c.n, got, want)
		}
	}
	for _, c := range []struct {
		name string
		rule string
	}{{strings.Repeat("a", 64), ""}, {strings.Repeat("a", 65), RuleNameLength}, {"", RuleNamePattern}} {
		res, err := Verify(writeSkill(t, "x", "---\nname: \""+c.name+"\"\ndescription: d\n---\n"))
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, r := range rulesOf(res.Violations) {
			if r != RuleNameDirMismatch {
				got = append(got, r)
			}
		}
		var want []string
		if c.rule != "" {
			want = []string{c.rule}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("name of %d characters: refused by %v, want %v", len(c.name), got, want)
		}
	}
	res, _ := Verify(writeSkill(t, "x", "---\nname: x\ndescription: \"\"\n---\n"))
	if got := rulesOf(res.Violations); !reflect.DeepEqual(got, []string{RuleDescriptionLength}) {
		t.Errorf("empty description: %v", got)
	}
}

func TestScriptsBesideSkillMdAreRefused(t *testing.T) {
	dir := writeSkill(t, "demo", skillText(""))
	res, _ := Verify(dir)
	if !res.Passed() {
		t.Fatalf("baseline skill refused: %+v", res.Violations)
	}
	if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scripts", "x.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, _ = Verify(dir)
	if got := rulesOf(res.Violations); !reflect.DeepEqual(got, []string{RuleFileScript}) || res.Hash != "" {
		t.Fatalf("refused by %v with hash %q, want only %s", got, res.Hash, RuleFileScript)
	}
}

func TestSymlinkIsRefusedAnywhere(t *testing.T) {
	for _, where := range []string{"link", "references/link"} {
		dir := writeSkill(t, "demo", skillText(""))
		if err := os.MkdirAll(filepath.Join(dir, "references"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(dir, "SKILL.md"), filepath.Join(dir, where)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		res, _ := Verify(dir)
		want := []string{RuleFileSymlink}
		if got := rulesOf(res.Violations); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: refused by %v, want %v", where, got, want)
		}
	}
}

func TestReferencesAndAssetsAreAllowedNested(t *testing.T) {
	dir := writeSkill(t, "demo", skillText(""))
	for _, p := range []string{"references/a/b/c.md", "assets/d.bin"} {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if res, _ := Verify(dir); !res.Passed() {
		t.Fatalf("refused: %+v", res.Violations)
	}
}

func TestVerifyUsageErrors(t *testing.T) {
	if _, err := Verify(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("a missing path must be an error, not a refusal")
	}
	file := filepath.Join(t.TempDir(), "f")
	_ = os.WriteFile(file, nil, 0o644)
	if _, err := Verify(file); err == nil {
		t.Error("a file is not a skill directory")
	}
}

func TestCanonicalHashHeaderRule(t *testing.T) {
	body := "---\nname: demo\ndescription: d\n---\n\n# Demo\n"
	header := HeaderOpen + "\n_id: x\n-->\n\n"

	plain, v := Hash([]byte(body))
	if v != nil {
		t.Fatal(v)
	}
	withHeader, v := Hash([]byte(header + body))
	if v != nil {
		t.Fatal(v)
	}
	if plain != withHeader {
		t.Errorf("the header changed the hash: %s vs %s", plain, withHeader)
	}

	cases := map[string]string{
		"no closing line":       HeaderOpen + "\n_id: x\n" + body,
		"no blank line":         HeaderOpen + "\n_id: x\n-->\n" + body,
		"two lines then text":   HeaderOpen + "\n-->\ntext\n",
		"closing line at EOF":   HeaderOpen + "\n-->",
		"closing then EOF only": HeaderOpen + "\n-->\n",
	}
	for name, text := range cases {
		if _, v := Hash([]byte(text)); v == nil || v.Rule != RuleHeaderMalformed {
			t.Errorf("%s: got %+v, want %s", name, v, RuleHeaderMalformed)
		}
	}

	// A line that only starts like the opener is not a header: the text is hashed whole.
	notHeader := HeaderOpen + " extra\n" + body
	if got, _ := Hash([]byte(notHeader)); got == plain {
		t.Error("an opener with extra text on its line must not be stripped")
	}
	// A header that is not on the first line is not stripped.
	late := "\n" + header + body
	if got, _ := Hash([]byte(late)); got == plain {
		t.Error("a header after the first line must not be stripped")
	}
	// Stripping happens once.
	twice := header + header + body
	if got, _ := Hash([]byte(twice)); got == plain {
		t.Error("only one header is stripped")
	}
}

func TestOneByteChangesTheHash(t *testing.T) {
	data, err := os.ReadFile(repoPath("examples", "valid", "read-files", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	base, _ := Hash(data)
	for _, i := range []int{0, len(data) / 2, len(data) - 1} {
		mutated := append([]byte(nil), data...)
		mutated[i] ^= 0x01
		if got, v := Hash(mutated); v == nil && got == base {
			t.Errorf("flipping a bit at byte %d left the hash unchanged", i)
		}
	}
}

func TestProjectedExampleHashesWithoutItsHeader(t *testing.T) {
	data, err := os.ReadFile(repoPath("examples", "valid", "projected-skill", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, after, ok := strings.Cut(string(data), "-->\n\n")
	if !ok {
		t.Fatal("example has no header")
	}
	if got, want := HashOfCanonical([]byte(after)), mustHash(t, data); got != want {
		t.Errorf("hash %s differs from the hash of the text after the header %s", want, got)
	}
}

func mustHash(t *testing.T, b []byte) string {
	t.Helper()
	h, v := Hash(b)
	if v != nil {
		t.Fatal(v)
	}
	return h
}

func TestStatedHashesMatch(t *testing.T) {
	data, err := os.ReadFile(repoPath("examples", "HASHES.txt"))
	if err != nil {
		t.Fatal(err)
	}
	lines := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		lines++
		hash, path, ok := strings.Cut(line, "  ")
		if !ok {
			t.Fatalf("bad line %q", line)
		}
		res, err := Verify(repoPath("examples", filepath.FromSlash(path)))
		if err != nil || !res.Passed() {
			t.Fatalf("%s: %v %+v", path, err, res.Violations)
		}
		if res.Hash != hash {
			t.Errorf("%s: printed %s, stated %s", path, res.Hash, hash)
		}
	}
	if lines < 2 {
		t.Errorf("expected the two fleet skills in HASHES.txt, found %d lines", lines)
	}
}

// The worked examples in the specification state hashes; they are held to the
// verifier so the text cannot drift from the code.
func TestSpecWorkedExamplesStateTheRealHashes(t *testing.T) {
	specText, err := os.ReadFile(repoPath("spec", "SKILL_FORMAT_V1.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"read-files", "projected-skill"} {
		res, err := Verify(repoPath("examples", "valid", name))
		if err != nil || !res.Passed() {
			t.Fatalf("%s: %v %+v", name, err, res.Violations)
		}
		if !strings.Contains(string(specText), "sha256 = "+res.Hash) {
			t.Errorf("spec does not state the hash %s of examples/valid/%s", res.Hash, name)
		}
	}
}

// The public text makes no claim that a skill is fit to run. The banned words
// are the ones the owner's rule names.
func TestPublicTextMakesNoSafetyClaim(t *testing.T) {
	for _, p := range []string{"spec/SKILL_FORMAT_V1.md", "README.md", "SECURITY.md", "CONTRIBUTING.md", "examples/README.md"} {
		data, err := os.ReadFile(repoPath(filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			low := strings.ToLower(line)
			if strings.Contains(low, "safe") || strings.Contains(low, "secure skill") {
				t.Errorf("%s:%d: %q", p, i+1, line)
			}
		}
	}
}

func TestTextRules(t *testing.T) {
	// The text rule does not stop the run: the profile rules are evaluated on
	// the text with the line ends and the mark normalized, so a skill whose only
	// fault is the text is refused by the text rule alone.
	for name, text := range map[string]string{
		"carriage return": "---\r\nname: demo\r\ndescription: d\r\n---\r\n",
		"bom":             "\xef\xbb\xbf---\nname: demo\ndescription: d\n---\n",
	} {
		res, _ := Verify(writeSkill(t, "demo", text))
		if got := rulesOf(res.Violations); !reflect.DeepEqual(got, []string{RuleTextEncoding}) {
			t.Errorf("%s: refused by %v", name, got)
		}
		if res.Level != 1 {
			t.Errorf("%s: level %d, want 1 (valid Agent Skills, outside the profile)", name, res.Level)
		}
	}
	// A NUL byte or invalid UTF-8 cannot be normalized away: the text rule is
	// reported, with whatever else the reader finds on the same bytes.
	for name, text := range map[string]string{
		"nul":          "---\nname: demo\x00\n---\n",
		"invalid utf8": "---\nname: \xff\n---\n",
	} {
		res, _ := Verify(writeSkill(t, "demo", text))
		if got := rulesOf(res.Violations); !contains(got, RuleTextEncoding) {
			t.Errorf("%s: refused by %v, want %s among them", name, got, RuleTextEncoding)
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestFrontMatterReader(t *testing.T) {
	ok := map[string]map[string]any{
		"---\nname: a\n---\n":                                {"name": "a"},
		"---\nname: \"a b\\n\"\n---\n":                       {"name": "a b\n"},
		"---\nname: 'it''s'\n---\n":                          {"name": "it's"},
		"---\nn: 3\nb: true\nz: null\n---\n":                 {"n": 3.0, "b": true, "z": nil},
		"---\n# comment\n\nname: a\n---\n":                   {"name": "a"},
		"---\nname: a\nmetadata:\n  k: \"v\"\n  j: x\n---\n": {"name": "a", "metadata": map[string]any{"k": "v", "j": "x"}},
		"---\nname: a\nmetadata:\n---\n":                     {"name": "a", "metadata": nil},
		"---\nname: a — b, c (d); e\n---\n":                  {"name": "a — b, c (d); e"},
		"---\nname: a\nbody-text-after: x\n---\nmore\n---\n": {"name": "a", "body-text-after": "x"},
	}
	for text, want := range ok {
		got, v := ParseFrontMatter([]byte(text))
		if v != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v %+v, want %v", text, got, v, want)
		}
	}
	bad := map[string]string{
		"description: x\n---\n":          RuleFrontMatterMissing,
		"---\nname: a\n":                 RuleFrontMatterMissing,
		"---\nname: a\nname: b\n---\n":   RuleFrontMatterSyntax,
		"---\nname: on\n---\n":           RuleFrontMatterSyntax,
		"---\nname: No\n---\n":           RuleFrontMatterSyntax,
		"---\nname: a: b\n---\n":         RuleFrontMatterSyntax,
		"---\nname: a #c\n---\n":         RuleFrontMatterSyntax,
		"---\nname: >\n  folded\n---\n":  RuleFrontMatterSyntax,
		"---\nname: [a]\n---\n":          RuleFrontMatterSyntax,
		"---\nname: \"a\n---\n":          RuleFrontMatterSyntax,
		"---\nname: 'a\n---\n":           RuleFrontMatterSyntax,
		"---\n  name: a\n---\n":          RuleFrontMatterSyntax,
		"---\nm:\n  a: 1\n  a: 2\n---\n": RuleFrontMatterSyntax,
		"---\nm:\n   a: 1\n---\n":        RuleFrontMatterSyntax,
		"---\nm:\n\ta: 1\n---\n":         RuleFrontMatterSyntax,
		"---\nname:a\n---\n":             RuleFrontMatterSyntax,
		"---\n- item\n---\n":             RuleFrontMatterSyntax,
		"---\nbad key: x\n---\n":         RuleFrontMatterSyntax,
		"---\nname: tRUE\n---\n":         RuleFrontMatterSyntax,
		"---\nname: nUlL\n---\n":         RuleFrontMatterSyntax,
		"---\nname: ,a\n---\n":           RuleFrontMatterSyntax,
		"---\nname: ]a\n---\n":           RuleFrontMatterSyntax,
		"---\nname: }a\n---\n":           RuleFrontMatterSyntax,
	}
	for text, rule := range bad {
		if _, v := ParseFrontMatter([]byte(text)); v == nil || v.Rule != rule {
			t.Errorf("%q: got %+v, want %s", text, v, rule)
		}
	}

	// A plain scalar that a YAML 1.1 or 1.2-core reader resolves to something other
	// than the string this reader would return is refused (review condition C1).
	divergent := map[string]string{
		"leading hash":            "#hello",
		"hex":                     "0x1F",
		"octal 1.2":               "0o17",
		"binary":                  "0b101",
		"octal 1.1":               "017",
		"inf":                     ".inf",
		"inf upper":               ".INF",
		"nan mixed":               ".NaN",
		"signed inf":              "+.inf",
		"timestamp date":          "2026-10-31",
		"timestamp T":             "2026-10-31T10:20:30Z",
		"timestamp spaced":        "2026-10-31 10:20:30 +1",
		"underscore int":          "1_000",
		"underscore float":        "1_0.5",
		"sexagesimal int":         "1:20",
		"sexagesimal three parts": "1:20:30",
		"sexagesimal float":       "1:20.5",
	}
	for name, v := range divergent {
		text := "---\nname: a\ndescription: " + v + "\n---\n"
		if _, viol := ParseFrontMatter([]byte(text)); viol == nil || viol.Rule != RuleFrontMatterSyntax {
			t.Errorf("%s (%q): got %+v, want %s", name, v, viol, RuleFrontMatterSyntax)
		}
	}
	// Non-printable code points anywhere in the front matter, comments included.
	for name, r := range map[string]string{
		"ESC": "\x1b", "SOH": "\x01", "DEL": "\x7f", "NEL U+0085": "\u0085",
		"C1 U+0090": "\u0090", "LS U+2028": " ", "PS U+2029": " ",
	} {
		for where, text := range map[string]string{
			"value":   "---\nname: a\ndescription: x" + r + "y\n---\n",
			"comment": "---\n# x" + r + "y\nname: a\n---\n",
			"quoted":  "---\nname: a\ndescription: \"x" + r + "y\"\n---\n",
		} {
			if _, viol := ParseFrontMatter([]byte(text)); viol == nil || viol.Rule != RuleFrontMatterSyntax {
				t.Errorf("%s in %s: got %+v, want %s", name, where, viol, RuleFrontMatterSyntax)
			}
		}
	}
	// Near misses stay strings: only what a YAML reader resolves otherwise is refused.
	for _, v := range []string{"0xZZ", "2026-10-31 release notes", "1:a", "1_a", "a_b", ".infinity", "0o9", "0b2", "v1:20", "tab here", "é — ü"} {
		text := "---\nname: a\ndescription: " + v + "\n---\n"
		got, viol := ParseFrontMatter([]byte(text))
		if viol != nil || got["description"] != v {
			t.Errorf("%q: got %v %+v, want the string unchanged", v, got, viol)
		}
	}
}

func TestPlainScalarTypesAreCaughtByTheSchema(t *testing.T) {
	for _, extra := range []string{"license: 42\n", "license: true\n", "license:\n", "metadata:\n  a: true\n", "allowed-tools: 1.5\n"} {
		res, _ := Verify(writeSkill(t, "demo", skillText(extra)))
		got := rulesOf(res.Violations)
		if len(got) != 1 || (got[0] != RuleFieldType && got[0] != RuleMetadataValueType) {
			t.Errorf("%q: refused by %v", extra, got)
		}
	}
}

func TestQuotedSaphanValuesPass(t *testing.T) {
	extra := "metadata:\n  saphan.origin: project\n  saphan.default: \"off\"\n  saphan.roles: \"a, b\"\n"
	if res, _ := Verify(writeSkill(t, "demo", skillText(extra))); !res.Passed() {
		t.Fatalf("refused: %+v", res.Violations)
	}
}

func TestSchemaCompileRefusesUnsupportedKeywords(t *testing.T) {
	for _, s := range []string{
		`{"type":"object","oneOf":[]}`,
		`{"properties":{"a":{"format":"uri"}}}`,
		`{"allOf":[{"$ref":"#/x"}]}`,
		`{"pattern":"("}`,
		`[]`,
		`not json`,
	} {
		if _, err := Compile([]byte(s)); err == nil {
			t.Errorf("schema %s compiled; an unsupported keyword must be refused", s)
		}
	}
	if _, err := Compile(schemaBytes(t)); err != nil {
		t.Errorf("the published schema does not compile: %v", err)
	}
}

func schemaBytes(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(repoPath("spec", "skill-format-v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestEmbeddedSchemaIsThePublishedFile(t *testing.T) {
	if string(spec.SchemaV1) != string(schemaBytes(t)) {
		t.Error("the embedded schema differs from spec/skill-format-v1.schema.json")
	}
}
