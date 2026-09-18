// Command agent is an interactive CodeAct financial reconciliation agent.
// It reads natural-language questions from stdin, has an LLM answer them by
// writing and running JavaScript against CSV files in a working directory,
// and prints the result.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DHurtado714/codeact/internal/agent"
	"github.com/DHurtado714/codeact/internal/llm"
	"github.com/DHurtado714/codeact/internal/runtime"
	"github.com/DHurtado714/codeact/internal/tools"
)

func main() {
	dir := flag.String("dir", ".", "working directory containing the CSV files")
	timeout := flag.Duration("timeout", 10*time.Second, "JS execution timeout per code block")
	flag.Parse()

	absDir, err := loadWorkdir(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	loadDotEnv(".env")

	provider, err := llm.NewProviderFromEnv(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	rt := runtime.New(*timeout, tools.Register(absDir))
	loop := agent.New(provider, rt, os.Stdout)

	fmt.Printf("codeact agent ready (provider=%s, dir=%s). Type a question, or 'exit' to quit.\n", provider.Name(), absDir)
	runREPL(loop, os.Stdin, os.Stdout)
}

func loadWorkdir(dir string) (string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("working directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("working directory %q is not a directory", dir)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving working directory %q: %w", dir, err)
	}
	return abs, nil
}

// loadDotEnv sets any KEY=VALUE lines from path into the process environment,
// without overriding vars already set. Missing file is not an error — env
// vars set directly in the shell are enough.
func loadDotEnv(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		if _, alreadySet := os.LookupEnv(key); alreadySet {
			continue
		}
		os.Setenv(key, strings.TrimSpace(value))
	}
}

func runREPL(loop *agent.Loop, in *os.File, out *os.File) {
	scanner := bufio.NewScanner(in)
	ctx := context.Background()

	for {
		fmt.Fprint(out, "\n> ")
		if !scanner.Scan() {
			return
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			return
		}

		answer, err := loop.Turn(ctx, input)
		if err != nil {
			fmt.Fprintln(out, "error:", err)
			continue
		}
		fmt.Fprintln(out, "\n"+answer)
	}
}
