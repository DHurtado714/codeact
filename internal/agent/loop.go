// Package agent implements the think -> code -> execute -> observe loop.
package agent

import (
	"context"
	"fmt"
	"io"

	"github.com/DHurtado714/codeact/internal/llm"
	"github.com/DHurtado714/codeact/internal/runtime"
)

// maxIterations bounds how many code executions a single Turn may take
// before giving up, so a model stuck in a retry loop can't run forever.
const maxIterations = 8

// Loop drives one conversation: it keeps the message history across turns
// and, within a turn, cycles the model through writing code, running it,
// and reading back the result until it gives a plain-text final answer.
type Loop struct {
	provider llm.Provider
	rt       *runtime.Runtime
	out      io.Writer
	history  []llm.Message
}

// New builds a Loop. out receives a human-readable trace of every code
// block executed and its output/error, for display in the terminal.
func New(provider llm.Provider, rt *runtime.Runtime, out io.Writer) *Loop {
	return &Loop{provider: provider, rt: rt, out: out}
}

// Turn appends userInput to the history, then drives the think/code/execute
// loop until the model replies with plain text (no code block) or the
// iteration budget runs out. It returns that final text.
func (l *Loop) Turn(ctx context.Context, userInput string) (string, error) {
	l.history = append(l.history, llm.Message{Role: "user", Content: userInput})

	for i := 0; i < maxIterations; i++ {
		reply, err := l.provider.Complete(ctx, SystemPrompt, l.history)
		if err != nil {
			// The trailing user message has no matching assistant reply now;
			// drop it so the next call keeps strict user/assistant alternation.
			l.history = l.history[:len(l.history)-1]
			return "", fmt.Errorf("llm call failed: %w", err)
		}
		l.history = append(l.history, llm.Message{Role: "assistant", Content: reply})

		code, ok := ExtractCode(reply)
		if !ok {
			return reply, nil
		}

		fmt.Fprintf(l.out, "\n--- code ---\n%s\n", code)
		res := l.rt.Run(code)

		observation := res.Output
		if res.Err != nil {
			fmt.Fprintf(l.out, "--- error ---\n%s\n", res.Err)
			observation += "Error: " + res.Err.Error()
		} else {
			fmt.Fprintf(l.out, "--- output ---\n%s\n", res.Output)
			if observation == "" {
				observation = "(no output)"
			}
		}
		l.history = append(l.history, llm.Message{Role: "user", Content: observation})
	}

	return "", fmt.Errorf("reached max iterations (%d) without a final answer", maxIterations)
}
