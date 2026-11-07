package exportfmt_test

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/saphan-ai/skills-studio/exportfmt"
)

// examples/export/<format>/ holds what `skills export examples/valid/all-fields
// --format <format>` writes. The test below looks the format up in a registry,
// so a format taken out of the registry turns its sample red.

var updateSamples = flag.Bool("update-samples", false, "rewrite examples/export from the current adapters")

const (
	sampleSkill = "../examples/valid/all-fields"
	sampleRoot  = "../examples/export"
)

// checkSample exports the sample skill with the adapter reg holds under format
// and compares the files with the sample directory, both ways.
func checkSample(reg *exportfmt.Registry, format string) error {
	skill, err := exportfmt.Load(sampleSkill)
	if err != nil {
		return err
	}
	res, err := reg.Export(format, skill)
	if err != nil {
		return err
	}
	want := map[string][]byte{}
	for _, f := range res.Files {
		want[f.Path] = f.Data
	}
	root := filepath.Join(sampleRoot, format)
	have := map[string][]byte{}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		data, rerr := os.ReadFile(p)
		have[filepath.ToSlash(rel)] = data
		return rerr
	})
	if err != nil {
		return fmt.Errorf("the sample directory %s cannot be read: %w", root, err)
	}
	var problems []string
	for p, data := range want {
		if got, ok := have[p]; !ok {
			problems = append(problems, p+" is exported and absent from the sample")
		} else if !bytes.Equal(got, data) {
			problems = append(problems, p+" differs from the sample")
		}
	}
	for p := range have {
		if _, ok := want[p]; !ok {
			problems = append(problems, p+" is in the sample and not exported")
		}
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

func TestSamples(t *testing.T) {
	reg := exportfmt.Default()
	if *updateSamples {
		skill, err := exportfmt.Load(sampleSkill)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range reg.Names() {
			res, err := reg.Export(name, skill)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(sampleRoot, name)
			if err := os.RemoveAll(dir); err != nil {
				t.Fatal(err)
			}
			if _, err := exportfmt.WriteFiles(dir, res.Files); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, name := range reg.Names() {
		t.Run(name, func(t *testing.T) {
			if err := checkSample(reg, name); err != nil {
				t.Fatal(err)
			}
		})
	}
	// Every sample directory belongs to a registered format.
	entries, err := os.ReadDir(sampleRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if _, ok := reg.Get(e.Name()); !ok && e.IsDir() {
			t.Errorf("examples/export/%s is a sample of a format that is not registered", e.Name())
		}
	}
}

// Negative control (law #27): the field mutated is the registry's list of
// adapters. With cursor-rules taken out, its sample's test is red, and the
// samples of the formats still registered stay green.
func TestSampleFailsWhenAdapterLeavesTheRegistry(t *testing.T) {
	var without []exportfmt.Adapter
	for _, a := range exportfmt.Builtins() {
		if a.Name() != "cursor-rules" {
			without = append(without, a)
		}
	}
	reg, err := exportfmt.NewRegistry(without...)
	if err != nil {
		t.Fatal(err)
	}
	err = checkSample(reg, "cursor-rules")
	if err == nil || !strings.Contains(err.Error(), "unknown format") {
		t.Fatalf("the cursor-rules sample passed with the adapter out of the registry: err = %v", err)
	}
	t.Logf("red as required: %v", err)
	if err := checkSample(reg, "agents-md"); err != nil {
		t.Errorf("the agents-md sample went red for a different cause: %v", err)
	}
}
