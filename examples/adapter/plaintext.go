// Package adapter is a worked example of an export format: one Go file and its
// test. It writes <name>.txt, the description and the body as plain text, and
// names every other piece of the skill as a loss. Copy it, rename it, change
// Export. See docs/ADAPTERS.md, "Adding a format".
package adapter

import (
	"errors"
	"strings"

	"github.com/saphan-ai/skills-studio/exportfmt"
)

// PlainText is the adapter. It holds no state.
type PlainText struct{}

// A compile-time assertion: this type is an adapter.
var _ exportfmt.Adapter = PlainText{}

// Name is the value of --format.
func (PlainText) Name() string { return "plain-text" }

// Export is deterministic and touches no disk. It refuses a skill that was not
// read by exportfmt.Load, in one sentence.
func (PlainText) Export(s exportfmt.Skill) ([]exportfmt.File, []exportfmt.Loss, error) {
	if s.Name == "" || s.Description == "" {
		return nil, nil, errors.New("the skill has no name; export needs a skill read by exportfmt.Load")
	}
	text := strings.TrimSpace(s.Description) + "\n"
	if body := strings.Trim(s.Body, "\n"); body != "" {
		text += "\n" + body + "\n"
	}
	files := []exportfmt.File{{Path: s.Name + ".txt", Data: []byte(text), Mode: 0o644}}

	// Every piece this format has no place for is a loss, one line each. The
	// conformance test fails an adapter that drops a piece without saying so.
	var losses []exportfmt.Loss
	lose := func(item string) {
		losses = append(losses, exportfmt.Loss{Item: item, Reason: "a plain text file has no place for " + item})
	}
	if s.License != "" {
		lose("license")
	}
	if s.Compatibility != "" {
		lose("compatibility")
	}
	if s.AllowedTools != "" {
		lose("allowed-tools")
	}
	if len(s.Metadata) > 0 {
		lose("metadata")
	}
	for _, k := range s.ExtraKeys() {
		lose(k)
	}
	for _, g := range s.FileGroups() {
		lose(g)
	}
	if s.HasProjectionHeader() {
		lose("projection-header")
	}
	if s.Hash != "" {
		lose("canonical-hash")
	}
	return files, losses, nil
}
