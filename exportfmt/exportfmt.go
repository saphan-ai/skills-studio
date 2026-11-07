// Package exportfmt writes a skill in the format another tool reads.
//
// One format is one adapter: one Go type behind the small Adapter interface.
// A registry lists the adapters; adding a format means a type, its tests, one
// line in Builtins and a section in docs/ADAPTERS.md. Nothing shared changes.
//
// An adapter never touches the disk and never reads the skill directory. It
// receives a Skill and returns the files it would write and the Losses: every
// piece of the skill the target cannot carry. The caller validates what an
// adapter returned (ValidateOutput) before a byte is written, so an adapter
// written by anyone cannot write outside the output directory.
package exportfmt

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
)

// File is one file an export writes, or one file of the skill besides SKILL.md.
type File struct {
	// Path is relative to the root, slash separated, clean, never starting with
	// "/" or "..".
	Path string
	Data []byte
	// Mode carries only the permission bits; an export writes 0644, or 0755
	// when any execute bit is set.
	Mode fs.FileMode
}

// Skill is a skill directory that was read and is valid Agent Skills (level 1).
type Skill struct {
	Name          string
	Description   string
	License       string
	Compatibility string
	AllowedTools  string
	// Metadata is the metadata map; the specification makes every value a string.
	Metadata map[string]string
	// Extra holds every other front matter key, in the JSON data model: a key
	// the Agent Skills page does not define.
	Extra map[string]any
	// Body is the Markdown after the front matter, with LF line ends.
	Body string
	// Files is every file of the directory besides SKILL.md, sorted by path.
	Files []File
	// SkillMD is the SKILL.md exactly as it was read.
	SkillMD []byte
	// Hash is the canonical hash; empty unless the skill is also in the Saphan
	// profile (level 2).
	Hash string
}

// Loss is one piece of the skill an export does not carry.
type Loss struct {
	// Item names the piece: a front matter key ("license", "compatibility",
	// "allowed-tools", "metadata", or an extra key), "canonical-hash", or a
	// file ("LICENSE.txt") or a top level directory ending in "/"
	// ("references/") whose files are not carried.
	Item string
	// Reason is one sentence: why the target cannot carry it.
	Reason string
}

// String is the one line the command prints.
func (l Loss) String() string { return l.Item + ": " + l.Reason }

// Adapter writes a skill in one format.
type Adapter interface {
	// Name is the value of --format: lowercase letters, digits and hyphens.
	Name() string
	// Export returns the files to write and every loss. It must be
	// deterministic, must not touch the disk, and must refuse (return an error
	// that is one sentence) a skill it cannot write, such as the zero Skill.
	Export(s Skill) ([]File, []Loss, error)
}

// Importer is optional. An adapter whose format can be read back as a skill
// also satisfies it; the conformance test then runs a round trip.
type Importer interface {
	Adapter
	// Import reads what Export wrote. dir is the directory Export's files were
	// written into.
	Import(dir string) (Skill, error)
}

var namePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Registry is a set of adapters by name.
type Registry struct {
	byName map[string]Adapter
}

// NewRegistry refuses a name that is not lowercase letters, digits and
// hyphens, an empty list entry, and a name registered twice.
func NewRegistry(adapters ...Adapter) (*Registry, error) {
	r := &Registry{byName: map[string]Adapter{}}
	for _, a := range adapters {
		if a == nil {
			return nil, fmt.Errorf("a nil adapter cannot be registered")
		}
		n := a.Name()
		if !namePattern.MatchString(n) {
			return nil, fmt.Errorf("the format name %q is not lowercase letters, digits and hyphens", n)
		}
		if _, dup := r.byName[n]; dup {
			return nil, fmt.Errorf("the format %q is registered twice", n)
		}
		r.byName[n] = a
	}
	return r, nil
}

// Get returns the adapter registered under name.
func (r *Registry) Get(name string) (Adapter, bool) {
	a, ok := r.byName[name]
	return a, ok
}

// Names lists the registered format names, sorted.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.byName))
	for n := range r.byName {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// UnknownFormatError is the refusal of a name nobody registered.
type UnknownFormatError struct {
	Name       string
	Registered []string
}

func (e *UnknownFormatError) Error() string {
	return fmt.Sprintf("unknown format %q; registered formats: %s", e.Name, strings.Join(e.Registered, ", "))
}

// Result is a validated export.
type Result struct {
	Files  []File
	Losses []Loss
}

// Export runs the adapter named format and validates what it returned.
func (r *Registry) Export(format string, s Skill) (Result, error) {
	a, ok := r.Get(format)
	if !ok {
		return Result{}, &UnknownFormatError{Name: format, Registered: r.Names()}
	}
	files, losses, err := a.Export(s)
	if err != nil {
		return Result{}, fmt.Errorf("format %s refuses this skill: %w", format, err)
	}
	if problems := ValidateOutput(files, losses); len(problems) > 0 {
		return Result{}, fmt.Errorf("format %s returned an output that cannot be written: %s", format, strings.Join(problems, "; "))
	}
	return Result{Files: files, Losses: losses}, nil
}

// ValidateOutput lists what is wrong with an adapter's return: a path that
// leaves the root or is not clean, a path twice, a path that is a directory
// of another path, a loss without an item or a reason.
func ValidateOutput(files []File, losses []Loss) []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range files {
		switch {
		case f.Path == "" || f.Path != path.Clean(f.Path) || strings.HasPrefix(f.Path, "/") ||
			f.Path == ".." || strings.HasPrefix(f.Path, "../") || strings.Contains(f.Path, `\`) || strings.ContainsRune(f.Path, 0):
			out = append(out, fmt.Sprintf("the path %q is not a clean relative slash path inside the output directory", f.Path))
		case seen[f.Path]:
			out = append(out, fmt.Sprintf("the path %q is returned twice", f.Path))
		}
		seen[f.Path] = true
	}
	for p := range seen {
		for d := path.Dir(p); d != "." && d != "/"; d = path.Dir(d) {
			if seen[d] {
				out = append(out, fmt.Sprintf("the path %q is both a file and a directory", d))
			}
		}
	}
	for _, l := range losses {
		if l.Item == "" || l.Reason == "" || strings.ContainsAny(l.Item+l.Reason, "\n\r") {
			out = append(out, fmt.Sprintf("the loss %q needs an item and a one-line reason", l.String()))
		}
	}
	sort.Strings(out)
	return out
}
