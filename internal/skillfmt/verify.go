package skillfmt

import (
	"fmt"
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
	// Hash is the canonical hash; empty when SKILL.md could not be read as a
	// skill document (a refusal in Violations says why).
	Hash string
	// Violations lists every refusal, sorted by rule then path. Empty means the
	// skill passes the schema and the file layout checks.
	Violations []Violation
}

// Passed reports whether the skill was accepted.
func (r Result) Passed() bool { return len(r.Violations) == 0 && r.Hash != "" }

// Verify checks the skill directory: file layout, front matter against the
// published schema, the name against the directory name, and the canonical
// hash. The error return is for what is not a refusal of the skill — a path
// that is not a readable directory.
func Verify(dir string) (Result, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return Result{}, err
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

	var res Result
	entries, err := os.ReadDir(abs)
	if err != nil {
		return Result{}, err
	}
	hasSkill := false
	for _, e := range entries {
		res.Violations = append(res.Violations, checkEntry(abs, e)...)
		if e.Name() == "SKILL.md" && e.Type().IsRegular() {
			hasSkill = true
		}
	}
	if !hasSkill {
		// Without SKILL.md there is nothing further to read.
		if !hasViolation(res.Violations, RuleFileSymlink, "SKILL.md") {
			res.Violations = append(res.Violations, Violation{Rule: RuleSkillMDMissing, Path: "SKILL.md", Message: "the directory has no SKILL.md"})
		}
		sortViolations(res.Violations)
		return res, nil
	}

	data, err := os.ReadFile(filepath.Join(abs, "SKILL.md"))
	if err != nil {
		return Result{}, err
	}
	hash, doc, vs := readDocument(data, schema, filepath.Base(abs))
	res.Violations = append(res.Violations, vs...)
	if n, ok := doc["name"].(string); ok {
		res.Name = n
	}
	if len(res.Violations) == 0 {
		res.Hash = hash
	}
	sortViolations(res.Violations)
	return res, nil
}

// readDocument runs the SKILL.md checks in order: text, header, front matter,
// schema, name against directory. The first three stop the run at their refusal.
func readDocument(data []byte, schema *Schema, dirName string) (string, map[string]any, []Violation) {
	if v := CheckText(data); v != nil {
		return "", nil, withPath(v, "SKILL.md")
	}
	text, v := CanonicalText(data)
	if v != nil {
		return "", nil, withPath(v, "SKILL.md")
	}
	doc, v := ParseFrontMatter(text)
	if v != nil {
		return "", nil, withPath(v, "SKILL.md")
	}
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
	return HashOfCanonical(text), doc, vs
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
func checkEntry(root string, e fs.DirEntry) []Violation {
	name := e.Name()
	switch {
	case e.Type()&fs.ModeSymlink != 0:
		return []Violation{{Rule: RuleFileSymlink, Path: name, Message: "a symbolic link; the skill holds regular files and directories only"}}
	case name == "scripts":
		return []Violation{{Rule: RuleFileScript, Path: name, Message: "a script never enters a skill; publish it as a tool and cite the tool by id"}}
	case name == "SKILL.md":
		if !e.Type().IsRegular() {
			return []Violation{{Rule: RuleFileUnlisted, Path: name, Message: "SKILL.md is not a regular file"}}
		}
		return nil
	case name == "references" || name == "assets":
		if !e.IsDir() {
			return []Violation{{Rule: RuleFileUnlisted, Path: name, Message: name + " must be a directory"}}
		}
		return walkData(root, name)
	}
	return []Violation{{Rule: RuleFileUnlisted, Path: name, Message: "only SKILL.md, references/ and assets/ may sit in a skill directory"}}
}

// walkData checks the tree under references/ or assets/ holds no symbolic link.
func walkData(root, name string) []Violation {
	var out []Violation
	_ = filepath.WalkDir(filepath.Join(root, name), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			rel, _ := filepath.Rel(root, p)
			out = append(out, Violation{Rule: RuleFileSymlink, Path: filepath.ToSlash(rel), Message: "a symbolic link; the skill holds regular files and directories only"})
		}
		return nil
	})
	return out
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
