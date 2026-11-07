package adapter

import (
	"strings"
	"testing"

	"github.com/saphan-ai/skills-studio/exportfmt"
	"github.com/saphan-ai/skills-studio/exportfmt/exporttest"
)

// The conformance test every adapter runs.
func TestConformance(t *testing.T) { exporttest.Conformance(t, PlainText{}) }

// The case the adapter refuses, in a sentence.
func TestRefusesAnUnreadSkill(t *testing.T) {
	_, _, err := PlainText{}.Export(exportfmt.Skill{})
	if err == nil || !strings.Contains(err.Error(), "no name") {
		t.Fatalf("err = %v, want a refusal that says the skill has no name", err)
	}
}

// Registering it is one entry beside the built-ins, and the registry then
// serves it by name.
func TestRegisteredBesideTheBuiltins(t *testing.T) {
	reg, err := exportfmt.NewRegistry(append(exportfmt.Builtins(), PlainText{})...)
	if err != nil {
		t.Fatal(err)
	}
	skill, err := exportfmt.Load("../valid/with-references")
	if err != nil {
		t.Fatal(err)
	}
	res, err := reg.Export("plain-text", skill)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 1 || res.Files[0].Path != "with-references.txt" {
		t.Fatalf("files = %+v", res.Files)
	}
	var items []string
	for _, l := range res.Losses {
		items = append(items, l.Item)
	}
	if got := strings.Join(items, " "); got != "assets/ references/ canonical-hash" {
		t.Errorf("losses = %q", got)
	}
}
