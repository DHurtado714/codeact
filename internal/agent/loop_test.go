package agent

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DHurtado714/codeact/internal/llm"
	"github.com/DHurtado714/codeact/internal/runtime"
)

// scriptedProvider returns replies in order, one per Complete call, and
// records the message history it was given on each call.
type scriptedProvider struct {
	replies    []string
	err        error
	calls      int
	historyLen []int
}

func (p *scriptedProvider) Complete(ctx context.Context, system string, msgs []llm.Message) (string, error) {
	p.historyLen = append(p.historyLen, len(msgs))
	if p.err != nil {
		return "", p.err
	}
	if p.calls >= len(p.replies) {
		return "", errors.New("scriptedProvider: no more replies scripted")
	}
	r := p.replies[p.calls]
	p.calls++
	return r, nil
}

func (p *scriptedProvider) Name() string { return "scripted" }

func TestTurn_ImmediateFinalAnswer(t *testing.T) {
	p := &scriptedProvider{replies: []string{"The answer is 42."}}
	l := New(p, runtime.New(time.Second, nil), &bytes.Buffer{})

	out, err := l.Turn(context.Background(), "what is the answer?")
	if err != nil {
		t.Fatalf("Turn() error = %v", err)
	}
	if out != "The answer is 42." {
		t.Errorf("Turn() = %q, want %q", out, "The answer is 42.")
	}
	if p.calls != 1 {
		t.Errorf("provider called %d times, want 1", p.calls)
	}
}

func TestTurn_CodeThenFinalAnswer(t *testing.T) {
	p := &scriptedProvider{replies: []string{
		"```js\nprint(1 + 1)\n```",
		"The sum is 2.",
	}}
	var trace bytes.Buffer
	l := New(p, runtime.New(time.Second, nil), &trace)

	out, err := l.Turn(context.Background(), "what is 1+1?")
	if err != nil {
		t.Fatalf("Turn() error = %v", err)
	}
	if out != "The sum is 2." {
		t.Errorf("Turn() = %q, want %q", out, "The sum is 2.")
	}
	if p.calls != 2 {
		t.Errorf("provider called %d times, want 2", p.calls)
	}
	if trace.Len() == 0 {
		t.Error("expected trace output to be written, got nothing")
	}
}

func TestTurn_ObservationFeedsBackAsUserMessage(t *testing.T) {
	p := &scriptedProvider{replies: []string{
		"```js\nprint(\"hi\")\n```",
		"done",
	}}
	l := New(p, runtime.New(time.Second, nil), &bytes.Buffer{})

	if _, err := l.Turn(context.Background(), "go"); err != nil {
		t.Fatalf("Turn() error = %v", err)
	}
	// history: user(go), assistant(code), user(observation) -> second Complete call sees 3 messages
	if len(p.historyLen) != 2 || p.historyLen[1] != 3 {
		t.Errorf("historyLen = %v, want second call to see 3 messages", p.historyLen)
	}
	// history: user(go), assistant(code), user(observation), assistant(done)
	observation := l.history[2]
	if observation.Role != "user" || observation.Content != "hi\n" {
		t.Errorf("observation message = %+v, want user message with output 'hi\\n'", observation)
	}
}

func TestTurn_ErrorObservationFeedsBack(t *testing.T) {
	p := &scriptedProvider{replies: []string{
		"```js\nthrow new Error(\"boom\")\n```",
		"fixed it",
	}}
	l := New(p, runtime.New(time.Second, nil), &bytes.Buffer{})

	if _, err := l.Turn(context.Background(), "go"); err != nil {
		t.Fatalf("Turn() error = %v", err)
	}
	found := false
	for _, m := range l.history {
		if m.Role == "user" && m.Content != "go" {
			found = true
			if !strings.Contains(m.Content, "boom") {
				t.Errorf("observation = %q, want it to mention 'boom'", m.Content)
			}
		}
	}
	if !found {
		t.Error("expected an observation message from the failed execution")
	}
}

func TestTurn_LLMErrorDropsOrphanMessage(t *testing.T) {
	p := &scriptedProvider{err: errors.New("network down")}
	l := New(p, runtime.New(time.Second, nil), &bytes.Buffer{})

	_, err := l.Turn(context.Background(), "go")
	if err == nil {
		t.Fatal("Turn() error = nil, want error")
	}
	if len(l.history) != 0 {
		t.Errorf("history = %v, want empty (orphan user message removed)", l.history)
	}
}

func TestTurn_MaxIterations(t *testing.T) {
	replies := make([]string, 0, maxIterations)
	for i := 0; i < maxIterations; i++ {
		replies = append(replies, "```js\nprint(\"again\")\n```")
	}
	p := &scriptedProvider{replies: replies}
	l := New(p, runtime.New(time.Second, nil), &bytes.Buffer{})

	_, err := l.Turn(context.Background(), "go")
	if err == nil {
		t.Fatal("Turn() error = nil, want error after exceeding max iterations")
	}
	if p.calls != maxIterations {
		t.Errorf("provider called %d times, want %d", p.calls, maxIterations)
	}
}
