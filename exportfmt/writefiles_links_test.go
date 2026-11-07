package exportfmt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/saphan-ai/skills-studio/exportfmt"
)

func entriesUnder(t *testing.T, dir string) []string {
	t.Helper()
	var got []string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && p != dir {
			got = append(got, p)
		}
		return nil
	})
	return got
}

// Mutated field: a path component under --out that is a symbolic link to a
// directory outside it. The write must be refused and nothing may appear in
// the target.
func TestWriteFilesRefusesALinkInsideOut(t *testing.T) {
	cases := []struct {
		name string
		link string // path of the link under out
		file string // file the export writes
	}{
		{"first component", ".cursor", ".cursor/rules/demo.mdc"},
		{"last directory", "demo", "demo/SKILL.md"},
		{"nested", "demo/references", "demo/references/r.md"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, outside := t.TempDir(), t.TempDir()
			if err := os.MkdirAll(filepath.Dir(filepath.Join(out, c.link)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(out, c.link)); err != nil {
				t.Skipf("cannot make a symbolic link here: %v", err)
			}
			_, err := exportfmt.WriteFiles(out, []exportfmt.File{{Path: c.file, Data: []byte("x\n"), Mode: 0o644}})
			if err == nil {
				t.Fatalf("WriteFiles followed the link %s and succeeded; wrote %q outside --out", c.link, entriesUnder(t, outside))
			}
			if !strings.Contains(err.Error(), "link") {
				t.Errorf("the refusal does not name a link: %v", err)
			}
			if got := entriesUnder(t, outside); len(got) != 0 {
				t.Errorf("files appeared outside --out: %q", got)
			}
		})
	}
}

// A link that is --out itself is the operator's choice and is followed.
func TestWriteFilesFollowsALinkedOutDirectory(t *testing.T) {
	real, base := t.TempDir(), t.TempDir()
	out := filepath.Join(base, "out")
	if err := os.Symlink(real, out); err != nil {
		t.Skipf("cannot make a symbolic link here: %v", err)
	}
	if _, err := exportfmt.WriteFiles(out, []exportfmt.File{{Path: "a/b.md", Data: []byte("x"), Mode: 0o644}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(real, "a", "b.md")); err != nil {
		t.Fatal(err)
	}
}
