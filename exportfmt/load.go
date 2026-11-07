package exportfmt

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/saphan-ai/skills-studio/internal/skillfmt"
)

// Bounds of this program, not rules of the Agent Skills page. A skill over one
// is refused by name, never truncated.
const (
	MaxFileBytes  = 4 << 20
	MaxTotalBytes = 32 << 20
	MaxFiles      = 1000
)

// RefusedError is a skill that is not valid Agent Skills (level 1), or that
// this program could not read whole.
type RefusedError struct {
	Violations []skillfmt.Violation
}

// Unread reports that every violation is export's own (RuleExportUnread): the
// skill may be valid Agent Skills, and `skills verify` may pass it, but export
// could not read it whole.
func (e *RefusedError) Unread() bool {
	for _, v := range e.Violations {
		if v.Rule != RuleExportUnread {
			return false
		}
	}
	return len(e.Violations) > 0
}

func (e *RefusedError) Error() string {
	var parts []string
	for _, v := range e.Violations {
		parts = append(parts, fmt.Sprintf("%s: %s: %s", v.Rule, v.Path, v.Message))
	}
	if e.Unread() {
		return "export could not read the skill: " + strings.Join(parts, "; ")
	}
	return "the skill is not valid Agent Skills: " + strings.Join(parts, "; ")
}

// Rule identifier of a refusal that belongs to export, not to the format.
const RuleExportUnread = "EXPORT-UNREAD"

func unread(p, format string, a ...any) error {
	return &RefusedError{Violations: []skillfmt.Violation{{Rule: RuleExportUnread, Path: p, Message: fmt.Sprintf(format, a...)}}}
}

// Load reads a skill directory. It refuses, by the named rule of level 1 of
// `skills verify`, a skill that is not valid Agent Skills; it refuses a file
// that is not a regular file (a symbolic link is never followed) and a skill
// over the bounds above. It is also the reader an importer uses.
func Load(dir string) (Skill, error) {
	res, err := skillfmt.Verify(dir)
	if err != nil {
		return Skill{}, err
	}
	if res.Level == 0 {
		return Skill{}, &RefusedError{Violations: res.Level1}
	}
	root := filepath.Clean(dir)
	var files []File
	total := 0
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if rel == "." || d.IsDir() {
			return nil
		}
		if rel == "SKILL.md" {
			return nil
		}
		if !d.Type().IsRegular() {
			return unread(rel, "not a regular file; export does not follow links or read special files")
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > MaxFileBytes {
			return unread(rel, "%d bytes; export reads at most %d bytes of one file", info.Size(), MaxFileBytes)
		}
		total += int(info.Size())
		if total > MaxTotalBytes || len(files) >= MaxFiles {
			return unread(rel, "the skill holds more than %d files or %d bytes", MaxFiles, MaxTotalBytes)
		}
		data, err := readBounded(p)
		if err != nil {
			return err
		}
		files = append(files, File{Path: rel, Data: data, Mode: info.Mode().Perm()})
		return nil
	})
	if err != nil {
		return Skill{}, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	md, err := readBounded(filepath.Join(root, "SKILL.md"))
	if err != nil {
		return Skill{}, err
	}
	doc, body, vs := skillfmt.SplitSkillMD(md)
	if vs != nil {
		return Skill{}, &RefusedError{Violations: vs}
	}
	s := Skill{Body: body, Files: files, SkillMD: md, Hash: res.Hash}
	for k, v := range doc {
		str, _ := v.(string)
		switch k {
		case "name":
			s.Name = str
		case "description":
			s.Description = str
		case "license":
			s.License = str
		case "compatibility":
			s.Compatibility = str
		case "allowed-tools":
			s.AllowedTools = str
		case "metadata":
			if m, ok := v.(map[string]any); ok && len(m) > 0 {
				s.Metadata = map[string]string{}
				for mk, mv := range m {
					s.Metadata[mk], _ = mv.(string)
				}
			}
		default:
			if s.Extra == nil {
				s.Extra = map[string]any{}
			}
			s.Extra[k] = v
		}
	}
	return s, nil
}

func readBounded(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxFileBytes {
		return nil, unread(filepath.Base(p), "more than %d bytes", MaxFileBytes)
	}
	return data, nil
}

// HasProjectionHeader reports whether SKILL.md, as it was read, opens with a
// decision-projection header. The header is not front matter and not body, so a
// target that writes neither carries it nowhere and has to say so.
func (s Skill) HasProjectionHeader() bool {
	first, _, _ := bytes.Cut(s.SkillMD, []byte("\n"))
	return string(first) == skillfmt.HeaderOpen
}

// ExtraKeys lists the extra front matter keys, sorted.
func (s Skill) ExtraKeys() []string {
	out := make([]string, 0, len(s.Extra))
	for k := range s.Extra {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// MetadataKeys lists the metadata keys, sorted.
func (s Skill) MetadataKeys() []string {
	out := make([]string, 0, len(s.Metadata))
	for k := range s.Metadata {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// FileGroups names the files of the skill by what an adapter reports as a
// loss: a top level directory ("references/") for every file inside one, the
// path for a file at the top. Sorted, once each.
func (s Skill) FileGroups() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range s.Files {
		g := f.Path
		if i := strings.IndexByte(g, '/'); i >= 0 {
			g = g[:i+1]
		}
		if !seen[g] {
			seen[g] = true
			out = append(out, g)
		}
	}
	sort.Strings(out)
	return out
}

// checkNoLink walks the path of one file component by component under out and
// refuses, naming it, a symbolic link on the way and a file that already exists
// at the end. out itself may be a link: it is the operator's own choice.
func checkNoLink(out, rel string) error {
	cur := out
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symbolic link under %s; an export never follows a link, so it would write outside the output directory", strings.Join(parts[:i+1], "/"), out)
		}
		if i == len(parts)-1 {
			return fmt.Errorf("%s already exists under %s; an export never overwrites a file", rel, out)
		}
	}
	return nil
}

// WriteFiles writes a validated export under out. It refuses before it writes
// anything if a file already exists there (an export never overwrites) or a
// symbolic link lies on the path of a file (named in the refusal), and it
// writes through a handle rooted at out, creating files with O_EXCL, so a link
// or a file that appears meanwhile is not followed or replaced. out itself is
// created if absent. It returns the paths written, in order.
func WriteFiles(out string, files []File) ([]string, error) {
	if problems := ValidateOutput(files, nil); len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}
	for _, f := range files {
		if err := checkNoLink(out, f.Path); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, err
	}
	// Every write goes through a handle rooted at out: a path component that is
	// (or becomes) a symbolic link out of it is refused by the OS call itself,
	// not only by the check above.
	root, err := os.OpenRoot(out)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	var written []string
	for _, f := range files {
		rel := filepath.FromSlash(f.Path)
		if err := root.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
			return written, fmt.Errorf("%s: %w (a symbolic link under %s is never followed)", f.Path, err, out)
		}
		perm := fs.FileMode(0o644)
		if f.Mode&0o111 != 0 {
			perm = 0o755
		}
		h, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if err != nil {
			return written, err
		}
		_, werr := h.Write(f.Data)
		cerr := h.Close()
		if werr != nil {
			return written, werr
		}
		if cerr != nil {
			return written, cerr
		}
		// O_CREATE is subject to the umask; the mode is the adapter's to state.
		if err := root.Chmod(rel, perm); err != nil {
			return written, err
		}
		written = append(written, path.Clean(f.Path))
	}
	return written, nil
}
