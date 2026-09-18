package runtime

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DHurtado714/codeact/internal/tools"
)

func TestRun_ReadCsvSurvivesFilterAndMap(t *testing.T) {
	dir := t.TempDir()
	content := "id,amount\n1,100\n2,-50\n3,25\n"
	if err := os.WriteFile(filepath.Join(dir, "data.csv"), []byte(content), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	r := New(time.Second, tools.Register(dir))
	res := r.Run(`
		var rows = readCsv("data.csv");
		var positive = rows.filter(function(r) { return parseFloat(r.amount) > 0; });
		var ids = positive.map(function(r) { return r.id; });
		print(ids.join(","));
	`)
	if res.Err != nil {
		t.Fatalf("Run() error = %v, output = %q", res.Err, res.Output)
	}
	if res.Output != "1,3\n" {
		t.Errorf("Run() output = %q, want %q", res.Output, "1,3\n")
	}
}
