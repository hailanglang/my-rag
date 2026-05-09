package parse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractTxt(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(p, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Extract(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractMd(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "readme.md")
	body := "# Title\n\nhello"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Extract(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != body {
		t.Fatalf("got %q", got)
	}
}

func TestExtract_unsupportedExt(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.docx")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Extract(p)
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("err=%v", err)
	}
}

func TestExtract_missingFile(t *testing.T) {
	_, err := Extract(filepath.Join(t.TempDir(), "nope.txt"))
	if err == nil || !os.IsNotExist(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestExtractPdf_fixture(t *testing.T) {
	p := filepath.Join("testdata", "dummy.pdf")
	if _, err := os.Stat(p); err != nil {
		t.Skip("testdata/dummy.pdf missing")
	}
	got, err := Extract(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) == "" {
		t.Fatal("expected non-empty text from fixture")
	}
}

func TestExtractPdf_garbage(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.pdf")
	if err := os.WriteFile(p, []byte("not a pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Extract(p)
	if err == nil {
		t.Fatal("expected error")
	}
}

