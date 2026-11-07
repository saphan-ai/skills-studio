// Package exporttest is the conformance test every export adapter runs.
//
//	func TestConformance(t *testing.T) { exporttest.Conformance(t, MyFormat{}) }
//
// It holds an adapter to four properties: its output is the same bytes every
// time; every piece of the skill is either in the output or named as a loss
// (and a loss names a piece the skill has); the output is writable (clean
// relative paths, no path twice); and, when the adapter also imports, a round
// trip returns the skill less what the adapter named as lost. It also checks
// that the adapter refuses a skill that was never read.
package exporttest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/saphan-ai/skills-studio/exportfmt"
)

// Conformance reports every problem Check finds as a test failure.
func Conformance(t *testing.T, a exportfmt.Adapter) {
	t.Helper()
	for _, p := range Check(a) {
		t.Errorf("%s: %s", a.Name(), p)
	}
}

// fixture is one skill directory the checks run on.
type fixture struct {
	name  string
	files map[string]string // path -> content
	exec  []string          // paths that are executable
}

var fixtures = []fixture{
	{
		// Level 2 (the Saphan profile): every front matter field the profile
		// defines, a references/ and an assets/ file, and so a canonical hash.
		name: "fx-full",
		files: map[string]string{
			"SKILL.md":            "<!-- saphan-decision-projection-v1\nZq-header-token: a header line\n-->\n\n---\nname: fx-full\ndescription: Zq-description-token for the conformance fixture.\nlicense: Zq-license-token\ncompatibility: Zq-compat-token\nallowed-tools: Zq-tools-token\nmetadata:\n  saphan.origin: builtin\n  author: Zq-metadata-token\n---\n\n# Fixture\n\nZq-body-token appears in the body.\n",
			"references/guide.md": "Zq-reference-token\n",
			"assets/table.txt":    "Zq-asset-token\n",
		},
	},
	{
		// Level 1 only: a script, a file beside SKILL.md, a front matter key
		// the Agent Skills page does not define.
		name: "fx-open",
		files: map[string]string{
			"SKILL.md":       "---\nname: fx-open\ndescription: Zq-open-description-token.\nx-vendor-key: Zq-extra-token\n---\nZq-open-body-token\n",
			"scripts/run.sh": "#!/bin/sh\necho Zq-script-token\n",
			"LICENSE.txt":    "Zq-licensefile-token\n",
		},
		exec: []string{"scripts/run.sh"},
	},
}

func (f fixture) write(dir string) (string, error) {
	root := filepath.Join(dir, f.name)
	for p, c := range f.files {
		target := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(target, []byte(c), 0o644); err != nil {
			return "", err
		}
	}
	for _, p := range f.exec {
		if err := os.Chmod(filepath.Join(root, filepath.FromSlash(p)), 0o755); err != nil {
			return "", err
		}
	}
	return root, nil
}

// Check runs the conformance checks and returns one line per problem; empty
// means the adapter conforms.
func Check(a exportfmt.Adapter) []string {
	var out []string
	add := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }

	tmp, err := os.MkdirTemp("", "exporttest-")
	if err != nil {
		return []string{"cannot make a temporary directory: " + err.Error()}
	}
	defer os.RemoveAll(tmp)

	if _, _, err := a.Export(exportfmt.Skill{}); err == nil {
		add("Export accepted a skill that was never read (the zero Skill); it must refuse it with an error")
	}

	for _, fx := range fixtures {
		dir, err := fx.write(filepath.Join(tmp, "src"))
		if err != nil {
			add("fixture %s cannot be written: %v", fx.name, err)
			continue
		}
		s, err := exportfmt.Load(dir)
		if err != nil {
			add("fixture %s is not readable by Load: %v", fx.name, err)
			continue
		}
		for _, p := range checkOne(a, fx.name, dir, s, tmp) {
			add("%s: %s", fx.name, p)
		}
	}
	return out
}

func checkOne(a exportfmt.Adapter, fx, dir string, s exportfmt.Skill, tmp string) []string {
	var out []string
	add := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }

	files, losses, err := a.Export(s)
	if err != nil {
		return []string{"Export refused a valid skill: " + err.Error()}
	}
	// Determinism: the same Skill twice, and the same directory read afresh
	// (maps are rebuilt, so a map order that leaks into the output shows).
	for i := 0; i < 3; i++ {
		src := s
		if i > 0 {
			var lerr error
			if src, lerr = exportfmt.Load(dir); lerr != nil {
				return append(out, "the fixture could not be read again: "+lerr.Error())
			}
		}
		f2, l2, err2 := a.Export(src)
		if err2 != nil {
			add("Export succeeded once and refused the same skill again: %v", err2)
			break
		}
		if !sameFiles(files, f2) || !reflect.DeepEqual(losses, l2) {
			add("Export is not deterministic: the same skill gave different files or losses on run %d", i+2)
			break
		}
	}
	for _, p := range exportfmt.ValidateOutput(files, losses) {
		add("the output cannot be written: %s", p)
	}

	// Every piece of the skill is carried or named as lost; a loss names a piece.
	pieces := piecesOf(s)
	lost := map[string]bool{}
	for _, l := range losses {
		matched := false
		for _, p := range pieces {
			if p.matchesLoss(l.Item) {
				matched = true
				lost[p.item] = true
			}
		}
		if !matched {
			add("the loss %q names nothing this skill has", l.Item)
		}
	}
	for _, p := range pieces {
		if lost[p.item] || p.carried(files) {
			continue
		}
		add("%s is neither in the output nor named as a loss", p.item)
	}

	// Round trip, when the adapter imports.
	if imp, ok := a.(exportfmt.Importer); ok {
		outDir := filepath.Join(tmp, "out-"+fx)
		if _, err := exportfmt.WriteFiles(outDir, files); err != nil {
			return append(out, "the output could not be written for the round trip: "+err.Error())
		}
		back, err := imp.Import(outDir)
		if err != nil {
			return append(out, "Import could not read what Export wrote: "+err.Error())
		}
		want := withoutLost(s, losses)
		if len(losses) > 0 {
			want.SkillMD, back.SkillMD = nil, nil
		}
		if !reflect.DeepEqual(want, back) {
			add("the round trip does not return the skill less its named losses:\n  want %s\n  got  %s", brief(want), brief(back))
		}
	}
	return out
}

func brief(s exportfmt.Skill) string {
	var paths []string
	for _, f := range s.Files {
		paths = append(paths, f.Path)
	}
	return fmt.Sprintf("name=%q description=%q license=%q compat=%q tools=%q metadata=%v extra=%v hash=%q files=%v body=%q",
		s.Name, s.Description, s.License, s.Compatibility, s.AllowedTools, s.Metadata, s.Extra, s.Hash, paths, s.Body)
}

func sameFiles(a, b []exportfmt.File) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Path != b[i].Path || !bytes.Equal(a[i].Data, b[i].Data) || a[i].Mode != b[i].Mode {
			return false
		}
	}
	return true
}

// piece is one thing the skill has, and how to find it in an output.
type piece struct {
	item   string // the loss Item that names it
	tokens []string
	data   []byte // a file's content (carried when some output file holds it)
	hash   string
	skill  []byte
}

func (p piece) matchesLoss(item string) bool {
	if item == p.item {
		return true
	}
	return strings.HasSuffix(item, "/") && strings.HasPrefix(p.item, item)
}

func (p piece) carried(files []exportfmt.File) bool {
	if p.hash != "" {
		// The hash is carried when an output file is the SKILL.md it was taken over.
		for _, f := range files {
			if bytes.Equal(f.Data, p.skill) {
				return true
			}
		}
		return false
	}
	for _, tok := range p.tokens {
		found := false
		for _, f := range files {
			if strings.Contains(f.Path, tok) || bytes.Contains(f.Data, []byte(tok)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if p.data != nil {
		for _, f := range files {
			if bytes.Contains(f.Data, p.data) {
				return true
			}
		}
		return false
	}
	return true
}

func piecesOf(s exportfmt.Skill) []piece {
	ps := []piece{
		{item: "name", tokens: []string{s.Name}},
		{item: "description", tokens: []string{strings.TrimSpace(s.Description)}},
	}
	if body := strings.TrimSpace(s.Body); body != "" {
		ps = append(ps, piece{item: "body", tokens: []string{body}})
	}
	if s.License != "" {
		ps = append(ps, piece{item: "license", tokens: []string{s.License}})
	}
	if s.Compatibility != "" {
		ps = append(ps, piece{item: "compatibility", tokens: []string{s.Compatibility}})
	}
	if s.AllowedTools != "" {
		ps = append(ps, piece{item: "allowed-tools", tokens: []string{s.AllowedTools}})
	}
	if len(s.Metadata) > 0 {
		var toks []string
		for _, k := range s.MetadataKeys() {
			toks = append(toks, k, s.Metadata[k])
		}
		ps = append(ps, piece{item: "metadata", tokens: toks})
	}
	for _, k := range s.ExtraKeys() {
		toks := []string{k}
		if str, ok := s.Extra[k].(string); ok {
			toks = append(toks, str)
		}
		ps = append(ps, piece{item: k, tokens: toks})
	}
	for _, f := range s.Files {
		ps = append(ps, piece{item: f.Path, data: f.Data})
	}
	if s.HasProjectionHeader() {
		ps = append(ps, piece{item: "projection-header", tokens: []string{"saphan-decision-projection-v1", "Zq-header-token"}})
	}
	if s.Hash != "" {
		ps = append(ps, piece{item: "canonical-hash", hash: s.Hash, skill: s.SkillMD})
	}
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].item < ps[j].item })
	return ps
}

// withoutLost is the skill with each named loss cleared: what a round trip may
// return.
func withoutLost(s exportfmt.Skill, losses []exportfmt.Loss) exportfmt.Skill {
	w := s
	w.Metadata = cloneMap(s.Metadata)
	w.Extra = map[string]any{}
	for k, v := range s.Extra {
		w.Extra[k] = v
	}
	if len(w.Extra) == 0 {
		w.Extra = nil
	}
	w.Files = append([]exportfmt.File(nil), s.Files...)
	for _, l := range losses {
		switch l.Item {
		case "license":
			w.License = ""
		case "compatibility":
			w.Compatibility = ""
		case "allowed-tools":
			w.AllowedTools = ""
		case "metadata":
			w.Metadata = nil
		case "canonical-hash":
			w.Hash = ""
		case "body":
			w.Body = ""
		default:
			delete(w.Extra, l.Item)
			if len(w.Extra) == 0 {
				w.Extra = nil
			}
			var keep []exportfmt.File
			for _, f := range w.Files {
				if f.Path != l.Item && !(strings.HasSuffix(l.Item, "/") && strings.HasPrefix(f.Path, l.Item)) {
					keep = append(keep, f)
				}
			}
			w.Files = keep
		}
	}
	return w
}

func cloneMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
