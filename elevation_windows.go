//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const seeMaskNoCloseProcess = 0x00000040

var shellExecuteEx = syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteExW")

// shellExecuteInfo matches the Windows SHELLEXECUTEINFOW layout.
type shellExecuteInfo struct {
	size       uint32
	mask       uint32
	window     uintptr
	verb       *uint16
	file       *uint16
	parameters *uint16
	directory  *uint16
	show       int32
	instance   uintptr
	idList     uintptr
	class      *uint16
	classKey   uintptr
	hotKey     uint32
	icon       uintptr
	process    uintptr
}

func elevatePrepareMod(gamePath, action, payloadPath, expectedHash string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	resultFile, err := os.CreateTemp("", "alpharing-elevation-*.txt")
	if err != nil {
		return err
	}
	resultPath := resultFile.Name()
	_ = resultFile.Close()
	defer os.Remove(resultPath)

	arguments := []string{"--elevated-mod", action, gamePath, payloadPath, expectedHash, resultPath}
	for i := range arguments {
		arguments[i] = syscall.EscapeArg(arguments[i])
	}
	info := shellExecuteInfo{
		mask:       seeMaskNoCloseProcess,
		verb:       syscall.StringToUTF16Ptr("runas"),
		file:       syscall.StringToUTF16Ptr(executable),
		parameters: syscall.StringToUTF16Ptr(strings.Join(arguments, " ")),
		show:       1,
	}
	info.size = uint32(unsafe.Sizeof(info))
	started, _, callErr := shellExecuteEx.Call(uintptr(unsafe.Pointer(&info)))
	if started == 0 {
		if errors.Is(callErr, syscall.Errno(1223)) {
			return fmt.Errorf("UAC permission was canceled")
		}
		return fmt.Errorf("could not start elevated file helper: %w", callErr)
	}
	if info.process == 0 {
		return fmt.Errorf("elevated file helper did not return a process handle")
	}
	defer syscall.CloseHandle(syscall.Handle(info.process))
	result, err := syscall.WaitForSingleObject(syscall.Handle(info.process), syscall.INFINITE)
	if err != nil {
		return fmt.Errorf("could not wait for elevated file helper: %w", err)
	}
	if result != syscall.WAIT_OBJECT_0 {
		return fmt.Errorf("elevated file helper wait ended unexpectedly (%d)", result)
	}
	var exitCode uint32
	if err := syscall.GetExitCodeProcess(syscall.Handle(info.process), &exitCode); err != nil {
		return fmt.Errorf("could not read elevated file helper result: %w", err)
	}
	if exitCode != 0 {
		message, _ := os.ReadFile(resultPath)
		if len(message) > 0 {
			return fmt.Errorf("%s", strings.TrimSpace(string(message)))
		}
		return fmt.Errorf("elevated file helper failed (exit code %d)", exitCode)
	}
	return nil
}
