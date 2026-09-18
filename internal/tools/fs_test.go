package tools

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestListFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644)

	files, err := listFiles(dir, ".")
	if err != nil {
		t.Fatalf("listFiles() error = %v", err)
	}
	sort.Strings(files)
	if len(files) != 2 || files[0] != "a.txt" || files[1] != "b.txt" {
		t.Errorf("listFiles() = %v, want [a.txt b.txt]", files)
	}
}

func TestReadWriteFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := writeFile(dir, "out.txt", "hello world"); err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}
	content, err := readFile(dir, "out.txt")
	if err != nil {
		t.Fatalf("readFile() error = %v", err)
	}
	if content != "hello world" {
		t.Errorf("readFile() = %q, want %q", content, "hello world")
	}
}

func TestPathTraversal_Rejected(t *testing.T) {
	dir := t.TempDir()

	cases := []string{
		"../secret.txt",
		"../../etc/passwd",
		"foo/../../bar",
	}
	for _, p := range cases {
		if _, err := readFile(dir, p); err == nil {
			t.Errorf("readFile(%q) error = nil, want path traversal rejected", p)
		}
		if err := writeFile(dir, p, "x"); err == nil {
			t.Errorf("writeFile(%q) error = nil, want path traversal rejected", p)
		}
		if _, err := listFiles(dir, p); err == nil {
			t.Errorf("listFiles(%q) error = nil, want path traversal rejected", p)
		}
	}
}

func TestSymlinkEscape_Rejected(t *testing.T) {
	workdir := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("top secret"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	link := filepath.Join(workdir, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks not supported in this environment: %v", err)
	}

	if _, err := readFile(workdir, "escape/secret.txt"); err == nil {
		t.Error("readFile() error = nil, want symlink escape to be rejected")
	}
	if err := writeFile(workdir, "escape/overwrite.txt", "pwned"); err == nil {
		t.Error("writeFile() error = nil, want symlink escape to be rejected")
	}
	if _, err := listFiles(workdir, "escape"); err == nil {
		t.Error("listFiles() error = nil, want symlink escape to be rejected")
	}
}

func TestDotfile_Rejected(t *testing.T) {
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, ".env"), []byte("LLM_API_KEY=sk-secret"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if _, err := readFile(workdir, ".env"); err == nil {
		t.Error("readFile(\".env\") error = nil, want dotfile access rejected")
	}
	if _, err := readFile(workdir, "sub/.git/config"); err == nil {
		t.Error("readFile(\"sub/.git/config\") error = nil, want dotfile access rejected")
	}
}

func TestAbsolutePath_TreatedAsRelativeToWorkdir(t *testing.T) {
	// filepath.Join treats a leading "/" in the second argument as just
	// another path segment, so an absolute-looking path stays contained
	// inside workdir instead of escaping to the real filesystem root.
	dir := t.TempDir()
	if err := writeFile(dir, "/out.txt", "hi"); err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out.txt")); err != nil {
		t.Errorf("expected file to land inside workdir: %v", err)
	}
}
