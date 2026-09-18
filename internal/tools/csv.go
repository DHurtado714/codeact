package tools

import (
	"encoding/csv"
	"fmt"
	"os"
)

// readCsv parses path (relative to workdir) as CSV, using the first row as
// column headers. Every value comes back as a string — the JS side is
// responsible for any numeric conversion (e.g. parseFloat on an "amount"
// column).
func readCsv(workdir, path string) ([]map[string]string, error) {
	full, err := resolvePath(workdir, path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(full)
	if err != nil {
		return nil, fmt.Errorf("readCsv: %w", err)
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("readCsv: %w", err)
	}
	if len(records) == 0 {
		return []map[string]string{}, nil
	}

	headers := records[0]
	rows := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		row := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(record) {
				row[h] = record[i]
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}
