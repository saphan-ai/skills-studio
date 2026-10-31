package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var writeExamples = flag.Bool("write-examples", false, "rewrite examples/signing from the demonstration key")

const (
	readFiles     = "../../examples/valid/read-files"
	readFilesHash = "7273c8112a081b0c93ac5b4f7a33f8dc4b6b50df2b55f0ae0dfdd8ff7000db2c"
	readGit       = "../../examples/valid/read-git"
	readGitHash   = "76a80fd3b7916e0e77efe5657da679a5f35aa136008bc1ae78bd1044e44dab07"
)

func do(args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = run(args, &out, &errb)
	return code, out.String(), errb.String()
}

// demoKey is the published demonstration key: its seed is public, so a
// signature made with it proves nothing about who signed.
func demoKey() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("skills-studio demonstration key: public, never use it to sign a real skill"))
	return ed25519.NewKeyFromSeed(seed[:])
}

func digest(t *testing.T, hexHash string) []byte {
	t.Helper()
	b, err := hex.DecodeString(hexHash)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVerifyPrintsTheStatedHashOfBothFleetSkills(t *testing.T) {
	for dir, want := range map[string]string{readFiles: readFilesHash, readGit: readGitHash} {
		code, out, errOut := do("verify", dir)
		if code != 0 || errOut != "" {
			t.Fatalf("%s: exit %d, stderr %q", dir, code, errOut)
		}
		if !strings.Contains(out, "canonical-sha256: "+want+"\n") {
			t.Errorf("%s: printed %q, want hash %s", dir, out, want)
		}
		if !strings.Contains(out, "signature: not checked") {
			t.Errorf("%s: output does not say the signature was not checked: %q", dir, out)
		}
	}
}

func TestExitCodes(t *testing.T) {
	invalid := "../../examples/invalid/sf-file-script/has-script"
	tmp := t.TempDir()
	sig := writeFile(t, tmp, "sig", nil)
	cases := []struct {
		name string
		args []string
		code int
	}{
		{"no arguments", nil, 2},
		{"unknown verb", []string{"check", readFiles}, 2},
		{"no directory", []string{"verify"}, 2},
		{"two directories", []string{"verify", readFiles, readGit}, 2},
		{"unknown flag", []string{"verify", readFiles, "--force"}, 2},
		{"flag without value", []string{"verify", readFiles, "--signature"}, 2},
		{"signature without key", []string{"verify", readFiles, "--signature", sig}, 2},
		{"key without signature", []string{"verify", readFiles, "--pubkey", sig}, 2},
		{"missing directory", []string{"verify", filepath.Join(tmp, "absent")}, 2},
		{"missing signature file", []string{"verify", readFiles, "--signature", filepath.Join(tmp, "absent"), "--pubkey", sig}, 2},
		{"refused skill", []string{"verify", invalid}, 1},
		{"refused skill with signature flags", []string{"verify", invalid, "--signature", sig, "--pubkey", sig}, 1},
		{"pass", []string{"verify", readFiles}, 0},
		{"help", []string{"--help"}, 0},
	}
	for _, c := range cases {
		if code, _, _ := do(c.args...); code != c.code {
			t.Errorf("%s: exit %d, want %d", c.name, code, c.code)
		}
	}
}

func TestRefusalNamesItsRuleOnStderr(t *testing.T) {
	code, out, errOut := do("verify", "../../examples/invalid/sf-file-script/has-script")
	if code != 1 || out != "" {
		t.Fatalf("exit %d, stdout %q", code, out)
	}
	if !strings.HasPrefix(errOut, "refused SF-FILE-SCRIPT: scripts: ") {
		t.Errorf("stderr %q does not name the rule and the path", errOut)
	}
}

func TestSignatureOffline(t *testing.T) {
	tmp := t.TempDir()
	priv := demoKey()
	pub := priv.Public().(ed25519.PublicKey)
	sig := ed25519.Sign(priv, digest(t, readFilesHash))

	pkix, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string][]byte{
		"raw":    pub,
		"hex":    []byte(hex.EncodeToString(pub) + "\n"),
		"base64": []byte(base64.StdEncoding.EncodeToString(pub) + "\n"),
		"pem":    pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkix}),
	}
	sigs := map[string][]byte{
		"raw":    sig,
		"hex":    []byte(hex.EncodeToString(sig) + "\n"),
		"base64": []byte(base64.StdEncoding.EncodeToString(sig) + "\n"),
	}
	for kn, kb := range keys {
		for sn, sb := range sigs {
			code, out, errOut := do("verify", readFiles,
				"--signature", writeFile(t, tmp, "s-"+sn, sb), "--pubkey", writeFile(t, tmp, "k-"+kn, kb))
			if code != 0 || !strings.Contains(out, "signature: ed25519 valid over canonical-sha256") {
				t.Errorf("key %s, signature %s: exit %d, out %q, err %q", kn, sn, code, out, errOut)
			}
		}
	}
}

func TestSignatureRefusals(t *testing.T) {
	tmp := t.TempDir()
	priv := demoKey()
	pub := priv.Public().(ed25519.PublicKey)
	good := ed25519.Sign(priv, digest(t, readFilesHash))
	pubFile := writeFile(t, tmp, "pub", []byte(base64.StdEncoding.EncodeToString(pub)))

	other := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, 32))
	otherPub := writeFile(t, tmp, "otherpub", []byte(base64.StdEncoding.EncodeToString(other.Public().(ed25519.PublicKey))))
	flipped := append([]byte(nil), good...)
	flipped[0] ^= 1
	overHexText := ed25519.Sign(priv, []byte(readFilesHash))
	overOtherSkill := ed25519.Sign(priv, digest(t, readGitHash))

	cases := []struct {
		name string
		sig  []byte
		key  string
		rule string
	}{
		{"flipped signature bit", flipped, pubFile, "SF-SIGNATURE-INVALID"},
		{"other key", good, otherPub, "SF-SIGNATURE-INVALID"},
		{"signature over the hex text", overHexText, pubFile, "SF-SIGNATURE-INVALID"},
		{"signature over another skill", overOtherSkill, pubFile, "SF-SIGNATURE-INVALID"},
		{"short signature", good[:10], pubFile, "SF-SIGNATURE-MALFORMED"},
		{"garbage signature", []byte("not a signature"), pubFile, "SF-SIGNATURE-MALFORMED"},
		{"short key", good, writeFile(t, tmp, "short", []byte("abcd")), "SF-PUBKEY-MALFORMED"},
		{"pem that is not a key", good, writeFile(t, tmp, "pem", pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("x")})), "SF-PUBKEY-MALFORMED"},
	}
	for _, c := range cases {
		sig := writeFile(t, tmp, "sig", c.sig)
		code, out, errOut := do("verify", readFiles, "--signature", sig, "--pubkey", c.key)
		if code != 1 || out != "" || !strings.HasPrefix(errOut, "refused "+c.rule+": ") {
			t.Errorf("%s: exit %d, stdout %q, stderr %q, want rule %s", c.name, code, out, errOut, c.rule)
		}
	}
}

// Negative control: a valid skill, signed, then one byte of SKILL.md changed.
// The hash moves and the old signature no longer verifies.
func TestOneByteChangeFailsTheSignature(t *testing.T) {
	tmp := t.TempDir()
	data, err := os.ReadFile(filepath.Join(readFiles, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(tmp, "read-files")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, skill, "SKILL.md", data)

	priv := demoKey()
	sig := writeFile(t, tmp, "sig", ed25519.Sign(priv, digest(t, readFilesHash)))
	pub := writeFile(t, tmp, "pub", priv.Public().(ed25519.PublicKey))

	if code, _, errOut := do("verify", skill, "--signature", sig, "--pubkey", pub); code != 0 {
		t.Fatalf("unmodified copy refused: %s", errOut)
	}

	i := bytes.Index(data, []byte("Reading files"))
	mutated := append([]byte(nil), data...)
	mutated[i] = 'r' // 'R' -> 'r': one byte
	writeFile(t, skill, "SKILL.md", mutated)

	code, out, errOut := do("verify", skill)
	if code != 0 || strings.Contains(out, readFilesHash) {
		t.Fatalf("modified skill: exit %d, out %q; the hash must move and the skill still be valid", code, out)
	}
	code, _, errOut = do("verify", skill, "--signature", sig, "--pubkey", pub)
	if code != 1 || !strings.HasPrefix(errOut, "refused SF-SIGNATURE-INVALID") {
		t.Fatalf("modified skill with old signature: exit %d, stderr %q", code, errOut)
	}
}

// The demonstration signature and key under examples/signing are regenerated
// deterministically. The flag -write-examples rewrites them; otherwise the
// test compares.
func TestSigningExamplesAreCurrent(t *testing.T) {
	priv := demoKey()
	files := map[string][]byte{
		"demo-public-key.b64": []byte(base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey)) + "\n"),
		"read-files.sig.b64":  []byte(base64.StdEncoding.EncodeToString(ed25519.Sign(priv, digest(t, readFilesHash))) + "\n"),
	}
	dir := "../../examples/signing"
	for name, want := range files {
		p := filepath.Join(dir, name)
		if *writeExamples {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, want, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		got, err := os.ReadFile(p)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s is missing or stale; run: go test ./cmd/skills -run TestSigningExamplesAreCurrent -args -write-examples", p)
		}
	}
	code, out, errOut := do("verify", readFiles,
		"--signature", filepath.Join(dir, "read-files.sig.b64"), "--pubkey", filepath.Join(dir, "demo-public-key.b64"))
	if code != 0 {
		t.Errorf("the committed demonstration does not verify: exit %d, %q %q", code, out, errOut)
	}
}
