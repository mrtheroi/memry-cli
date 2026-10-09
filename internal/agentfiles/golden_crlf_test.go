package agentfiles_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// filesWithCR returns the files that match pattern and contain a carriage
// return.
func filesWithCR(t *testing.T, pattern string) []string {
	t.Helper()
	paths, err := filepath.Glob(pattern)
	if err != nil || len(paths) == 0 {
		t.Fatalf("no files match %q (err %v)", pattern, err)
	}
	var found []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.IndexByte(data, '\r') >= 0 {
			found = append(found, path)
		}
	}
	return found
}

func TestFilesWithCR_FindsACarriageReturn(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lf.txt"), []byte("a\nb\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	crlf := filepath.Join(dir, "crlf.txt")
	if err := os.WriteFile(crlf, []byte("a\r\nb\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := filesWithCR(t, filepath.Join(dir, "*.txt"))

	if len(got) != 1 || got[0] != crlf {
		t.Errorf("filesWithCR = %v, want [%s]", got, crlf)
	}
}

// The goldens hold byte-exact PHP output, so a checkout that turns LF into
// CRLF (Windows autocrlf) must fail loudly here instead of in an obscure
// diff elsewhere. .gitattributes pins eol=lf; this guards against it.
func TestGoldens_NoCRLF(t *testing.T) {
	if got := filesWithCR(t, filepath.Join("..", "*", "testdata", "*")); len(got) != 0 {
		t.Errorf("goldens with a carriage return (checked out with CRLF?): %v", got)
	}
}
