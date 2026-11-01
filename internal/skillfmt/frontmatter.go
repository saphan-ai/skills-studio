package skillfmt

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	keyPattern    = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	numberPattern = regexp.MustCompile(`^[-+]?([0-9]+\.?[0-9]*|\.[0-9]+)([eE][-+]?[0-9]+)?$`)
	// yamlOneOneBool are plain words a YAML 1.1 reader turns into booleans. The
	// format refuses them unquoted, so every reader sees the same string.
	yamlOneOneBool = map[string]bool{"y": true, "n": true, "yes": true, "no": true, "on": true, "off": true}
)

// Plain scalars a YAML 1.1 or 1.2-core reader resolves to something other than
// the string (or decimal number) this reader returns. Each is refused.
var divergentPatterns = []*regexp.Regexp{
	// 1.2 core and 1.1 integers in other bases; 1.1 octal (leading zero).
	regexp.MustCompile(`^[-+]?0(x[0-9a-fA-F_]+|o[0-7_]+|b[01_]+|[0-7_]+)$`),
	// 1.2 core and 1.1 special floats, any letter case.
	regexp.MustCompile(`^(?i)[-+]?\.(inf|nan)$`),
	// 1.1 timestamps: a date, or a date and time.
	regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`),
	regexp.MustCompile(`^[0-9]{4}-[0-9]{1,2}-[0-9]{1,2}([Tt]|[ \t]+)[0-9]{1,2}:[0-9]{2}:[0-9]{2}(\.[0-9]*)?([ \t]*(Z|[-+][0-9]{1,2}(:[0-9]{2})?))?$`),
	// 1.1 numbers with underscores.
	regexp.MustCompile(`^[-+]?[0-9][0-9_]*(\.[0-9_]*)?([eE][-+]?[0-9_]+)?$`),
	// 1.1 sexagesimal integers and floats.
	regexp.MustCompile(`^[-+]?[0-9][0-9_]*(:[0-5]?[0-9])+(\.[0-9_]*)?$`),
}

func divergentPlain(s string) bool {
	for i, p := range divergentPatterns {
		if !p.MatchString(s) {
			continue
		}
		if i == 4 && !strings.Contains(s, "_") { // a plain decimal number is read as a number
			continue
		}
		return true
	}
	return false
}

// nonPrintable reports a code point a YAML reader refuses or reads as a line
// break: C0 other than TAB, DEL, C1 (U+0085 included), U+2028 and U+2029.
func nonPrintable(r rune) bool {
	return (r < 0x20 && r != '\t') || r == 0x7f || (r >= 0x80 && r <= 0x9f) || r == 0x2028 || r == 0x2029
}

func syntax(line int, format string, a ...any) *Violation {
	return &Violation{
		Rule:    RuleFrontMatterSyntax,
		Path:    "line " + strconv.Itoa(line),
		Message: fmt.Sprintf(format, a...),
	}
}

// ParseFrontMatter reads the front matter block at the head of text (SKILL.md
// with any header already removed) into the JSON data model: map[string]any,
// with string, float64, bool, nil and map[string]any values.
//
// It reads a subset of YAML: top-level "key: value" lines, one nested block of
// "  key: value" lines under a key with an empty value, scalars that are plain,
// double-quoted (JSON escapes) or single-quoted. Anything else is refused.
func ParseFrontMatter(text []byte) (map[string]any, *Violation) {
	lines := strings.Split(string(text), "\n")
	if lines[0] != "---" {
		return nil, &Violation{Rule: RuleFrontMatterMissing, Message: "the first line of SKILL.md (after any header) is not ---"}
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, &Violation{Rule: RuleFrontMatterMissing, Message: "the front matter is not closed by a line ---"}
	}

	doc := map[string]any{}
	var block map[string]any
	for i := 1; i < end; i++ {
		n := i + 1 // 1-based line number within the text
		line := lines[i]
		for _, r := range line {
			if nonPrintable(r) {
				return nil, syntax(n, "the non-printable code point U+%04X; YAML readers refuse or reinterpret it", r)
			}
		}
		switch {
		case strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.Contains(line, "\t"):
			return nil, syntax(n, "a tab character; indent with spaces")
		case strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   "):
			if block == nil {
				return nil, syntax(n, "an indented line that does not follow a key with an empty value")
			}
			k, v, viol := splitKeyValue(line[2:], n)
			if viol != nil {
				return nil, viol
			}
			if _, dup := block[k]; dup {
				return nil, syntax(n, "the key %q appears twice", k)
			}
			if v == "" {
				block[k] = nil
				continue
			}
			val, viol := parseScalar(v, n)
			if viol != nil {
				return nil, viol
			}
			block[k] = val
		case strings.HasPrefix(line, " "):
			return nil, syntax(n, "indentation other than two spaces under one key")
		default:
			k, v, viol := splitKeyValue(line, n)
			if viol != nil {
				return nil, viol
			}
			if _, dup := doc[k]; dup {
				return nil, syntax(n, "the key %q appears twice", k)
			}
			block = nil
			if v == "" {
				if nextIsIndented(lines, i, end) {
					block = map[string]any{}
					doc[k] = block
				} else {
					doc[k] = nil
				}
				continue
			}
			val, viol := parseScalar(v, n)
			if viol != nil {
				return nil, viol
			}
			doc[k] = val
		}
	}
	return doc, nil
}

func nextIsIndented(lines []string, i, end int) bool {
	for j := i + 1; j < end; j++ {
		if strings.TrimSpace(lines[j]) == "" {
			continue
		}
		return strings.HasPrefix(lines[j], "  ")
	}
	return false
}

// splitKeyValue splits "key: value" (or "key:") at the first colon.
func splitKeyValue(line string, n int) (key, value string, v *Violation) {
	k, rest, ok := strings.Cut(line, ":")
	if !ok || !keyPattern.MatchString(k) {
		return "", "", syntax(n, "expected \"key: value\" with a key of letters, digits, dot, underscore or hyphen")
	}
	if rest != "" && !strings.HasPrefix(rest, " ") {
		return "", "", syntax(n, "the colon after %q must be followed by a space", k)
	}
	return k, strings.TrimSpace(rest), nil
}

func parseScalar(s string, n int) (any, *Violation) {
	switch s[0] {
	case '"':
		var out string
		if err := json.Unmarshal([]byte(s), &out); err != nil {
			return nil, syntax(n, "a double-quoted string that is not a valid JSON string")
		}
		return out, nil
	case '\'':
		if len(s) < 2 || s[len(s)-1] != '\'' {
			return nil, syntax(n, "a single-quoted string that is not closed")
		}
		inner := s[1 : len(s)-1]
		if strings.Contains(strings.ReplaceAll(inner, "''", ""), "'") {
			return nil, syntax(n, "a single quote inside a single-quoted string must be doubled")
		}
		return strings.ReplaceAll(inner, "''", "'"), nil
	case '[', ']', '{', '}', ',', '&', '*', '!', '|', '>', '%', '@', '`', '-', '?', '#':
		return nil, syntax(n, "a value starting with %q: lists, maps, anchors, tags, comments and block scalars are not in the subset; quote the text", string(s[0]))
	}
	if divergentPlain(s) {
		return nil, syntax(n, "the plain value %q is a number, date or special float in some YAML versions; write it in quotes", s)
	}
	if strings.Contains(s, ": ") || strings.HasSuffix(s, ":") || strings.Contains(s, " #") {
		return nil, syntax(n, "a plain value containing a colon or a comment marker; quote the text")
	}
	lower := strings.ToLower(s)
	switch {
	case yamlOneOneBool[lower]:
		return nil, syntax(n, "the plain word %q reads as a boolean in some YAML versions; write it in quotes", s)
	case s == "true":
		return true, nil
	case s == "false":
		return false, nil
	case s == "null" || s == "~":
		return nil, nil
	case lower == "true" || lower == "false" || lower == "null":
		// Only the exact lowercase words are bool or null; a YAML reader differs on
		// the other letter cases, so they are refused rather than guessed.
		return nil, syntax(n, "the plain word %q is a boolean or null in some YAML versions and a string in others; write it in quotes", s)
	case numberPattern.MatchString(s):
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, nil
		}
	}
	return s, nil
}
