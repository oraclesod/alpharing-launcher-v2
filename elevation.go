package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// runElevatedMod is the headless entry point for the Windows UAC helper.
// It never starts Steam or opens the launcher UI.
func runElevatedMod(args []string) int {
	if len(args) != 5 || (args[0] != "install" && args[0] != "uninstall") {
		return 2
	}
	gamePath, payloadPath, expectedHash, resultPath := args[1], args[2], args[3], args[4]
	if info, err := os.Stat(filepath.Join(gamePath, modDLLSubpath)); err != nil || !info.IsDir() {
		_ = os.WriteFile(resultPath, []byte("MCC binaries folder was not found"), 0600)
		return 1
	}
	var err error
	if args[0] == "install" {
		var data []byte
		data, err = os.ReadFile(payloadPath)
		if err == nil {
			if _, decodeErr := hex.DecodeString(expectedHash); decodeErr != nil {
				err = fmt.Errorf("invalid expected DLL hash")
			} else {
				err = replaceDLL(gamePath, data, expectedHash)
			}
		}
	} else {
		err = removeDLL(gamePath)
	}
	if err != nil {
		_ = os.WriteFile(resultPath, []byte(err.Error()), 0600)
		return 1
	}
	return 0
}
