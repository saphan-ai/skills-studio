package skillfmt

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"unicode/utf8"
)

// HeaderOpen is the line that opens a decision-projection header.
const HeaderOpen = "<!-- saphan-decision-projection-v1"

// headerClose is the line that closes it.
const headerClose = "-->"

// CheckText refuses SKILL.md bytes that are not UTF-8 text with LF line ends,
// no NUL and no byte order mark (rule SF-TEXT-ENCODING).
func CheckText(b []byte) *Violation {
	switch {
	case !utf8.Valid(b):
		return &Violation{Rule: RuleTextEncoding, Message: "SKILL.md is not valid UTF-8"}
	case bytes.HasPrefix(b, []byte("\xef\xbb\xbf")):
		return &Violation{Rule: RuleTextEncoding, Message: "SKILL.md starts with a byte order mark"}
	case bytes.IndexByte(b, '\r') >= 0:
		return &Violation{Rule: RuleTextEncoding, Message: "SKILL.md contains a carriage return; line ends are LF"}
	case bytes.IndexByte(b, 0) >= 0:
		return &Violation{Rule: RuleTextEncoding, Message: "SKILL.md contains a NUL byte"}
	}
	return nil
}

// CanonicalText returns the text the canonical hash is taken over: the file
// without a decision-projection header. A header is recognised only when the
// first line of the file is exactly HeaderOpen. It runs to the first line that
// is exactly "-->", and the one blank line after that line goes with it.
func CanonicalText(b []byte) ([]byte, *Violation) {
	first, _, _ := bytes.Cut(b, []byte("\n"))
	if string(first) != HeaderOpen {
		return b, nil
	}
	rest := b
	for {
		line, after, found := bytes.Cut(rest, []byte("\n"))
		if string(line) == headerClose {
			if !found {
				return nil, headerBad("the header closing line is the last line of the file")
			}
			blank, tail, ok := bytes.Cut(after, []byte("\n"))
			if len(blank) != 0 || !ok {
				return nil, headerBad("the header closing line is not followed by one blank line")
			}
			return tail, nil
		}
		if !found {
			return nil, headerBad("the header is not closed by a line \"-->\"")
		}
		rest = after
	}
}

func headerBad(msg string) *Violation {
	return &Violation{Rule: RuleHeaderMalformed, Message: msg}
}

// Hash returns the canonical hash of SKILL.md bytes: the lowercase hex sha256
// of CanonicalText.
func Hash(b []byte) (string, *Violation) {
	text, v := CanonicalText(b)
	if v != nil {
		return "", v
	}
	return HashOfCanonical(text), nil
}

// HashOfCanonical is the lowercase hex sha256 of text already stripped.
func HashOfCanonical(text []byte) string {
	sum := sha256.Sum256(text)
	return hex.EncodeToString(sum[:])
}
