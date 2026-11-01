// Package skillcheck runs deterministic static checks over a skill directory.
//
// It reads files and nothing else: no model, no network. Each rule matches a
// literal pattern in text, so a clean report is evidence that those patterns
// are absent and is never a statement about what the skill does. Every report
// carries remarks saying what a text scan cannot see.
package skillcheck

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/saphan-ai/skills-studio/internal/skillfmt"
)

// Severity of a finding.
type Severity string

const (
	// Refuse means the skill must not be accepted as it stands.
	Refuse Severity = "refuse"
	// Flag means a human should read the line before accepting the skill.
	Flag Severity = "flag"
)

// Limits that keep the walk bounded. A tree past a limit is an error, never a
// partial read presented as a whole.
const (
	MaxFileBytes = 8 << 20
	MaxEntries   = 10000
	MaxDepth     = 16
	// MaxTotalBytes bounds the bytes read across the whole tree, so the work
	// and the memory of one run do not grow with the number of files.
	MaxTotalBytes = 64 << 20
	// DefaultMaxDomains is the default for Config.MaxDomains.
	DefaultMaxDomains = 10
)

// totalLimit is MaxTotalBytes; a test lowers it so the bound is exercised
// without reading 64 MiB.
var totalLimit = MaxTotalBytes

// Config holds the tunable defaults.
type Config struct {
	// MaxDomains is the most comma- or semicolon-separated alternatives one
	// description sentence may list before SC-DESCRIPTION-OVERCLAIM flags it.
	MaxDomains int
}

// Finding is one thing a rule saw.
type Finding struct {
	Rule     string   `json:"rule"`
	Path     string   `json:"path"`
	Line     int      `json:"line"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

// Report is the outcome of one run.
type Report struct {
	// Findings are sorted by path, line, rule.
	Findings []Finding `json:"findings"`
	// Remarks are the measurement caveats of this run, one line each, without
	// the "remark: " prefix.
	Remarks []string `json:"remarks"`
	// Files is the number of regular files read as text.
	Files int `json:"files_scanned"`
	// Rules lists the rule ids that ran.
	Rules []string `json:"rules_run"`
}

// Refused reports whether any finding is a refuse.
func (r Report) Refused() bool {
	for _, f := range r.Findings {
		if f.Severity == Refuse {
			return true
		}
	}
	return false
}

// file is one regular file of the skill.
type file struct {
	path string // slash-separated, relative to the skill root
	mode fs.FileMode
	data []byte // the whole file when text; else only its first bytes (headerBytes)
	text bool   // valid UTF-8, no NUL, not truncated
	why  string // when !text: why it was not scanned as text
}

// skill is the tree under one directory.
type skill struct {
	files    []file
	dirs     []string // slash-separated relative paths of directories
	symlinks []string
	special  []string // entries that are neither regular files, directories nor links

	// front matter, read with the format's own functions
	fm        map[string]any
	fmErr     string // empty when fm was read
	fmLineOff int    // lines of projection header before the canonical text
	fmLines   []string
}

// Check reads the skill directory and runs every rule.
func Check(dir string, cfg Config) (Report, error) {
	if cfg.MaxDomains <= 0 {
		cfg.MaxDomains = DefaultMaxDomains
	}
	s, err := load(dir)
	if err != nil {
		return Report{}, err
	}
	var rep Report
	for _, r := range Rules {
		rep.Rules = append(rep.Rules, r.ID)
		rep.Findings = append(rep.Findings, r.Check(s, cfg)...)
	}
	sort.SliceStable(rep.Findings, func(i, j int) bool {
		a, b := rep.Findings[i], rep.Findings[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Rule < b.Rule
	})
	for _, f := range s.files {
		if f.text {
			rep.Files++
		}
	}
	rep.Remarks = remarks(s, rep.Rules)
	return rep, nil
}

func load(dir string) (*skill, error) {
	clean := filepath.Clean(dir)
	info, err := os.Lstat(clean)
	if err != nil {
		return nil, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s is a symbolic link; give the skill directory itself", dir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	root, err := filepath.Abs(clean)
	if err != nil {
		return nil, err
	}
	s := &skill{}
	entries := 0
	total := 0
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if p == root {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		entries++
		if entries > MaxEntries {
			return fmt.Errorf("more than %d entries under %s; the check does not read a tree this large", MaxEntries, dir)
		}
		if strings.Count(rel, "/")+1 > MaxDepth {
			return fmt.Errorf("%s is deeper than %d levels; the check does not read a tree this deep", rel, MaxDepth)
		}
		switch t := d.Type(); {
		case t&fs.ModeSymlink != 0:
			s.symlinks = append(s.symlinks, rel)
		case d.IsDir():
			s.dirs = append(s.dirs, rel)
		case t.IsRegular():
			fi, err := d.Info()
			if err != nil {
				return err
			}
			f, n := readFile(p, rel, fi.Mode())
			total += n
			if total > totalLimit {
				return fmt.Errorf("more than %d MiB to read under %s; the check does not read a tree this large", totalLimit>>20, dir)
			}
			s.files = append(s.files, f)
		default:
			s.special = append(s.special, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.readFrontMatter()
	return s, nil
}

// headerBytes is how much of a file that is not text is kept: enough for a
// shebang and a magic number.
const headerBytes = 16

// readFile reads one regular file and returns it with the number of bytes read.
// A file that cannot be read is a file that was not scanned, never an error of
// the run: the report names it.
func readFile(p, rel string, mode fs.FileMode) (file, int) {
	f := file{path: rel, mode: mode}
	fh, err := os.Open(p)
	if err != nil {
		f.why = "unreadable: " + oneLine(err.Error())
		return f, 0
	}
	defer fh.Close()
	data, err := io.ReadAll(io.LimitReader(fh, MaxFileBytes+1))
	if err != nil {
		f.why = "unreadable: " + oneLine(err.Error())
		return f, len(data)
	}
	n := len(data)
	switch {
	case n > MaxFileBytes:
		f.why = fmt.Sprintf("over %d MiB", MaxFileBytes>>20)
	case !utf8.Valid(data):
		f.why = "not valid UTF-8"
	case strings.IndexByte(string(data), 0) >= 0:
		f.why = "contains NUL"
	default:
		f.data = data
		f.text = true
		return f, n
	}
	if len(data) > headerBytes {
		data = data[:headerBytes]
	}
	f.data = data
	return f, n
}

func (s *skill) readFrontMatter() {
	var f *file
	for i := range s.files {
		if s.files[i].path == "SKILL.md" {
			f = &s.files[i]
		}
	}
	if f == nil {
		s.fmErr = "there is no SKILL.md"
		return
	}
	if !f.text {
		s.fmErr = "SKILL.md is not UTF-8 text"
		return
	}
	if v := skillfmt.CheckText(f.data); v != nil {
		s.fmErr = v.Rule + ": " + v.Message
		return
	}
	canon, v := skillfmt.CanonicalText(f.data)
	if v != nil {
		s.fmErr = v.Rule + ": " + v.Message
		return
	}
	doc, v := skillfmt.ParseFrontMatter(canon)
	if v != nil {
		s.fmErr = v.Rule + ": " + v.Message
		return
	}
	s.fm = doc
	s.fmLineOff = strings.Count(string(f.data[:len(f.data)-len(canon)]), "\n")
	s.fmLines = strings.Split(string(canon), "\n")
}

// fmLine returns the 1-based line of SKILL.md where front matter key starts.
func (s *skill) fmLine(key string) int {
	for i, l := range s.fmLines {
		if strings.HasPrefix(l, key+":") {
			return i + 1 + s.fmLineOff
		}
	}
	return 1
}

// lines splits text into lines; line n of the file is element n-1.
func lines(f file) []string { return strings.Split(string(f.data), "\n") }
