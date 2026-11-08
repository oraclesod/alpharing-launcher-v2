//go:build !windows

package main

import "fmt"

func elevatePrepareMod(gamePath, action, payloadPath, expectedHash string) error {
	return fmt.Errorf("administrator elevation is only available on Windows")
}
