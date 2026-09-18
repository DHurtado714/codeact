// Package runtime wraps a goja VM to execute untrusted JS snippets safely.
package runtime

import (
	"bytes"
	"fmt"
	"time"

	"github.com/dop251/goja"
)

const defaultTimeout = 10 * time.Second

// Result is what a Run call produces. Err is never a Go-level failure of
// Run itself — it's the JS program's own failure (syntax error, thrown
// exception, or timeout), which the agent loop feeds back to the model as
// an observation so it can try again.
type Result struct {
	Output string
	Err    error
}

// Register is called once against a fresh VM to install host functions
// (print, readCsv, ...) before the program runs.
type Register func(vm *goja.Runtime)

// Runtime executes JS snippets with a bounded timeout.
type Runtime struct {
	timeout  time.Duration
	register Register
}

// New builds a Runtime. register installs whatever tools the program should
// see; timeout <= 0 uses the default of 10s.
func New(timeout time.Duration, register Register) *Runtime {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Runtime{timeout: timeout, register: register}
}

// Run executes code in a brand-new VM and returns what it printed and/or
// how it failed. It never panics and never returns a Go error directly.
//
// A new goja.Runtime is created per call rather than reused across turns:
// each agent turn should start from a clean slate, with no globals or state
// leaking from a previous (possibly failed) attempt.
func (r *Runtime) Run(code string) Result {
	vm := goja.New()
	var out bytes.Buffer

	print := func(args ...goja.Value) {
		for i, a := range args {
			if i > 0 {
				out.WriteByte(' ')
			}
			out.WriteString(a.String())
		}
		out.WriteByte('\n')
	}
	vm.Set("print", print)

	if r.register != nil {
		r.register(vm)
	}

	// The channel is buffered so that if the timeout branch of the select
	// fires first, the goroutine below can still send its result (or panic
	// recovery) without blocking forever — otherwise it would leak.
	done := make(chan error, 1)

	go func() {
		defer func() {
			if p := recover(); p != nil {
				done <- fmt.Errorf("interpreter panic: %v", p)
			}
		}()
		_, err := vm.RunString(code)
		done <- err
	}()

	select {
	case err := <-done:
		return Result{Output: out.String(), Err: err}
	case <-time.After(r.timeout):
		vm.Interrupt("execution timed out")
		<-done // drain so the goroutine's send doesn't leak
		return Result{Output: out.String(), Err: fmt.Errorf("execution timed out after %s", r.timeout)}
	}
}
