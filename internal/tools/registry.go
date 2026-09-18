// Package tools implements the host functions exposed to JS: CSV/file
// access scoped to a workdir, all path-traversal safe.
package tools

import "github.com/dop251/goja"

// Register returns a runtime.Register-compatible function that installs
// readCsv, listFiles, readFile and writeFile into vm, all scoped to workdir.
//
// Each Go function that can fail is wrapped in a closure that panics with
// vm.ToValue(err.Error()) instead of returning (value, error) directly —
// goja would otherwise turn a Go (T, error) return into a two-element JS
// array, which is awkward to consume. A panic inside a host function
// becomes a normal catchable JS exception instead.
func Register(workdir string) func(vm *goja.Runtime) {
	return func(vm *goja.Runtime) {
		vm.Set("readCsv", func(path string) []map[string]string {
			rows, err := readCsv(workdir, path)
			if err != nil {
				panic(vm.ToValue(err.Error()))
			}
			return rows
		})

		vm.Set("listFiles", func(dir string) []string {
			files, err := listFiles(workdir, dir)
			if err != nil {
				panic(vm.ToValue(err.Error()))
			}
			return files
		})

		vm.Set("readFile", func(path string) string {
			content, err := readFile(workdir, path)
			if err != nil {
				panic(vm.ToValue(err.Error()))
			}
			return content
		})

		vm.Set("writeFile", func(path, content string) {
			if err := writeFile(workdir, path, content); err != nil {
				panic(vm.ToValue(err.Error()))
			}
		})
	}
}
