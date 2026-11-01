package skillfmt

import (
	"bytes"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// C2: a header followed by more than one blank line is not a header-malformed
// file: one blank line goes with the header, the next makes the canonical text
// begin with an empty line, so the front matter is missing.
func TestHeaderFollowedByTwoBlankLinesIsFrontMatterMissing(t *testing.T) {
	body := "---\nname: demo\ndescription: d\n---\n\n# Demo\n"
	text := HeaderOpen + "\n_id: x\n-->\n\n\n" + body
	if _, v := Hash([]byte(text)); v != nil {
		t.Fatalf("the hash step refused it: %+v", v)
	}
	res, err := Verify(writeSkill(t, "demo", text))
	if err != nil {
		t.Fatal(err)
	}
	if got := rulesOf(res.Violations); len(got) != 1 || got[0] != RuleFrontMatterMissing {
		t.Errorf("refused by %v, want only %s", got, RuleFrontMatterMissing)
	}
}

// C2: the header is outside the hash and the signature, so the texts say that a
// consumer gives an agent the canonical text and never the stored file.
func TestPublicTextSaysTheHeaderIsNotCovered(t *testing.T) {
	read := func(parts ...string) string {
		b, err := os.ReadFile(repoPath(parts...))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	specText := read("spec", "SKILL_FORMAT_V1.md")
	section := func(text, from, to string) string {
		i := strings.Index(text, from)
		j := strings.Index(text, to)
		if i < 0 || j < i {
			t.Fatalf("sections %q..%q not found", from, to)
		}
		return text[i:j]
	}
	s31 := section(specText, "### 3.1 ", "## 4. ")
	s10 := section(specText, "## 10. ", "## 11. ")
	for name, c := range map[string][2]string{
		"spec 3.1 two blank lines": {s31, "SF-FRONTMATTER-MISSING"},
		"spec 3.1 not covered":     {s31, "is not covered by the canonical hash or by any signature"},
		"spec 3.1 consumer MUST":   {s31, "MUST give an agent the canonical text"},
		"spec 10 not covered":      {s10, "is not covered by the hash or by a signature"},
		"spec 10 consumer MUST":    {s10, "MUST give an agent the canonical text"},
		"README not covered":       {read("README.md"), "is not covered by the hash or by a signature"},
		"README consumer MUST":     {read("README.md"), "MUST give an agent the canonical text"},
		"SECURITY canonical":       {read("SECURITY.md"), "two different canonical texts"},
	} {
		if !strings.Contains(strings.Join(strings.Fields(c[0]), " "), c[1]) {
			t.Errorf("%s: %q is missing", name, c[1])
		}
	}
	if strings.Contains(read("SECURITY.md"), "two different `SKILL.md` texts") {
		t.Error("SECURITY.md still says two different SKILL.md texts")
	}
}

// C3: the small-order public keys. The list is checked here against the group
// law, not against memory: each of the eight points must satisfy 8P = identity
// and they must be distinct, which is every point of that order since the
// curve's group has a cyclic 2-part of order 8.
func TestSmallOrderListIsExactlyTheTorsionPoints(t *testing.T) {
	p := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(19))
	d := new(big.Int).Mul(big.NewInt(-121665), new(big.Int).ModInverse(big.NewInt(121666), p))
	d.Mod(d, p)
	type pt struct{ x, y *big.Int }
	add := func(a, b pt) pt {
		xy := new(big.Int).Mul(new(big.Int).Mul(a.x, b.x), new(big.Int).Mul(a.y, b.y))
		dxy := new(big.Int).Mul(d, xy)
		dxy.Mod(dxy, p)
		x := new(big.Int).Add(new(big.Int).Mul(a.x, b.y), new(big.Int).Mul(b.x, a.y))
		x.Mul(x, new(big.Int).ModInverse(new(big.Int).Add(big.NewInt(1), dxy), p)).Mod(x, p)
		y := new(big.Int).Add(new(big.Int).Mul(a.y, b.y), new(big.Int).Mul(a.x, b.x))
		den := new(big.Int).Sub(big.NewInt(1), dxy)
		den.Mod(den, p)
		y.Mul(y, new(big.Int).ModInverse(den, p)).Mod(y, p)
		return pt{x, y}
	}
	sqrtM1 := new(big.Int).Exp(big.NewInt(2), new(big.Int).Rsh(new(big.Int).Sub(p, big.NewInt(1)), 2), p)
	seen := map[string]bool{}
	for _, y := range smallOrderY {
		// recover x from y: x^2 = (y^2-1)/(d*y^2+1)
		y2 := new(big.Int).Mul(y, y)
		num := new(big.Int).Sub(y2, big.NewInt(1))
		den := new(big.Int).Add(new(big.Int).Mul(d, y2), big.NewInt(1))
		x2 := num.Mul(num, new(big.Int).ModInverse(den.Mod(den, p), p))
		x2.Mod(x2, p)
		x := new(big.Int).Exp(x2, new(big.Int).Rsh(new(big.Int).Add(p, big.NewInt(3)), 3), p)
		if new(big.Int).Mod(new(big.Int).Mul(x, x), p).Cmp(x2) != 0 {
			x.Mul(x, sqrtM1).Mod(x, p)
		}
		if new(big.Int).Mod(new(big.Int).Mul(x, x), p).Cmp(x2) != 0 {
			t.Fatalf("y=%v is not on the curve", y)
		}
		for _, xs := range []*big.Int{x, new(big.Int).Mod(new(big.Int).Neg(x), p)} {
			key := xs.String() + "," + y.String()
			if seen[key] {
				continue
			}
			seen[key] = true
			q := pt{xs, y}
			for i := 0; i < 3; i++ { // 2^3 = 8
				q = add(q, q)
			}
			if q.x.Sign() != 0 || q.y.Cmp(big.NewInt(1)) != 0 {
				t.Errorf("point (%v, %v) does not satisfy 8P = identity", xs, y)
			}
		}
	}
	if len(seen) != 8 {
		t.Errorf("the list yields %d distinct small-order points, want 8", len(seen))
	}
}

func TestSmallOrderPublicKeysAreRefused(t *testing.T) {
	enc := func(first byte, mid, last byte) []byte {
		b := bytes.Repeat([]byte{mid}, 32)
		b[0], b[31] = first, last
		return b
	}
	identity := make([]byte, 32)
	identity[0] = 1
	zero := make([]byte, 32)
	cases := map[string][]byte{
		"identity 01 00..00":       identity,
		"zero 00..00":              zero,
		"minus one ec ff..ff 7f":   enc(0xec, 0xff, 0x7f),
		"p non-canonical":          enc(0xed, 0xff, 0x7f),
		"p+1 non-canonical":        enc(0xee, 0xff, 0x7f),
		"identity with sign bit":   append(append([]byte{1}, make([]byte, 30)...), 0x80),
		"zero with sign bit":       append(make([]byte, 31), 0x80),
		"order 8 (libsodium list)": mustDecodeHex(t, "26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc05"),
		"order 8, other":           mustDecodeHex(t, "c7176a703d4dd84fba3c0b760d10670f2a2053fa2c39ccc64ec7fd7792ac037a"),
		"order 8, sign bit":        mustDecodeHex(t, "26e8958fc2b227b045c3f489f2ef98f0d5dfac05d3c63339b13802886d53fc85"),
	}
	for name, key := range cases {
		if _, v := ParsePublicKey(key); v == nil || v.Rule != RulePubkeyMalformed {
			t.Errorf("%s: got %+v, want %s", name, v, RulePubkeyMalformed)
		}
	}
	// A key that is not of small order is still read.
	good := bytes.Repeat([]byte{9}, 32)
	if _, v := ParsePublicKey(good); v != nil {
		t.Errorf("an ordinary 32-byte key was refused: %+v", v)
	}
	// The same refusal through CheckSignature, with the signature of the report: R = identity, S = 0.
	sig := append(append([]byte{1}, make([]byte, 31)...), make([]byte, 32)...)
	if v := CheckSignature(strings.Repeat("ab", 32), sig, identity); v == nil || v.Rule != RulePubkeyMalformed {
		t.Errorf("CheckSignature with the identity key: got %+v, want %s", v, RulePubkeyMalformed)
	}
}

func mustDecodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := DecodeHash(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// C4: an unreadable directory under references/ is an error, never skipped.
func TestUnreadableDataDirectoryIsNotSkipped(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod 000 does not deny reads here")
	}
	for _, top := range []string{"references", "assets"} {
		dir := writeSkill(t, "demo", skillText(""))
		locked := filepath.Join(dir, top, "locked")
		if err := os.MkdirAll(locked, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(locked, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
		res, err := Verify(dir)
		if err == nil {
			t.Errorf("%s: an unreadable subdirectory was skipped: passed=%v violations=%+v", top, res.Passed(), res.Violations)
		}
	}
}

// C4: the skill root is not a symbolic link.
func TestSymlinkedSkillRootIsRefused(t *testing.T) {
	real := writeSkill(t, "other", skillText(""))
	link := filepath.Join(t.TempDir(), "demo")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, arg := range []string{link, link + string(filepath.Separator)} {
		res, err := Verify(arg)
		if err != nil {
			t.Fatal(err)
		}
		if got := rulesOf(res.Violations); len(got) != 1 || got[0] != RuleFileSymlink || res.Passed() {
			t.Errorf("%q: refused by %v (passed=%v), want only %s", arg, got, res.Passed(), RuleFileSymlink)
		}
	}
}
