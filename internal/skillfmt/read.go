package skillfmt

import (
	"bytes"
	"unicode/utf8"
)

// SplitSkillMD reads the text of a SKILL.md as level 1 reads it: a byte order
// mark and CR LF are normalised, a projection header is set aside, and the
// front matter is read by the full YAML reader into the JSON data model. It
// returns the front matter and the Markdown after the closing --- line. It
// applies no rule of either level; callers that need a verdict call Verify.
// It exists so that a program built on this reader (an exporter) does not
// write a second front matter reader that could disagree with this one.
func SplitSkillMD(data []byte) (doc map[string]any, body string, vs []Violation) {
	text, v := CanonicalText(normalizeText(data))
	if v != nil {
		return nil, "", asViolation(RuleASFrontMatterMissing, "SKILL.md", "the file opens with a projection header that is not well formed: %s", v.Message)
	}
	fm, state := frontMatterBlock(text)
	switch state {
	case blockMissing:
		return nil, "", asViolation(RuleASFrontMatterMissing, "SKILL.md", "the first line of SKILL.md (after any header) is not ---")
	case blockUnclosed:
		return nil, "", asViolation(RuleASFrontMatterMissing, "SKILL.md", "the front matter is not closed by a line ---")
	}
	if !utf8.Valid(fm) {
		return nil, "", asViolation(RuleASFrontMatterYAML, "SKILL.md", "the front matter is not valid UTF-8")
	}
	doc, _, vs = readYAMLMapping(fm)
	if vs != nil {
		return nil, "", vs
	}
	// The body starts after: the opening line, the front matter lines, the closing line.
	off := bytes.IndexByte(text, '\n') + 1 + len(fm)
	if rest := text[off:]; bytes.IndexByte(rest, '\n') >= 0 {
		off += bytes.IndexByte(rest, '\n') + 1
	} else {
		off = len(text)
	}
	return doc, string(text[off:]), nil
}
