package exporttest_test

import (
	"strings"
	"testing"

	"github.com/saphan-ai/skills-studio/exportfmt"
	"github.com/saphan-ai/skills-studio/exportfmt/exporttest"
)

// Negative controls (law #27): each wraps a conforming built-in, mutates
// exactly one thing the conformance test guards, and the test must go red on
// that thing and on nothing else.

func wantProblem(t *testing.T, a exportfmt.Adapter, substr string) {
	t.Helper()
	got := exporttest.Check(a)
	for _, p := range got {
		if strings.Contains(p, substr) {
			t.Logf("red as required: %s", p)
			return
		}
	}
	t.Fatalf("Check did not report %q; it reported %q", substr, got)
}

// Mutated field: the Loss entry with Item "license". The adapter writes no
// license and no longer says so.
type dropsLossEntry struct {
	exportfmt.Adapter
	item string
}

func (d dropsLossEntry) Export(s exportfmt.Skill) ([]exportfmt.File, []exportfmt.Loss, error) {
	files, losses, err := d.Adapter.Export(s)
	var kept []exportfmt.Loss
	for _, l := range losses {
		if l.Item != d.item {
			kept = append(kept, l)
		}
	}
	return files, kept, err
}

func TestControlAUnnamedDroppedFieldFails(t *testing.T) {
	if p := exporttest.Check(exportfmt.CursorRules{}); len(p) != 0 {
		t.Fatalf("the unmutated adapter is not green: %q", p)
	}
	wantProblem(t, dropsLossEntry{exportfmt.CursorRules{}, "license"}, "license is neither in the output nor named as a loss")
	wantProblem(t, dropsLossEntry{exportfmt.AgentsMD{}, "references/"}, "references/guide.md is neither in the output nor named as a loss")
	wantProblem(t, dropsLossEntry{exportfmt.CopilotInstructions{}, "canonical-hash"}, "canonical-hash is neither in the output nor named as a loss")
}

// Mutated field: the front matter of the file an adapter writes. The description
// is cut from the output while the loss list stays as it was.
type dropsDescription struct{ exportfmt.Adapter }

func (d dropsDescription) Export(s exportfmt.Skill) ([]exportfmt.File, []exportfmt.Loss, error) {
	s.Description = "x"
	files, losses, err := d.Adapter.Export(s)
	return files, losses, err
}

func TestControlAChangedContentFails(t *testing.T) {
	wantProblem(t, dropsDescription{exportfmt.AgentsMD{}}, "description is neither in the output nor named as a loss")
}

// Mutated field: the Item of a loss, which names a piece the skill does not have.
type namesNothing struct{ exportfmt.Adapter }

func (n namesNothing) Export(s exportfmt.Skill) ([]exportfmt.File, []exportfmt.Loss, error) {
	files, losses, err := n.Adapter.Export(s)
	return files, append(losses, exportfmt.Loss{Item: "colour", Reason: "the target has no colour"}), err
}

func TestControlALossNamingNothingFails(t *testing.T) {
	wantProblem(t, namesNothing{exportfmt.AgentsMD{}}, `the loss "colour" names nothing this skill has`)
}

// Mutated field: the bytes of the output, which change on every call.
type counter struct {
	exportfmt.Adapter
	n *int
}

func (c counter) Export(s exportfmt.Skill) ([]exportfmt.File, []exportfmt.Loss, error) {
	files, losses, err := c.Adapter.Export(s)
	*c.n++
	for i := range files {
		files[i].Data = append(append([]byte(nil), files[i].Data...), []byte(strings.Repeat("\n", *c.n))...)
	}
	return files, losses, err
}

func TestControlCNondeterministicAdapterFails(t *testing.T) {
	n := 0
	wantProblem(t, counter{exportfmt.AgentsMD{}, &n}, "not deterministic")
}

// Mutated field: the order of the files returned, taken from a map.
type mapOrder struct{ exportfmt.Adapter }

func (m mapOrder) Export(s exportfmt.Skill) ([]exportfmt.File, []exportfmt.Loss, error) {
	files, losses, err := m.Adapter.Export(s)
	byPath := map[string]exportfmt.File{}
	for _, f := range files {
		byPath[f.Path] = f
	}
	files = files[:0]
	for _, f := range byPath { // Go randomises this order
		files = append(files, f)
	}
	return files, losses, err
}

func TestControlCMapOrderFails(t *testing.T) {
	// agentskills writes several files, so a random order shows within a few runs.
	failed := false
	for i := 0; i < 20 && !failed; i++ {
		failed = len(exporttest.Check(mapOrder{exportfmt.AgentSkills{}})) > 0
	}
	if !failed {
		t.Fatal("a map-ordered output passed conformance twenty times")
	}
}

// Mutated field: the file an importer returns. It loses a file and says nothing.
type lyingImporter struct{ exportfmt.AgentSkills }

func (l lyingImporter) Import(dir string) (exportfmt.Skill, error) {
	s, err := l.AgentSkills.Import(dir)
	s.Files = s.Files[:0]
	return s, err
}

func TestControlRoundTripLossFails(t *testing.T) {
	wantProblem(t, lyingImporter{}, "the round trip does not return the skill")
}

// Mutated field: the Loss entry with Item "projection-header". The fixture
// opens with a projection header; the adapter drops it and no longer says so.
func TestControlAProjectionHeaderLossRemovedFails(t *testing.T) {
	for _, a := range []exportfmt.Adapter{exportfmt.CursorRules{}, exportfmt.AgentsMD{}, exportfmt.CopilotInstructions{}} {
		wantProblem(t, dropsLossEntry{a, "projection-header"}, "projection-header is neither in the output nor named as a loss")
	}
}

// Mutated field: the path of a file the adapter returns, which climbs out of
// the output directory. Check must say the output cannot be written.
type escapesOut struct{ exportfmt.Adapter }

func (e escapesOut) Export(s exportfmt.Skill) ([]exportfmt.File, []exportfmt.Loss, error) {
	files, losses, err := e.Adapter.Export(s)
	if err == nil {
		files = append(append([]exportfmt.File(nil), files...), exportfmt.File{Path: "../x", Data: []byte("x"), Mode: 0o644})
	}
	return files, losses, err
}

func TestControlUnwritableOutputFails(t *testing.T) {
	wantProblem(t, escapesOut{exportfmt.AgentsMD{}}, "cannot be written")
}

// An adapter that accepts the skill nobody read.
type acceptsAnything struct{}

func (acceptsAnything) Name() string { return "accepts-anything" }
func (acceptsAnything) Export(exportfmt.Skill) ([]exportfmt.File, []exportfmt.Loss, error) {
	return nil, nil, nil
}

func TestControlZeroSkillAccepted(t *testing.T) {
	wantProblem(t, acceptsAnything{}, "never read")
}
