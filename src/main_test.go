package main

import (
	"os/exec"
	"testing"
)

func BenchmarkCompileWithLLC(b *testing.B) {
	cmd := exec.Command("llc", "-filetype=obj", "-o", "../tmp_compile/example", "../tmp_compile/example.cffc.ll")
	cmd.Run()
}

func BenchmarkCompileWithClang(b *testing.B) {
	cmd := exec.Command("clang", "-o", "../tmp_compile/example", "../tmp_compile/example.cffc.ll")
	cmd.Run()
}
