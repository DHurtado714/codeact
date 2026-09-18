package runtime

import (
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
)

func TestRun_Success(t *testing.T) {
	r := New(time.Second, nil)
	res := r.Run(`print("hello", 1 + 1)`)
	if res.Err != nil {
		t.Fatalf("Run() error = %v, want nil", res.Err)
	}
	if res.Output != "hello 2\n" {
		t.Errorf("Run() output = %q, want %q", res.Output, "hello 2\n")
	}
}

func TestRun_SyntaxError(t *testing.T) {
	r := New(time.Second, nil)
	res := r.Run(`this is not valid js (((`)
	if res.Err == nil {
		t.Fatal("Run() error = nil, want syntax error")
	}
}

func TestRun_ThrownException(t *testing.T) {
	r := New(time.Second, nil)
	res := r.Run(`throw new Error("boom")`)
	if res.Err == nil {
		t.Fatal("Run() error = nil, want thrown exception")
	}
	if !strings.Contains(res.Err.Error(), "boom") {
		t.Errorf("Run() error = %v, want it to mention 'boom'", res.Err)
	}
}

func TestRun_CapturesOutputEvenOnError(t *testing.T) {
	r := New(time.Second, nil)
	res := r.Run(`print("before"); throw new Error("after")`)
	if res.Err == nil {
		t.Fatal("Run() error = nil, want thrown exception")
	}
	if res.Output != "before\n" {
		t.Errorf("Run() output = %q, want %q", res.Output, "before\n")
	}
}

func TestRun_Timeout(t *testing.T) {
	r := New(50*time.Millisecond, nil)
	start := time.Now()
	res := r.Run(`while (true) {}`)
	elapsed := time.Since(start)

	if res.Err == nil {
		t.Fatal("Run() error = nil, want timeout error")
	}
	if elapsed > 2*time.Second {
		t.Errorf("Run() took %v, want it to return promptly after timeout", elapsed)
	}
}

func TestRun_DoesNotPanicOnHostPanic(t *testing.T) {
	register := func(vm *goja.Runtime) {
		vm.Set("explode", func() {
			panic(vm.ToValue("host tool failed"))
		})
	}
	r := New(time.Second, register)
	res := r.Run(`explode()`)
	if res.Err == nil {
		t.Fatal("Run() error = nil, want error from host panic turned into JS exception")
	}
}

func TestRun_FreshVMPerCall(t *testing.T) {
	r := New(time.Second, nil)
	r.Run(`globalThis.leaked = 42`)
	res := r.Run(`print(typeof leaked)`)
	if res.Err != nil {
		t.Fatalf("Run() error = %v", res.Err)
	}
	if res.Output != "undefined\n" {
		t.Errorf("Run() output = %q, want %q (no state leaking across calls)", res.Output, "undefined\n")
	}
}

func TestRun_ReturnsPromptlyEvenIfHostToolHangs(t *testing.T) {
	register := func(vm *goja.Runtime) {
		vm.Set("hang", func() {
			time.Sleep(5 * time.Second) // outlives the interrupt grace period
		})
	}
	r := New(50*time.Millisecond, register)

	start := time.Now()
	res := r.Run(`hang()`)
	elapsed := time.Since(start)

	if res.Err == nil {
		t.Fatal("Run() error = nil, want timeout error")
	}
	if elapsed > 3*time.Second {
		t.Errorf("Run() took %v, want it to give up waiting on the stuck host call well under 3s", elapsed)
	}
}

func TestRun_RegisteredToolIsUsable(t *testing.T) {
	register := func(vm *goja.Runtime) {
		vm.Set("double", func(n int) int { return n * 2 })
	}
	r := New(time.Second, register)
	res := r.Run(`print(double(21))`)
	if res.Err != nil {
		t.Fatalf("Run() error = %v", res.Err)
	}
	if res.Output != "42\n" {
		t.Errorf("Run() output = %q, want %q", res.Output, "42\n")
	}
}
