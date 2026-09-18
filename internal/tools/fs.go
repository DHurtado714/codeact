package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolvePath joins path onto workdir and rejects anything that would
// escape it (e.g. "../../etc/passwd" or an absolute path).
func resolvePath(workdir, path string) (string, error) {
	full := filepath.Join(workdir, path)
	rel, err := filepath.Rel(workdir, full)
	if err != nil {
		return "", fmt.Errorf("invalid path %q", path)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes the working directory", path)
	}
	return full, nil
}

func listFiles(workdir, dir string) ([]string, error) {
	full, err := resolvePath(workdir, dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, fmt.Errorf("listFiles: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}

func readFile(workdir, path string) (string, error) {
	full, err := resolvePath(workdir, path)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return "", fmt.Errorf("readFile: %w", err)
	}
	return string(content), nil
}

func writeFile(workdir, path, content string) error {
	full, err := resolvePath(workdir, path)
	if err != nil {
		return err
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writeFile: %w", err)
	}
	return nil
}
