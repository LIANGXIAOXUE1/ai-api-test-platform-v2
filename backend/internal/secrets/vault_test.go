package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeyRestartAndWrongKey(t *testing.T) {
	dir := t.TempDir()
	v, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := v.Seal("test-secret-key")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sealed, Prefix) || strings.Contains(sealed, "test-secret-key") {
		t.Fatal("plaintext stored")
	}
	v2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := v2.Unseal(sealed)
	if err != nil || plain != "test-secret-key" {
		t.Fatal("restart lost credential")
	}
	wrong, _ := Open(t.TempDir())
	if _, err := wrong.Unseal(sealed); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err := v.Unseal("plaintext"); err == nil {
		t.Fatal("unmigrated plaintext accepted")
	}
	if _, err := v.Unseal(sealed + "bad"); err == nil {
		t.Fatal("tampered cipher accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, "project-test.key"), []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("invalid key silently replaced")
	}
}
