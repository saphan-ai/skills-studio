package skillcheck

import (
	"fmt"
	"strings"
)

// Fixed remarks: printed on every run, whatever the findings. Each is one line.
// A test asserts that every run carries each of them.
var FixedRemarks = []string{
	"a text scan cannot detect semantic hijacking: instructions that comply in wording and still steer an agent against its user; one outside study, cited at the owner's request and not re-checked here, measured 0 % detection by a text scanner alone",
	"behaviour that lives at a fetched URL is outside the files read: this check opens no URL, so it cannot say what any URL returns now or will return later",
	"the behaviour of a binary or non-text asset is not read; when the skill has one, a later remark lists it",
	"patterns are literal text: a phrase in another language, in an encoding, or built up from parts is not matched, and neither are full-width letters, HTML entities, look-alike letters, combining marks, a URL split over lines, a URL without a scheme, or the variation selectors U+FE00 to U+FE0F",
	"a clean result is evidence that these patterns are absent and nothing more: it is not a statement that the skill is harmless, and no model ran on it",
}

// remarks builds the full list for one run: the fixed lines, then what this
// run could not read.
func remarks(s *skill, rules []string) []string {
	out := append([]string{}, FixedRemarks...)
	var unread []string
	for _, f := range s.files {
		if !f.text {
			unread = append(unread, fmt.Sprintf("%s (%s)", f.path, f.why))
		}
	}
	unread = append(unread, s.special...)
	if len(unread) > 0 {
		out = append(out, fmt.Sprintf("%d file(s) were not scanned as text: %s", len(unread), listShort(unread)))
	}
	if len(s.symlinks) > 0 {
		out = append(out, fmt.Sprintf("%d symbolic link(s) were not followed and their targets were not read: %s", len(s.symlinks), listShort(s.symlinks)))
	}
	if s.fmErr != "" {
		out = append(out, fmt.Sprintf("the front matter could not be read (%s), so SC-ALLOWED-TOOLS and SC-DESCRIPTION-OVERCLAIM did not run", oneLine(s.fmErr)))
	}
	out = append(out, "rules run: "+strings.Join(rules, " "))
	return out
}

func listShort(items []string) string {
	const max = 5
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(items[:max], ", "), len(items)-max)
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
