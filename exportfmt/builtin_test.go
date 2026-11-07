package exportfmt_test

import (
	"strings"
	"testing"

	"github.com/saphan-ai/skills-studio/exportfmt"
	"github.com/saphan-ai/skills-studio/exportfmt/exporttest"
)

// Every built-in runs the conformance test; the list is the registry's own, so
// an adapter added to Builtins is held to it without a line here.
func TestBuiltinsConform(t *testing.T) {
	for _, a := range exportfmt.Builtins() {
		t.Run(a.Name(), func(t *testing.T) { exporttest.Conformance(t, a) })
	}
}

func TestBuiltinNames(t *testing.T) {
	got := strings.Join(exportfmt.Default().Names(), " ")
	want := "agents-md agentskills copilot-instructions cursor-rules"
	if got != want {
		t.Fatalf("registered names = %q, want %q", got, want)
	}
}

func TestRegistryRefusals(t *testing.T) {
	if _, err := exportfmt.NewRegistry(exportfmt.AgentsMD{}, exportfmt.AgentsMD{}); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Errorf("a name registered twice: err = %v, want a refusal naming it", err)
	}
	if _, err := exportfmt.NewRegistry(badName{}); err == nil || !strings.Contains(err.Error(), "Bad_Name") {
		t.Errorf("a malformed name: err = %v, want a refusal naming it", err)
	}
	if _, err := exportfmt.NewRegistry(nil); err == nil {
		t.Errorf("a nil adapter was registered")
	}
}

type badName struct{ exportfmt.AgentsMD }

func (badName) Name() string { return "Bad_Name" }

func TestUnknownFormatListsRegisteredNames(t *testing.T) {
	_, err := exportfmt.Default().Export("nope", exportfmt.Skill{})
	var u *exportfmt.UnknownFormatError
	if err == nil || !asUnknown(err, &u) {
		t.Fatalf("err = %v, want UnknownFormatError", err)
	}
	for _, n := range exportfmt.Default().Names() {
		if !strings.Contains(err.Error(), n) {
			t.Errorf("the refusal %q does not list %s", err, n)
		}
	}
}

func asUnknown(err error, target **exportfmt.UnknownFormatError) bool {
	u, ok := err.(*exportfmt.UnknownFormatError)
	*target = u
	return ok
}

// A hand-written adapter's paths are checked before a byte is written.
func TestValidateOutputRefusesPathsOutsideTheRoot(t *testing.T) {
	for _, p := range []string{"../x", "/etc/x", "a/../../x", "a//b", "./a", "", `a\b`, ".."} {
		if got := exportfmt.ValidateOutput([]exportfmt.File{{Path: p}}, nil); len(got) == 0 {
			t.Errorf("path %q was accepted", p)
		}
	}
	if got := exportfmt.ValidateOutput([]exportfmt.File{{Path: "a"}, {Path: "a/b"}}, nil); len(got) == 0 {
		t.Errorf("a path that is a file and a directory was accepted")
	}
	if got := exportfmt.ValidateOutput([]exportfmt.File{{Path: "a"}, {Path: "a"}}, nil); len(got) == 0 {
		t.Errorf("a path twice was accepted")
	}
	if got := exportfmt.ValidateOutput(nil, []exportfmt.Loss{{Item: "x"}}); len(got) == 0 {
		t.Errorf("a loss without a reason was accepted")
	}
}

// What each built-in refuses: the skill that was never read, in a sentence.
func TestBuiltinsRefuseTheZeroSkill(t *testing.T) {
	for _, a := range exportfmt.Builtins() {
		_, _, err := a.Export(exportfmt.Skill{})
		if err == nil || !strings.Contains(err.Error(), "no name") {
			t.Errorf("%s: err = %v, want a refusal that says the skill has no name", a.Name(), err)
		}
	}
}
