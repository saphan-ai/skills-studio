package exportfmt

import (
	"errors"
	"strings"

	"gopkg.in/yaml.v3"
)

// Builtins is the registry: one line per built-in format. Adding a format adds
// one line here and nothing else in shared code.
func Builtins() []Adapter {
	return []Adapter{
		AgentSkills{},
		CursorRules{},
		AgentsMD{},
		CopilotInstructions{},
	}
}

// Default is the registry of the built-in formats.
func Default() *Registry {
	r, err := NewRegistry(Builtins()...)
	if err != nil {
		panic("exportfmt: the built-in registry is malformed: " + err.Error())
	}
	return r
}

// A compile-time assertion per adapter: what each one is, stated where it is
// written.
var (
	_ Adapter  = AgentSkills{}
	_ Importer = AgentSkills{}
	_ Adapter  = CursorRules{}
	_ Adapter  = AgentsMD{}
	_ Adapter  = CopilotInstructions{}
)

var errNoSkill = errors.New("the skill has no name; export needs a skill read by Load")

// textLosses lists every piece of s that a target of one Markdown file cannot
// carry, given what the target says about the pieces. carried names the front
// matter pieces the target does carry ("name", "description").
func textLosses(s Skill, target string, carried ...string) []Loss {
	has := map[string]bool{}
	for _, c := range carried {
		has[c] = true
	}
	var out []Loss
	lose := func(item, what string) {
		out = append(out, Loss{Item: item, Reason: target + " has no place for " + what})
	}
	if s.License != "" && !has["license"] {
		lose("license", "a license field")
	}
	if s.Compatibility != "" && !has["compatibility"] {
		lose("compatibility", "a compatibility field")
	}
	if s.AllowedTools != "" && !has["allowed-tools"] {
		lose("allowed-tools", "a list of allowed tools; the skill's tool permissions are not carried")
	}
	if len(s.Metadata) > 0 && !has["metadata"] {
		lose("metadata", "a metadata map")
	}
	for _, k := range s.ExtraKeys() {
		lose(k, "the front matter key "+k)
	}
	for _, g := range s.FileGroups() {
		if strings.HasSuffix(g, "/") {
			lose(g, "the files of a skill directory; the skill's "+g+" files are not written")
		} else {
			lose(g, "a file beside SKILL.md; "+g+" is not written")
		}
	}
	if s.HasProjectionHeader() {
		lose("projection-header", "the decision-projection header that opens SKILL.md; it is neither front matter nor body")
	}
	if s.Hash != "" {
		lose("canonical-hash", "the canonical hash, which is taken over SKILL.md; the exported text is not that file")
	}
	return out
}

// markdownBody is the Markdown of a plain target: the name as the heading, the
// description, then the body of the skill.
func markdownBody(s Skill) []byte {
	var b strings.Builder
	b.WriteString("# " + s.Name + "\n\n")
	b.WriteString(strings.TrimSpace(s.Description) + "\n")
	if body := strings.Trim(s.Body, "\n"); body != "" {
		b.WriteString("\n" + body + "\n")
	}
	return []byte(b.String())
}

func needSkill(s Skill) error {
	if s.Name == "" || s.Description == "" {
		return errNoSkill
	}
	return nil
}

// AgentsMD writes AGENTS.md: one plain Markdown file, no front matter.
type AgentsMD struct{}

func (AgentsMD) Name() string { return "agents-md" }

func (AgentsMD) Export(s Skill) ([]File, []Loss, error) {
	if err := needSkill(s); err != nil {
		return nil, nil, err
	}
	return []File{{Path: "AGENTS.md", Data: markdownBody(s), Mode: 0o644}},
		textLosses(s, "AGENTS.md", "name", "description"), nil
}

// CopilotInstructions writes .github/copilot-instructions.md: one plain
// Markdown file, no front matter.
type CopilotInstructions struct{}

func (CopilotInstructions) Name() string { return "copilot-instructions" }

func (CopilotInstructions) Export(s Skill) ([]File, []Loss, error) {
	if err := needSkill(s); err != nil {
		return nil, nil, err
	}
	return []File{{Path: ".github/copilot-instructions.md", Data: markdownBody(s), Mode: 0o644}},
		textLosses(s, "copilot-instructions.md", "name", "description"), nil
}

// CursorRules writes .cursor/rules/<name>.mdc: front matter with description,
// globs and alwaysApply, then Markdown. The rule is written to apply when the
// agent judges the description relevant: no globs, alwaysApply false.
type CursorRules struct{}

func (CursorRules) Name() string { return "cursor-rules" }

type cursorFrontMatter struct {
	Description string `yaml:"description"`
	Globs       string `yaml:"globs"`
	AlwaysApply bool   `yaml:"alwaysApply"`
}

func (CursorRules) Export(s Skill) ([]File, []Loss, error) {
	if err := needSkill(s); err != nil {
		return nil, nil, err
	}
	fm, err := yaml.Marshal(cursorFrontMatter{Description: strings.TrimSpace(s.Description)})
	if err != nil {
		return nil, nil, err
	}
	body := strings.Trim(s.Body, "\n")
	text := "---\n" + string(fm) + "---\n"
	if body != "" {
		text += "\n" + body + "\n"
	}
	return []File{{Path: ".cursor/rules/" + s.Name + ".mdc", Data: []byte(text), Mode: 0o644}},
		textLosses(s, "a Cursor rule", "name", "description"), nil
}
