//go:build windows

package main

import (
	"testing"
	"unsafe"
)

func TestShellExecuteInfoLayout(t *testing.T) {
	want := uintptr(112)
	if unsafe.Sizeof(uintptr(0)) == 4 {
		want = 60
	}
	if got := unsafe.Sizeof(shellExecuteInfo{}); got != want {
		t.Fatalf("SHELLEXECUTEINFOW size = %d, want %d", got, want)
	}
}
