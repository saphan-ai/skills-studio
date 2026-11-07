package skillfmt

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/saphan-ai/skills-studio/spec"
)

// Result is what Verify found in one skill directory.
type Result struct {
	// Name is the front matter name, when it could be read.
	Name string
	// Level is the level reached: 2 is the Saphan profile (valid Agent Skills and
	// every profile rule met), 1 is valid Agent Skills with at least one profile
	// rule unmet, 0 is neither.
	Level int
	// Hash is the canonical hash; empty unless Level is 2.
	Hash string
	// Violations lists every profile (level 2) rule the skill breaks that the
	// verifier could evaluate, sorted by rule then path. For a level 1 skill it
	// is the whole list of what keeps the skill out of level 2.
	Violations []Violation
	// Level1 lists every level 1 rule the skill breaks, sorted by rule then
	// path. Empty means the skill is valid Agent Skills.
	Level1 []Violation
	// NotEvaluated names the profile rules the verifier could not evaluate
	// because the file was too large to read whole (SF-SKILLMD-SIZE).
	NotEvaluated []string
}

// Passed reports whether the skill reached level 2.
func (r Result) Passed() bool { return r.Level == 2 }

// Verify checks the skill directory at two levels: level 1, the rules of the
// Agent Skills page, and level 2, the Saphan profile (file layout, front matter
// against the published schema, the name against the directory name, and the
// canonical hash). The error return is for what is not a refusal of the skill
// — a path that is not a readable directory.
func Verify(dir string) (Result, error) {
	// Lstat on the cleaned path: a trailing separator would make the system follow a link.
	info, err := os.Lstat(filepath.Clean(dir))
	if err != nil {
		return Result{}, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return Result{
			Violations: []Violation{{Rule: RuleFileSymlink, Path: ".", Message: "the skill directory is a symbolic link; give the directory itself"}},
			Level1:     []Violation{{Rule: RuleASUnread, Path: ".", Message: "the skill directory is a symbolic link; this verifier does not read through links"}},
		}, nil
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("%s is not a directory", dir)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Result{}, err
	}
	schema, err := Compile(spec.SchemaV1)
	if err != nil {
		return Result{}, fmt.Errorf("the embedded schema is unusable: %w", err)
	}

	res, err := verifyLevels(abs, schema)
	if err != nil {
		return Result{}, err
	}
	sortViolations(res.Violations)
	sortViolations(res.Level1)
	switch {
	case len(res.Level1) > 0:
		res.Level = 0
	case len(res.Violations) > 0:
		res.Level = 1
	default:
		res.Level = 2
	}
	if res.Level != 2 {
		res.Hash = ""
	}
	return res, nil
}

func verifyLevels(abs string, schema *Schema) (Result, error) {
	var res Result
	entries, err := os.ReadDir(abs)
	if err != nil {
		return Result{}, err
	}
	hasSkill := false
	for _, e := range entries {
		vs, err := checkEntry(abs, e)
		if err != nil {
			return Result{}, err
		}
		res.Violations = append(res.Violations, vs...)
		if e.Name() == "SKILL.md" && e.Type().IsRegular() {
			hasSkill = true
		}
	}
	l1 := readLevel1(abs)
	res.Level1 = l1.Violations
	if !hasSkill {
		// Without SKILL.md there is nothing further to read.
		if !hasViolation(res.Violations, RuleFileSymlink, "SKILL.md") {
			res.Violations = append(res.Violations, Violation{Rule: RuleSkillMDMissing, Path: "SKILL.md", Message: "the directory has no SKILL.md"})
		}
		return res, nil
	}

	data, tooBig, err := readSkillMD(filepath.Join(abs, "SKILL.md"))
	if err != nil {
		return Result{}, err
	}
	if tooBig != nil {
		// The file is not read whole. The front matter rules are evaluated on
		// what level 1 read from its head; the text rules cannot be.
		res.Violations = append(res.Violations, *tooBig)
		res.NotEvaluated = []string{RuleTextEncoding, RuleHeaderMalformed, RuleFrontMatterSyntax}
		if l1.Doc != nil {
			res.Violations = append(res.Violations, schemaViolations(schema, l1.Doc, filepath.Base(abs))...)
			if n, ok := l1.Doc["name"].(string); ok {
				res.Name = n
			}
		}
		return res, nil
	}
	hash, doc, vs := readDocument(data, schema, filepath.Base(abs), l1.Doc)
	res.Violations = append(res.Violations, vs...)
	if doc == nil {
		doc = l1.Doc // the subset reader read nothing; level 1 may have
	}
	if n, ok := doc["name"].(string); ok {
		res.Name = n
	}
	res.Hash = hash
	return res, nil
}

// readSkillMD reads SKILL.md, never more than MaxSkillMDBytes+1 bytes. A size
// over the limit is refused from the Lstat size before any read; the bounded
// read also holds if the file grows between the size and the read.
func readSkillMD(path string) ([]byte, *Violation, error) {
	tooBig := &Violation{
		Rule:    RuleSkillMDSize,
		Path:    "SKILL.md",
		Message: fmt.Sprintf("SKILL.md is larger than %d bytes", MaxSkillMDBytes),
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if info.Size() > MaxSkillMDBytes {
		return nil, tooBig, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxSkillMDBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if len(data) > MaxSkillMDBytes {
		return nil, tooBig, nil
	}
	return data, nil, nil
}

// readDocument runs the SKILL.md checks in order: text, header, front matter,
// schema, name against directory. A text refusal does not stop the run: the
// text is normalized and read on. A header or front matter refusal that leaves
// no front matter to read stops it; a syntax or a missing-front-matter refusal
// (a fence with trailing blanks, which level 1 reads and the subset reader does
// not) continues on the front matter level 1 read (l1, nil when it did not), so that every rule the skill
// breaks is reported. The hash is returned even when there are refusals; the
// caller keeps it only at level 2.
func readDocument(data []byte, schema *Schema, dirName string, l1 map[string]any) (string, map[string]any, []Violation) {
	var vs []Violation
	if v := CheckText(data); v != nil {
		vs = append(vs, withPath(v, "SKILL.md")...)
		data = normalizeText(data)
	}
	text, v := CanonicalText(data)
	if v != nil {
		return "", nil, append(vs, withPath(v, "SKILL.md")...)
	}
	doc, v := ParseFrontMatter(text)
	if v != nil {
		vs = append(vs, withPath(v, "SKILL.md")...)
		if (v.Rule != RuleFrontMatterSyntax && v.Rule != RuleFrontMatterMissing) || l1 == nil {
			return "", nil, vs
		}
		doc = l1
	}
	vs = append(vs, schemaViolations(schema, doc, dirName)...)
	return HashOfCanonical(text), doc, vs
}

// schemaViolations validates the front matter against the schema and the name
// against the directory name.
func schemaViolations(schema *Schema, doc map[string]any, dirName string) []Violation {
	vs := schema.Validate(doc)
	for i := range vs {
		vs[i].Path = "SKILL.md#" + vs[i].Path
	}
	if n, ok := doc["name"].(string); ok && n != dirName {
		vs = append(vs, Violation{
			Rule:    RuleNameDirMismatch,
			Path:    "SKILL.md#/name",
			Message: fmt.Sprintf("name %q differs from the directory name %q", n, dirName),
		})
	}
	return vs
}

func withPath(v *Violation, path string) []Violation {
	v.Path = path + prefixed(v.Path)
	return []Violation{*v}
}

func prefixed(p string) string {
	if p == "" {
		return ""
	}
	return ":" + p
}

// checkEntry applies the layout rules to one entry of the skill root.
// The error return is for a tree that could not be read: the check cannot say
// the layout is clean, so it says nothing about it.
func checkEntry(root string, e fs.DirEntry) ([]Violation, error) {
	name := e.Name()
	switch {
	case e.Type()&fs.ModeSymlink != 0:
		return []Violation{{Rule: RuleFileSymlink, Path: name, Message: "a symbolic link; the skill holds regular files and directories only"}}, nil
	case name == "scripts":
		return []Violation{{Rule: RuleFileScript, Path: name, Message: "a script never enters a skill; publish it as a tool and cite the tool by id"}}, nil
	case name == "SKILL.md":
		if !e.Type().IsRegular() {
			return []Violation{{Rule: RuleFileUnlisted, Path: name, Message: "SKILL.md is not a regular file"}}, nil
		}
		return nil, nil
	case name == "references" || name == "assets":
		if !e.IsDir() {
			return []Violation{{Rule: RuleFileUnlisted, Path: name, Message: name + " must be a directory"}}, nil
		}
		return walkData(root, name)
	}
	return []Violation{{Rule: RuleFileUnlisted, Path: name, Message: "only SKILL.md, references/ and assets/ may sit in a skill directory"}}, nil
}

// walkData checks the tree under references/ or assets/ holds no symbolic link.
// A directory it cannot read is an error, never a skipped subtree.
func walkData(root, name string) ([]Violation, error) {
	var out []Violation
	err := filepath.WalkDir(filepath.Join(root, name), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			rel, _ := filepath.Rel(root, p)
			out = append(out, Violation{Rule: RuleFileSymlink, Path: filepath.ToSlash(rel), Message: "a symbolic link; the skill holds regular files and directories only"})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func hasViolation(vs []Violation, rule, path string) bool {
	for _, v := range vs {
		if v.Rule == rule && v.Path == path {
			return true
		}
	}
	return false
}

func sortViolations(vs []Violation) {
	sort.SliceStable(vs, func(i, j int) bool {
		if vs[i].Rule != vs[j].Rule {
			return vs[i].Rule < vs[j].Rule
		}
		return vs[i].Path < vs[j].Path
	})
}
