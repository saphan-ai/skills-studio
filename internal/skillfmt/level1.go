package skillfmt

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Level 1 is "valid Agent Skills": the rules of the Agent Skills specification
// page, read on 2026-11-02 (https://agentskills.io/specification), and nothing
// else. The front matter is read by a full YAML reader (gopkg.in/yaml.v3). The
// rules are in spec/SKILL_FORMAT_V1.md, section 13.
//
// The name charset is the one place the page contradicts itself: it says
// "unicode lowercase alphanumeric characters (`a-z`, `0-9`)". Level 1 reads the
// words (unicode lowercase letters and digits); level 2 reads the ranges (ASCII).

// AgentSkillsPageRead is the date the Agent Skills specification page was read
// to write level 1. The page carries no version or date of its own.
const AgentSkillsPageRead = "2026-11-02"

// Level 1 rule identifiers. They share nothing with the SF- rules of the
// profile: an AS- rule names a sentence of the Agent Skills page, an SF- rule
// names a rule of this format.
const (
	RuleASUnread             = "AS-UNREAD"
	RuleASSkillMDMissing     = "AS-SKILLMD-MISSING"
	RuleASFrontMatterMissing = "AS-FRONTMATTER-MISSING"
	RuleASFrontMatterYAML    = "AS-FRONTMATTER-YAML"
	RuleASNameRequired       = "AS-NAME-REQUIRED"
	RuleASNameLength         = "AS-NAME-LENGTH"
	RuleASNameChars          = "AS-NAME-CHARS"
	RuleASNameHyphen         = "AS-NAME-HYPHEN"
	RuleASNameDirMismatch    = "AS-NAME-DIR-MISMATCH"
	RuleASDescRequired       = "AS-DESCRIPTION-REQUIRED"
	RuleASDescLength         = "AS-DESCRIPTION-LENGTH"
	RuleASCompatLength       = "AS-COMPATIBILITY-LENGTH"
	RuleASMetadataType       = "AS-METADATA-TYPE"
	RuleASMetadataValueType  = "AS-METADATA-VALUE-TYPE"
	RuleASFieldType          = "AS-FIELD-TYPE"
)

// Level1Rules is the closed list of level 1 rules. A test holds it to the
// specification text.
var Level1Rules = []Rule{
	{RuleASUnread, "SKILL.md or the skill directory was not read: a symbolic link, or a front matter longer than the reader's bound", false},
	{RuleASSkillMDMissing, "the directory has no SKILL.md (a regular file)", false},
	{RuleASFrontMatterMissing, "SKILL.md does not open with a front matter block closed by ---", false},
	{RuleASFrontMatterYAML, "the front matter is not YAML, or is not a mapping, or repeats a key", false},
	{RuleASNameRequired, "name is absent", false},
	{RuleASNameLength, "name is not 1 to 64 characters", false},
	{RuleASNameChars, "name has a character other than a unicode lowercase letter, a digit or a hyphen", false},
	{RuleASNameHyphen, "name starts or ends with a hyphen, or holds two hyphens in a row", false},
	{RuleASNameDirMismatch, "name differs from the parent directory name", false},
	{RuleASDescRequired, "description is absent", false},
	{RuleASDescLength, "description is not 1 to 1024 characters", false},
	{RuleASCompatLength, "compatibility is not 1 to 500 characters", false},
	{RuleASMetadataType, "metadata is not a map", false},
	{RuleASMetadataValueType, "a metadata key or value is not a string (a key YAML reads as an integer, a boolean, null or a float is refused)", false},
	{RuleASFieldType, "name, description, compatibility or allowed-tools is not a string", false},
}

// Reader bounds. They are limits of this program, not rules of the Agent
// Skills page; a file over one of them is reported as AS-UNREAD, never as read.
const (
	// MaxLevel1PrefixBytes is how much of the head of SKILL.md level 1 reads:
	// the optional header and the whole front matter must lie inside it. The
	// body is never read.
	MaxLevel1PrefixBytes = MaxSkillMDBytes
	maxYAMLNodes         = 10000
	maxYAMLDepth         = 64
)

// level1Read is what the level 1 reader obtained from one skill directory.
type level1Read struct {
	// Doc is the front matter in the JSON data model; nil when it was not read.
	Doc map[string]any
	// Violations lists every level 1 refusal.
	Violations []Violation
}

// readLevel1 reads SKILL.md in abs against the Agent Skills page. abs is an
// absolute path of a directory that is not a symbolic link.
func readLevel1(abs string) level1Read {
	path := filepath.Join(abs, "SKILL.md")
	info, err := os.Lstat(path)
	switch {
	case os.IsNotExist(err):
		return level1Read{Violations: []Violation{{Rule: RuleASSkillMDMissing, Path: "SKILL.md", Message: "the directory has no SKILL.md"}}}
	case err != nil:
		return level1Read{Violations: []Violation{{Rule: RuleASUnread, Path: "SKILL.md", Message: err.Error()}}}
	case info.Mode()&os.ModeSymlink != 0:
		return level1Read{Violations: []Violation{{Rule: RuleASUnread, Path: "SKILL.md", Message: "a symbolic link; this verifier does not read through links"}}}
	case !info.Mode().IsRegular():
		return level1Read{Violations: []Violation{{Rule: RuleASSkillMDMissing, Path: "SKILL.md", Message: "SKILL.md is not a regular file"}}}
	}
	f, err := os.Open(path)
	if err != nil {
		return level1Read{Violations: []Violation{{Rule: RuleASUnread, Path: "SKILL.md", Message: err.Error()}}}
	}
	defer f.Close()
	head, err := io.ReadAll(io.LimitReader(f, MaxLevel1PrefixBytes+1))
	if err != nil {
		return level1Read{Violations: []Violation{{Rule: RuleASUnread, Path: "SKILL.md", Message: err.Error()}}}
	}
	truncated := len(head) > MaxLevel1PrefixBytes
	if truncated {
		head = head[:MaxLevel1PrefixBytes]
	}
	return readLevel1Text(head, truncated, filepath.Base(abs))
}

// normalizeText removes a byte order mark and turns CR LF and lone CR into LF,
// so that level 1 reads what a YAML reader reads from the same file.
func normalizeText(b []byte) []byte {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	if bytes.IndexByte(b, '\r') < 0 {
		return b
	}
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(b, []byte("\r"), []byte("\n"))
}

func asViolation(rule, path, format string, a ...any) []Violation {
	return []Violation{{Rule: rule, Path: path, Message: fmt.Sprintf(format, a...)}}
}

func readLevel1Text(head []byte, truncated bool, dirName string) level1Read {
	text, v := CanonicalText(normalizeText(head))
	if v != nil {
		// The projection header is part of the Saphan format; the page defines none.
		// Without a closed header the front matter cannot be located.
		return level1Read{Violations: asViolation(RuleASFrontMatterMissing, "SKILL.md", "the file opens with a projection header that is not well formed: %s", v.Message)}
	}
	fm, state := frontMatterBlock(text)
	switch state {
	case blockMissing:
		return level1Read{Violations: asViolation(RuleASFrontMatterMissing, "SKILL.md", "the first line of SKILL.md (after any header) is not ---")}
	case blockUnclosed:
		if truncated {
			return level1Read{Violations: asViolation(RuleASUnread, "SKILL.md", "no closing --- within the first %d bytes, the most of the file this verifier reads", MaxLevel1PrefixBytes)}
		}
		return level1Read{Violations: asViolation(RuleASFrontMatterMissing, "SKILL.md", "the front matter is not closed by a line ---")}
	}
	if !utf8.Valid(fm) {
		return level1Read{Violations: asViolation(RuleASFrontMatterYAML, "SKILL.md", "the front matter is not valid UTF-8")}
	}
	doc, badKeys, vs := readYAMLMapping(fm)
	if vs != nil {
		return level1Read{Violations: vs}
	}
	return level1Read{Doc: doc, Violations: checkLevel1Doc(doc, badKeys, dirName)}
}

type blockState int

const (
	blockOK blockState = iota
	blockMissing
	blockUnclosed
)

// frontMatterBlock returns the lines between the opening and the closing ---.
// A line of --- with trailing spaces or tabs counts as ---.
func frontMatterBlock(text []byte) ([]byte, blockState) {
	isFence := func(line []byte) bool { return string(bytes.TrimRight(line, " \t")) == "---" }
	first, rest, _ := bytes.Cut(text, []byte("\n"))
	if !isFence(first) {
		return nil, blockMissing
	}
	off := 0
	for off <= len(rest) {
		line, after, found := bytes.Cut(rest[off:], []byte("\n"))
		if isFence(line) {
			return rest[:off], blockOK
		}
		if !found {
			break
		}
		off = len(rest) - len(after)
	}
	return nil, blockUnclosed
}

// readYAMLMapping reads the front matter with a full YAML reader into the JSON
// data model. Plain scalars are read as YAML 1.2 reads them: only true and
// false are booleans, and a date is a string.
//
// The second result is the set of metadata keys the reader did not read as a
// string (an integer, a boolean, null, a float or a merge key): the page says
// metadata maps string to string, and the JSON data model has only string keys,
// so the tag is read here, from the node, before the key is flattened to text.
func readYAMLMapping(fm []byte) (map[string]any, map[string]bool, []Violation) {
	var node yaml.Node
	if err := yaml.Unmarshal(fm, &node); err != nil {
		return nil, nil, asViolation(RuleASFrontMatterYAML, "SKILL.md", "the front matter is not YAML: %s", oneLine(err.Error()))
	}
	if node.Kind == 0 { // nothing between the two --- lines
		return map[string]any{}, nil, nil
	}
	if node.Kind != yaml.DocumentNode || len(node.Content) != 1 {
		return nil, nil, asViolation(RuleASFrontMatterYAML, "SKILL.md", "the front matter is not one YAML document")
	}
	root := node.Content[0]
	for root.Kind == yaml.AliasNode {
		root = root.Alias
	}
	if root.Kind == yaml.ScalarNode && root.Tag == "!!null" {
		return map[string]any{}, nil, nil
	}
	if root.Kind != yaml.MappingNode {
		return nil, nil, asViolation(RuleASFrontMatterYAML, "SKILL.md", "the front matter is not a mapping of keys to values")
	}
	c := &converter{}
	v, err := c.convert(root, 0)
	if err != nil {
		return nil, nil, asViolation(RuleASFrontMatterYAML, "SKILL.md", "%s", err.Error())
	}
	return v.(map[string]any), nonStringMetadataKeys(root), nil
}

// nonStringMetadataKeys names the keys of the top-level metadata mapping whose
// YAML tag is not !!str.
func nonStringMetadataKeys(root *yaml.Node) map[string]bool {
	bad := map[string]bool{}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "metadata" {
			continue
		}
		m := root.Content[i+1]
		for m.Kind == yaml.AliasNode {
			m = m.Alias
		}
		if m.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(m.Content); j += 2 {
			k := m.Content[j]
			for k.Kind == yaml.AliasNode {
				k = k.Alias
			}
			if k.Kind == yaml.ScalarNode && k.Tag != "!!str" {
				bad[k.Value] = true
			}
		}
	}
	return bad
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

type converter struct{ nodes int }

func (c *converter) convert(n *yaml.Node, depth int) (any, error) {
	c.nodes++
	if c.nodes > maxYAMLNodes || depth > maxYAMLDepth {
		return nil, fmt.Errorf("the front matter holds more than %d nodes or nests deeper than %d levels", maxYAMLNodes, maxYAMLDepth)
	}
	switch n.Kind {
	case yaml.AliasNode:
		return c.convert(n.Alias, depth+1)
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for _, e := range n.Content {
			v, err := c.convert(e, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case yaml.MappingNode:
		out := make(map[string]any, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			for k.Kind == yaml.AliasNode {
				k = k.Alias
			}
			if k.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("line %d: a key that is not a scalar", k.Line)
			}
			if _, dup := out[k.Value]; dup {
				return nil, fmt.Errorf("line %d: the key %q appears twice", k.Line, k.Value)
			}
			v, err := c.convert(n.Content[i+1], depth+1)
			if err != nil {
				return nil, err
			}
			out[k.Value] = v
		}
		return out, nil
	case yaml.ScalarNode:
		switch n.Tag {
		case "!!null":
			return nil, nil
		case "!!str", "!!timestamp", "!!binary":
			return n.Value, nil
		case "!!bool", "!!int", "!!float":
			var x any
			if err := n.Decode(&x); err != nil {
				return n.Value, nil
			}
			switch t := x.(type) {
			case int:
				return float64(t), nil
			case int64:
				return float64(t), nil
			case uint64:
				return float64(t), nil
			}
			return x, nil
		}
		return n.Value, nil // an explicit tag this reader does not define: the text
	}
	return nil, fmt.Errorf("line %d: a YAML node this reader does not read", n.Line)
}

// checkLevel1Doc applies the rules of the Agent Skills page to the front
// matter. A key whose value is YAML null is read as absent.
func checkLevel1Doc(doc map[string]any, badMetaKeys map[string]bool, dirName string) []Violation {
	var out []Violation
	add := func(rule, path, format string, a ...any) {
		out = append(out, Violation{Rule: rule, Path: "SKILL.md#/" + path, Message: fmt.Sprintf(format, a...)})
	}
	str := func(key string) (string, bool, bool) { // value, present, isString
		v, ok := doc[key]
		if !ok || v == nil {
			return "", false, false
		}
		s, isStr := v.(string)
		return s, true, isStr
	}

	name, present, isStr := str("name")
	switch {
	case !present:
		add(RuleASNameRequired, "name", "name is absent")
	case !isStr:
		add(RuleASFieldType, "name", "name is not a string")
	default:
		if n := utf8.RuneCountInString(name); n < 1 || n > 64 {
			add(RuleASNameLength, "name", "name is %d characters; it must be 1 to 64", n)
		}
		for _, r := range name {
			if r != '-' && !unicode.IsDigit(r) && !(unicode.IsLetter(r) && unicode.IsLower(r)) {
				add(RuleASNameChars, "name", "name holds %q, which is not a lowercase letter, a digit or a hyphen", r)
				break
			}
		}
		if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "--") {
			add(RuleASNameHyphen, "name", "name %q starts or ends with a hyphen or holds two hyphens in a row", name)
		}
		if name != dirName {
			add(RuleASNameDirMismatch, "name", "name %q differs from the directory name %q", name, dirName)
		}
	}

	desc, present, isStr := str("description")
	switch {
	case !present:
		add(RuleASDescRequired, "description", "description is absent")
	case !isStr:
		add(RuleASFieldType, "description", "description is not a string")
	default:
		if n := utf8.RuneCountInString(desc); n < 1 || n > 1024 {
			add(RuleASDescLength, "description", "description is %d characters; it must be 1 to 1024", n)
		}
	}

	compat, present, isStr := str("compatibility")
	switch {
	case !present:
	case !isStr:
		add(RuleASFieldType, "compatibility", "compatibility is not a string")
	default:
		if n := utf8.RuneCountInString(compat); n < 1 || n > 500 {
			add(RuleASCompatLength, "compatibility", "compatibility is %d characters; it must be 1 to 500", n)
		}
	}

	if _, present, isStr := str("allowed-tools"); present && !isStr {
		add(RuleASFieldType, "allowed-tools", "allowed-tools is not a string")
	}

	if v, ok := doc["metadata"]; ok && v != nil {
		m, isMap := v.(map[string]any)
		if !isMap {
			add(RuleASMetadataType, "metadata", "metadata is not a map")
		} else {
			for _, k := range sortedKeys(m) {
				if badMetaKeys[k] {
					add(RuleASMetadataValueType, "metadata/"+k, "the metadata key %q is not a string", k)
				} else if _, isStr := m[k].(string); !isStr {
					add(RuleASMetadataValueType, "metadata/"+k, "the metadata value of %q is not a string", k)
				}
			}
		}
	}
	return out
}
