package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolvePath joins path onto workdir and rejects anything that would
// escape it: a lexical "../.." climb, a symlink inside workdir that points
// outside it, or a dotfile (".env", ".git", ...) — the model's JS has no way
// to create a symlink itself, but a pre-existing one in workdir (or in a
// directory a careless -dir points at) would otherwise let it read or write
// past the sandbox boundary, and dotfiles are where credentials like the
// agent's own .env tend to live right next to the CSVs it's supposed to read.
func resolvePath(workdir, path string) (string, error) {
	full := filepath.Join(workdir, path)
	rel, err := filepath.Rel(workdir, full)
	if err != nil {
		return "", fmt.Errorf("invalid path %q", path)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes the working directory", path)
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part != "." && strings.HasPrefix(part, ".") {
			return "", fmt.Errorf("path %q refers to a hidden file or directory, which is not allowed", path)
		}
	}

	resolvedWorkdir, err := filepath.EvalSymlinks(workdir)
	if err != nil {
		return "", fmt.Errorf("resolve workdir: %w", err)
	}
	resolvedFull, err := resolveExistingSymlinks(full)
	if err != nil {
		return "", fmt.Errorf("invalid path %q: %w", path, err)
	}
	resolvedRel, err := filepath.Rel(resolvedWorkdir, resolvedFull)
	if err != nil || resolvedRel == ".." || strings.HasPrefix(resolvedRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes the working directory", path)
	}

	return full, nil
}

// resolveExistingSymlinks resolves symlinks on the deepest ancestor of path
// that actually exists, then reapplies the (not-yet-existing) remainder.
// filepath.EvalSymlinks requires the path to exist, but writeFile's target
// often doesn't yet — this lets a not-yet-created file still be checked
// against a symlinked ancestor directory.
func resolveExistingSymlinks(path string) (string, error) {
	for p := path; ; {
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			rest, err := filepath.Rel(p, path)
			if err != nil {
				return "", err
			}
			return filepath.Join(resolved, rest), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", fmt.Errorf("no existing ancestor directory found")
		}
		p = parent
	}
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
