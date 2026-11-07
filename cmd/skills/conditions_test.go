package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A signature file made of the identity point and a zero scalar verifies under
// the identity key in a plain ed25519 verifier. The key is refused instead.
func TestSmallOrderKeyCannotVerifyAnySkill(t *testing.T) {
	tmp := t.TempDir()
	key := append([]byte{1}, make([]byte, 31)...)
	sig := append(append([]byte{1}, make([]byte, 31)...), make([]byte, 32)...)
	for _, skill := range []string{readFiles, readGit} {
		code, out, errOut := do("verify", skill, "--signature", writeFile(t, tmp, "sig", sig), "--pubkey", writeFile(t, tmp, "pub", key))
		if code != 1 || out != "" || !strings.HasPrefix(errOut, "refused SF-PUBKEY-MALFORMED: ") {
			t.Errorf("%s: exit %d, stdout %q, stderr %q", skill, code, out, errOut)
		}
	}
}

func TestUnreadableDataDirectoryExitsTwo(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("chmod 000 does not deny reads here")
	}
	dir := filepath.Join(t.TempDir(), "read-files")
	locked := filepath.Join(dir, "references", "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(readFiles, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "SKILL.md", data)
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	if code, out, errOut := do("verify", dir); code != 2 || out != "" || errOut == "" {
		t.Errorf("exit %d, stdout %q, stderr %q, want exit 2", code, out, errOut)
	}
}

func TestSymlinkedSkillRootExitsOne(t *testing.T) {
	link := filepath.Join(t.TempDir(), "read-files")
	abs, err := filepath.Abs(readFiles)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(abs, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if code, out, errOut := do("verify", link); code != 1 || out != "" || !strings.Contains(errOut, "refused SF-FILE-SYMLINK: ") || !strings.Contains(errOut, "refused AS-UNREAD: ") {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}
