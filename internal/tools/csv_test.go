package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCsv_ParsesHeaders(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "data.csv")
	content := "id,amount,date\n1,100.50,2024-01-01\n2,-30,2024-01-02\n"
	if err := os.WriteFile(csvPath, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	rows, err := readCsv(dir, "data.csv")
	if err != nil {
		t.Fatalf("readCsv() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("readCsv() returned %d rows, want 2", len(rows))
	}
	if rows[0]["id"] != "1" || rows[0]["amount"] != "100.50" || rows[0]["date"] != "2024-01-01" {
		t.Errorf("row[0] = %+v, want id=1 amount=100.50 date=2024-01-01", rows[0])
	}
	if rows[1]["id"] != "2" || rows[1]["amount"] != "-30" {
		t.Errorf("row[1] = %+v, want id=2 amount=-30", rows[1])
	}
}

func TestReadCsv_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	csvPath := filepath.Join(dir, "empty.csv")
	if err := os.WriteFile(csvPath, []byte(""), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	rows, err := readCsv(dir, "empty.csv")
	if err != nil {
		t.Fatalf("readCsv() error = %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("readCsv() = %v, want empty slice", rows)
	}
}

func TestReadCsv_PathTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	if _, err := readCsv(dir, "../../etc/passwd"); err == nil {
		t.Fatal("readCsv() error = nil, want path traversal to be rejected")
	}
}

func TestReadCsv_MissingFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := readCsv(dir, "does-not-exist.csv"); err == nil {
		t.Fatal("readCsv() error = nil, want error for missing file")
	}
}
