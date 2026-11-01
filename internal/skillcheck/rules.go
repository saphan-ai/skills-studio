package skillcheck

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Rule ids.
const (
	RuleHiddenText         = "SC-HIDDEN-TEXT"
	RuleOverridePhrase     = "SC-OVERRIDE-PHRASE"
	RuleURL                = "SC-URL"
	RuleURLMovingRef       = "SC-URL-MOVING-REF"
	RuleURLFetch           = "SC-URL-FETCH"
	RuleExecutable         = "SC-EXECUTABLE"
	RuleCredential         = "SC-CREDENTIAL"
	RuleAllowedTools       = "SC-ALLOWED-TOOLS"
	RuleDescriptionOverrun = "SC-DESCRIPTION-OVERCLAIM"
)

// Rule is one static check. Check returns the findings of this rule only.
type Rule struct {
	ID       string
	Severity Severity
	// Summary is the one sentence the spec's rule table carries.
	Summary string
	Check   func(s *skill, cfg Config) []Finding
}

// Rules is the registry, in the order the spec lists them.
var Rules = []Rule{
	{RuleHiddenText, Refuse, "a zero-width character, a Unicode tag character (U+E0000 to U+E007F) or a bidirectional control in any text file", hiddenText},
	{RuleOverridePhrase, Flag, "a phrase from the published list that tells an agent to drop its instructions", overridePhrase},
	{RuleURL, Flag, "a URL that is neither a moving reference nor read by a fetch instruction", func(s *skill, _ Config) []Finding { return urlFindings(s, urlPlain) }},
	{RuleURLMovingRef, Flag, "a URL with a path segment main, master, HEAD or latest, so the bytes behind it change without the skill changing", func(s *skill, _ Config) []Finding { return urlFindings(s, urlMoving) }},
	{RuleURLFetch, Refuse, "a URL on or next to a line with fetch, WebFetch, curl or wget, so behaviour is read at run time from outside the hashed bytes", func(s *skill, _ Config) []Finding { return urlFindings(s, urlFetch) }},
	{RuleExecutable, Refuse, "a scripts/ directory, an executable file, a shebang, a compiled executable or a script file extension anywhere in the skill", executable},
	{RuleCredential, Flag, "a credential path (~/.ssh, ~/.aws, .netrc, id_rsa, *.pem) or an environment variable named like a token or key", credential},
	{RuleAllowedTools, Flag, "an allowed-tools front matter key, which pre-approves the tools it names", allowedTools},
	{RuleDescriptionOverrun, Flag, "a description that asks to be used for all, any or every task, or lists more alternatives in one sentence than the configured limit", descriptionOverclaim},
}

func finding(rule string, sev Severity, path string, line int, format string, a ...any) Finding {
	return Finding{Rule: rule, Severity: sev, Path: path, Line: line, Message: fmt.Sprintf(format, a...)}
}

// ---- hidden text ----

// hiddenClass names the class of an invisible or direction-changing code
// point, or "" for any other.
func hiddenClass(r rune) string {
	switch {
	case r == 0x200B || r == 0x200C || r == 0x200D || r == 0x2060 || (r >= 0x2061 && r <= 0x2064) || r == 0xFEFF || r == 0x180E:
		return "zero-width"
	case r >= 0xE0000 && r <= 0xE007F:
		return "Unicode tag"
	case r == 0x061C || r == 0x200E || r == 0x200F || (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069):
		return "bidi control"
	}
	return ""
}

func hiddenText(s *skill, _ Config) []Finding {
	var out []Finding
	for _, f := range s.files {
		if !f.text {
			continue
		}
		for i, l := range lines(f) {
			var seen []string
			have := map[rune]bool{}
			for _, r := range l {
				if c := hiddenClass(r); c != "" && !have[r] {
					have[r] = true
					seen = append(seen, fmt.Sprintf("U+%04X (%s)", r, c))
				}
			}
			if len(seen) > 0 {
				out = append(out, finding(RuleHiddenText, Refuse, f.path, i+1,
					"the line holds characters a reader cannot see, %s; the text an agent reads is not the text a person reviews", strings.Join(seen, ", ")))
			}
		}
	}
	return out
}

// ---- override phrases ----

// OverridePhrases is the published list (spec, "Static checks"). A phrase is
// matched in lower case, after whitespace runs become one space and invisible
// characters are dropped, so a line break inside a phrase does not hide it.
var OverridePhrases = []string{
	"ignore previous instructions",
	"ignore all previous instructions",
	"ignore prior instructions",
	"ignore all prior instructions",
	"ignore the above",
	"ignore your instructions",
	"ignore the system prompt",
	"disregard previous instructions",
	"disregard all previous instructions",
	"disregard your instructions",
	"forget previous instructions",
	"forget your instructions",
	"override your instructions",
	"override the system prompt",
	"reveal your system prompt",
	"do not tell the user",
	"don't tell the user",
	"without telling the user",
}

// normalize lower-cases text, collapses whitespace, drops invisible characters
// and straightens apostrophes. lineOf[i] is the 1-based source line of byte i
// of the result.
func normalize(text string) (string, []int) {
	var b strings.Builder
	var lineOf []int
	line := 1
	space := false
	emit := func(r rune) {
		n := len(string(r))
		b.WriteRune(r)
		for k := 0; k < n; k++ {
			lineOf = append(lineOf, line)
		}
	}
	for _, r := range text {
		switch {
		case r == '\n':
			if !space {
				emit(' ')
				space = true
			}
			line++
			continue
		case unicode.IsSpace(r):
			if !space {
				emit(' ')
				space = true
			}
			continue
		case hiddenClass(r) != "":
			continue
		}
		space = false
		switch r {
		case '’', '‘':
			r = '\''
		}
		emit(unicode.ToLower(r))
	}
	return b.String(), lineOf
}

func overridePhrase(s *skill, _ Config) []Finding {
	var out []Finding
	for _, f := range s.files {
		if !f.text {
			continue
		}
		norm, lineOf := normalize(string(f.data))
		type key struct {
			line   int
			phrase string
		}
		seen := map[key]bool{}
		for _, p := range OverridePhrases {
			for from := 0; ; {
				i := strings.Index(norm[from:], p)
				if i < 0 {
					break
				}
				at := from + i
				k := key{lineOf[at], p}
				if !seen[k] {
					seen[k] = true
					out = append(out, finding(RuleOverridePhrase, Flag, f.path, k.line,
						"the text says %q, which tells an agent to drop the instructions it was given; read the sentence and decide whether the skill means it", p))
				}
				from = at + len(p)
			}
		}
	}
	return out
}

// ---- URLs ----

type urlClass int

const (
	urlPlain urlClass = iota
	urlMoving
	urlFetch
)

var (
	urlPattern   = regexp.MustCompile("https?://[^\\s<>\"'`)\\]]+")
	fetchPattern = regexp.MustCompile(`(?i)\b(webfetch|fetch(es|ed|ing)?|curl|wget)\b`)
	movingRefs   = map[string]bool{"main": true, "master": true, "head": true, "latest": true}
)

// FetchWords is the published list of run-time fetch instructions.
var FetchWords = []string{"fetch", "WebFetch", "curl", "wget"}

// MovingRefs is the published list of moving reference path segments.
var MovingRefs = []string{"main", "master", "HEAD", "latest"}

func movingRef(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	for _, seg := range strings.Split(u.Path, "/") {
		seg = strings.ToLower(seg)
		if movingRefs[seg] {
			return true
		}
		if i := strings.LastIndex(seg, "@"); i >= 0 && movingRefs[seg[i+1:]] {
			return true
		}
	}
	return false
}

// urlFindings classifies every URL once, by the strongest class that applies:
// a fetch instruction, then a moving reference, then a plain URL. Each URL
// therefore yields exactly one finding, under exactly one rule.
func urlFindings(s *skill, want urlClass) []Finding {
	var out []Finding
	for _, f := range s.files {
		if !f.text {
			continue
		}
		ls := lines(f)
		for i, l := range ls {
			for _, raw := range urlPattern.FindAllString(l, -1) {
				raw = strings.TrimRight(raw, ".,;:!?*_")
				class := urlPlain
				fetchNear := false
				for j := i - 1; j <= i+1; j++ {
					if j >= 0 && j < len(ls) && fetchPattern.MatchString(ls[j]) {
						fetchNear = true
					}
				}
				switch {
				case fetchNear:
					class = urlFetch
				case movingRef(raw):
					class = urlMoving
				}
				if class != want {
					continue
				}
				switch class {
				case urlFetch:
					out = append(out, finding(RuleURLFetch, Refuse, f.path, i+1,
						"the text sits next to a fetch instruction for %s, so behaviour is read at run time from outside the hashed bytes; put what is needed in the skill or admit a tool", raw))
				case urlMoving:
					out = append(out, finding(RuleURLMovingRef, Flag, f.path, i+1,
						"%s names a moving reference, so the bytes behind it change without the skill changing; pin a commit or version", raw))
				default:
					out = append(out, finding(RuleURL, Flag, f.path, i+1,
						"the text cites %s; this check does not read it and cannot say what it holds", raw))
				}
			}
		}
	}
	return out
}

// ---- executables ----

var scriptExtensions = map[string]bool{
	".sh": true, ".bash": true, ".zsh": true, ".fish": true, ".py": true, ".rb": true, ".pl": true,
	".js": true, ".mjs": true, ".ts": true, ".ps1": true, ".bat": true, ".cmd": true, ".php": true, ".lua": true,
}

// ScriptExtensions is the published list, sorted.
func ScriptExtensions() []string {
	var out []string
	for e := range scriptExtensions {
		out = append(out, e)
	}
	sort.Strings(out)
	return out
}

var machineMagic = []struct {
	prefix string
	what   string
}{
	{"\x7fELF", "an ELF executable"},
	{"\xfe\xed\xfa\xce", "a Mach-O executable"},
	{"\xfe\xed\xfa\xcf", "a Mach-O executable"},
	{"\xce\xfa\xed\xfe", "a Mach-O executable"},
	{"\xcf\xfa\xed\xfe", "a Mach-O executable"},
	{"\xca\xfe\xba\xbe", "a Mach-O universal or Java class file"},
	{"MZ", "a Windows executable"},
}

const toolNext = "a script never enters a skill: admit it as a tool through `saphan tool admit` and cite the tool by id"

func executable(s *skill, _ Config) []Finding {
	var out []Finding
	for _, d := range s.dirs {
		if d == "scripts" {
			out = append(out, finding(RuleExecutable, Refuse, d, 1, "the skill has a scripts/ directory; %s", toolNext))
		}
	}
	for _, f := range s.files {
		if strings.HasPrefix(f.path, "scripts/") {
			continue // covered by the directory's finding
		}
		var why string
		switch {
		case f.mode&0o111 != 0:
			why = "has an executable permission bit"
		case strings.HasPrefix(string(f.data), "#!"):
			why = "starts with a shebang"
		case scriptExtensions[strings.ToLower(path.Ext(f.path))]:
			why = "has the script extension " + strings.ToLower(path.Ext(f.path))
		default:
			for _, m := range machineMagic {
				if strings.HasPrefix(string(f.data), m.prefix) {
					why = "is " + m.what
					break
				}
			}
		}
		if why != "" {
			out = append(out, finding(RuleExecutable, Refuse, f.path, 1, "the file %s; %s", why, toolNext))
		}
	}
	return out
}

// ---- credentials ----

var (
	credPathPattern = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])\.(?:ssh|aws)(?:/|[^A-Za-z0-9_]|$)|\.netrc\b|\bid_(?:rsa|dsa|ecdsa|ed25519)\b|[\w./~-]*\.pem\b`)
	envNamePattern  = regexp.MustCompile(`[A-Z][A-Z0-9_]{2,}`)
	secretWords     = []string{"TOKEN", "SECRET", "PASSWORD", "PASSWD", "APIKEY", "API_KEY", "ACCESS_KEY", "PRIVATE_KEY", "CREDENTIAL"}
)

func secretName(name string) bool {
	for _, w := range secretWords {
		if strings.Contains(name, w) {
			return true
		}
	}
	return false
}

func credential(s *skill, _ Config) []Finding {
	var out []Finding
	for _, f := range s.files {
		if !f.text {
			continue
		}
		for i, l := range lines(f) {
			var hits []string
			for _, m := range credPathPattern.FindAllString(l, -1) {
				hits = append(hits, strings.TrimRight(strings.TrimLeft(m, "/ \t\"'`("), "/ \t\"'`(),.;:"))
			}
			for _, loc := range envNamePattern.FindAllStringIndex(l, -1) {
				name := l[loc[0]:loc[1]]
				if !secretName(name) {
					continue
				}
				dollar := loc[0] > 0 && (l[loc[0]-1] == '$' || (loc[0] > 1 && l[loc[0]-1] == '{' && l[loc[0]-2] == '$'))
				if strings.Contains(name, "_") || dollar {
					hits = append(hits, name)
				}
			}
			if len(hits) > 0 {
				out = append(out, finding(RuleCredential, Flag, f.path, i+1,
					"the line names %s, which reaches for a credential; check the skill needs it and says why", strings.Join(dedupe(hits), ", ")))
			}
		}
	}
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// ---- allowed-tools ----

// splitTools splits the allowed-tools value on whitespace outside parentheses.
func splitTools(v string) []string {
	var out []string
	var cur strings.Builder
	depth := 0
	flush := func() {
		if t := strings.Trim(cur.String(), ","); t != "" {
			out = append(out, t)
		}
		cur.Reset()
	}
	for _, r := range v {
		switch {
		case r == '(':
			depth++
		case r == ')' && depth > 0:
			depth--
		case unicode.IsSpace(r) && depth == 0:
			flush()
			continue
		}
		cur.WriteRune(r)
	}
	flush()
	return out
}

func allowedTools(s *skill, _ Config) []Finding {
	v, ok := s.fm["allowed-tools"].(string)
	if !ok {
		return nil
	}
	tools := splitTools(v)
	if len(tools) == 0 {
		return nil
	}
	return []Finding{finding(RuleAllowedTools, Flag, "SKILL.md", s.fmLine("allowed-tools"),
		"allowed-tools pre-approves %s, which run without asking the user; confirm the skill needs each", strings.Join(tools, ", "))}
}

// ---- description over-claim ----

var (
	overclaimPattern = regexp.MustCompile(`(?i)\b(?:all|any|every)\s+(?:[a-z-]+\s+){0,2}(?:tasks?|requests?|prompts?|jobs?|queries|questions|situations?|projects?|languages?)\b|\b(?:everything|anything)\b`)
	sentenceEnd      = regexp.MustCompile(`[.!?](?:\s+|$)`)
	listLead         = regexp.MustCompile(`(?i)^(?:or|and)\s+`)
)

// longestList counts the items of the longest comma- or semicolon-separated
// list in any one sentence of text.
func longestList(text string) int {
	best := 0
	for _, sent := range sentenceEnd.Split(text, -1) {
		n := 0
		for _, item := range strings.FieldsFunc(sent, func(r rune) bool { return r == ',' || r == ';' }) {
			if strings.TrimSpace(listLead.ReplaceAllString(strings.TrimSpace(item), "")) != "" {
				n++
			}
		}
		if n > best {
			best = n
		}
	}
	return best
}

func descriptionOverclaim(s *skill, cfg Config) []Finding {
	d, ok := s.fm["description"].(string)
	if !ok {
		return nil
	}
	line := s.fmLine("description")
	var out []Finding
	if m := overclaimPattern.FindString(d); m != "" {
		out = append(out, finding(RuleDescriptionOverrun, Flag, "SKILL.md", line,
			"the description says %q, which asks to be chosen for work outside any one subject; name the cases it is for", m))
	}
	if n := longestList(d); n > cfg.MaxDomains {
		out = append(out, finding(RuleDescriptionOverrun, Flag, "SKILL.md", line,
			"the description lists %d alternatives in one sentence, more than the limit of %d; split the skill or name fewer cases", n, cfg.MaxDomains))
	}
	return out
}
